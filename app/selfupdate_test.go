package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/terva-sh/vuoro/server"
	"github.com/terva-sh/vuoro/update"
)

// fakeUpdater answers with latest, or fails with err, and records what it
// was asked to apply.
type fakeUpdater struct {
	latest  string
	err     error
	asked   int
	applied []string
}

func (f *fakeUpdater) Latest(context.Context, string) (*update.Release, error) {
	f.asked++
	if f.err != nil {
		return nil, f.err
	}
	return &update.Release{Tag: f.latest}, nil
}

func (f *fakeUpdater) Apply(_ context.Context, rel *update.Release, current, target string) error {
	f.applied = append(f.applied, current+"→"+rel.Tag+" at "+target)
	return nil
}

func selfUpdate(t *testing.T, a *App, method string) (int, selfUpdateView) {
	t.Helper()
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, httptest.NewRequest(method, "/api/self-update", nil))
	var v selfUpdateView
	json.Unmarshal(w.Body.Bytes(), &v)
	return w.Code, v
}

func TestADevelopmentBuildIsNeverChecked(t *testing.T) {
	f := &fakeUpdater{latest: "v0.2.0"}
	for _, version := range []string{"dev", "v0.1.2-0.20261029191500-efcff53abcde"} {
		a := &App{Version: version, Updates: f}
		if code, v := selfUpdate(t, a, "GET"); code != 200 || v.Release || v.Newer {
			t.Errorf("%s: %d %+v", version, code, v)
		}
		if code, _ := selfUpdate(t, a, "POST"); code != 409 {
			t.Errorf("%s: an update was applied: %d", version, code)
		}
	}
	if f.asked != 0 || len(f.applied) != 0 {
		t.Fatalf("asked %d, applied %v", f.asked, f.applied)
	}
}

// The window asks once a day, and the update is applied only on the
// person's request.
func TestTheWindowOffersANewerReleaseAndAppliesItOnlyWhenAsked(t *testing.T) {
	f := &fakeUpdater{latest: "v0.2.0"}
	a := &App{Version: "v0.1.1", Updates: f, Executable: func() (string, error) { return "/opt/vuoro", nil }}
	for range 3 {
		if code, v := selfUpdate(t, a, "GET"); code != 200 || !v.Release || !v.Newer || v.Latest != "v0.2.0" {
			t.Fatalf("%d %+v", code, v)
		}
	}
	if f.asked != 1 || len(f.applied) != 0 {
		t.Fatalf("asked %d times, applied %v", f.asked, f.applied)
	}
	code, v := selfUpdate(t, a, "POST")
	if code != 200 || v.Applied != "v0.2.0" || v.Newer {
		t.Fatalf("%d %+v", code, v)
	}
	if len(f.applied) != 1 || f.applied[0] != "v0.1.1→v0.2.0 at /opt/vuoro" {
		t.Fatalf("applied %v", f.applied)
	}
	// Once applied, it waits for a reopen, and is not applied twice.
	if code, _ := selfUpdate(t, a, "POST"); code != 409 || len(f.applied) != 1 {
		t.Fatalf("a second update: %d, %v", code, f.applied)
	}
	if _, v := selfUpdate(t, a, "GET"); v.Applied != "v0.2.0" || v.Newer {
		t.Fatalf("%+v", v)
	}
}

func TestAnUpToDateOrUnreachableReleaseOffersNothing(t *testing.T) {
	f := &fakeUpdater{latest: "v0.1.1"}
	a := &App{Version: "v0.1.1", Updates: f}
	if _, v := selfUpdate(t, a, "GET"); v.Newer || v.Latest != "v0.1.1" {
		t.Fatalf("%+v", v)
	}
	if code, _ := selfUpdate(t, a, "POST"); code != 409 || len(f.applied) != 0 {
		t.Fatalf("an update to the same release: %d", code)
	}
	f = &fakeUpdater{err: errors.New("offline")}
	a = &App{Version: "v0.1.1", Updates: f}
	selfUpdate(t, a, "GET")
	if _, v := selfUpdate(t, a, "GET"); v.Newer || v.Error != "offline" || f.asked != 1 {
		t.Fatalf("%+v, asked %d", v, f.asked)
	}
}

func TestSelfUpdateStaysBehindTheWindowGuard(t *testing.T) {
	f := &fakeUpdater{latest: "v0.2.0"}
	a := &App{Version: "v0.1.1", Updates: f, Executable: func() (string, error) { return "/opt/vuoro", nil }}
	h := server.New(server.Config{Addr: "127.0.0.1:9123", Token: "fixture-window-token", Dist: fstest.MapFS{}, API: a.Handler()})
	for _, method := range []string{"GET", "POST"} {
		for _, mode := range []string{"missing-token", "other-origin", "rebound-host", "agent-token"} {
			r := httptest.NewRequest(method, "http://127.0.0.1:9123/api/self-update", strings.NewReader("{}"))
			r.Header.Set(server.TokenHeader, "fixture-window-token")
			switch mode {
			case "missing-token":
				r.Header.Del(server.TokenHeader)
			case "other-origin":
				r.Header.Set("Origin", "https://elsewhere.test")
			case "rebound-host":
				r.Host = "elsewhere.test"
			case "agent-token":
				r.Header.Set(server.TokenHeader, "fixture-drafts-token")
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != 403 {
				t.Fatalf("%s %s bypassed the window guard: %d", method, mode, w.Code)
			}
		}
	}
	if f.asked != 0 || len(f.applied) != 0 {
		t.Fatalf("a refused request reached the updater: asked %d, applied %v", f.asked, f.applied)
	}
}
