package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/terva-sh/vuoro/server"
)

// gettingStarted reads the hints and the tour as the page does, with
// /api/setup.
func gettingStarted(t *testing.T, a *App) ([]string, tour) {
	t.Helper()
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/api/setup", nil))
	var out struct {
		Hints []string
		Tour  tour
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out.Hints, out.Tour
}

func TestADismissedHintOutlivesARestartAndComesBackOnReset(t *testing.T) {
	dir := t.TempDir()
	a := &App{StateDir: dir}
	if hints, _ := gettingStarted(t, a); hints == nil || len(hints) != 0 {
		t.Fatalf("a fresh start dismissed %v; every hint should show, and the list should be [] not null", hints)
	}
	for _, h := range []string{"local-empty", "first-drafts"} {
		if code, _ := call(t, a.Handler(), "/api/prefs/hint", map[string]any{"hint": h, "dismissed": true}); code != http.StatusOK {
			t.Fatalf("dismiss %s: %d", h, code)
		}
	}
	if hints, _ := gettingStarted(t, &App{StateDir: dir}); !slices.Equal(hints, []string{"local-empty", "first-drafts"}) {
		t.Fatalf("after a restart: %v", hints)
	}
	call(t, a.Handler(), "/api/prefs/hint", map[string]any{"hint": "local-empty", "dismissed": false})
	if hints, _ := gettingStarted(t, &App{StateDir: dir}); !slices.Equal(hints, []string{"first-drafts"}) {
		t.Fatalf("showing one again left %v", hints)
	}
	if code, _ := call(t, a.Handler(), "/api/prefs/hints/reset", map[string]any{}); code != http.StatusOK {
		t.Fatalf("reset: %d", code)
	}
	if hints, _ := gettingStarted(t, &App{StateDir: dir}); len(hints) != 0 {
		t.Fatalf("a reset left %v", hints)
	}
	if code, _ := call(t, a.Handler(), "/api/prefs/hint", map[string]any{"hint": "submit-everything", "dismissed": true}); code == http.StatusOK {
		t.Fatal("an unknown hint was dismissed")
	}
}

func TestTheToursProgressOutlivesARestartAndSettingsReplaysIt(t *testing.T) {
	dir := t.TempDir()
	a := &App{StateDir: dir}
	if _, got := gettingStarted(t, a); got != (tour{}) {
		t.Fatalf("a fresh start has the tour at %+v; it should not have started", got)
	}
	if code, _ := call(t, a.Handler(), "/api/prefs/tour", map[string]any{"state": "on", "step": "settings"}); code != http.StatusOK {
		t.Fatalf("step: %d", code)
	}
	if _, got := gettingStarted(t, &App{StateDir: dir}); got != (tour{State: "on", Step: "settings"}) {
		t.Fatalf("after a restart: %+v", got)
	}
	call(t, a.Handler(), "/api/prefs/tour", map[string]any{"state": "off", "step": "settings"})
	if _, got := gettingStarted(t, &App{StateDir: dir}); got.State != "off" {
		t.Fatalf("ending the tour left %+v", got)
	}
	// Settings replays it from the start.
	call(t, a.Handler(), "/api/prefs/tour", map[string]any{"state": "on"})
	if _, got := gettingStarted(t, &App{StateDir: dir}); got != (tour{State: "on"}) {
		t.Fatalf("a replay left %+v", got)
	}
	for _, bad := range []map[string]any{{"state": "paused"}, {"state": "on", "step": "start-a-run"}} {
		if code, _ := call(t, a.Handler(), "/api/prefs/tour", bad); code == http.StatusOK {
			t.Fatalf("%v was kept", bad)
		}
	}
}

func TestAMissingOrOlderPrefsFileReadsAsAFreshStart(t *testing.T) {
	for name, file := range map[string]string{
		"missing": "",
		// prefs.json as vuoro wrote it before getting started.
		"older": `{"waived":["submit"],"scheme":"dark","tab":"local"}`,
		// A hint and a step since removed, and a state no build knows.
		"edited":  `{"waived":[],"hints":["inbox-gone"],"tour":{"state":"paused","step":"gone"}}`,
		"damaged": `{"hints":`,
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if file != "" {
				if err := os.WriteFile(filepath.Join(dir, "prefs.json"), []byte(file), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			hints, got := gettingStarted(t, &App{StateDir: dir})
			if hints == nil || len(hints) != 0 || got != (tour{}) {
				t.Fatalf("hints %v, tour %+v; want a fresh start", hints, got)
			}
		})
	}
	// An older file keeps what it held when getting started is written.
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "prefs.json"), []byte(`{"waived":["submit"],"scheme":"dark","tab":"local"}`), 0o600)
	a := &App{StateDir: dir}
	call(t, a.Handler(), "/api/prefs/hint", map[string]any{"hint": "local-empty", "dismissed": true})
	if p := a.loadPrefs(); !slices.Equal(p.Waived, []string{"submit"}) || p.Scheme != "dark" || p.Tab != "local" {
		t.Fatalf("dismissing a hint lost the older prefs: %+v", p)
	}
}

func TestGettingStartedRoutesStayBehindWindowGuard(t *testing.T) {
	a := &App{StateDir: t.TempDir()}
	h := server.New(server.Config{Addr: "127.0.0.1:9123", Token: "fixture-window-token", Dist: fstest.MapFS{}, API: a.Handler()})
	for _, path := range []string{"/api/prefs/hint", "/api/prefs/hints/reset", "/api/prefs/tour"} {
		for _, mode := range []string{"missing-token", "other-origin", "rebound-host", "agent-token"} {
			r := httptest.NewRequest("POST", "http://127.0.0.1:9123"+path, strings.NewReader(`{"hint":"local-empty","dismissed":true,"state":"off"}`))
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
				t.Fatalf("%s %s bypassed window guard: %d", path, mode, w.Code)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(a.StateDir, "prefs.json")); err == nil {
		t.Fatal("a refused request wrote prefs.json")
	}
	// The window's own request still gets through.
	r := httptest.NewRequest("POST", "http://127.0.0.1:9123/api/prefs/tour", strings.NewReader(`{"state":"off"}`))
	r.Header.Set(server.TokenHeader, "fixture-window-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("the window's request: %d %s", w.Code, w.Body)
	}
}

// The window keeps its own lists of the hints and the tour's steps. A step
// or hint added there and not here would be dropped from prefs.json as
// unknown, so the person would see it again after every restart.
func TestTheWindowsHintsAndTourStepsAreTheOnesKept(t *testing.T) {
	read := func(name string) string {
		b, err := os.ReadFile(filepath.Join("..", "web", "src", name))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	var steps []string
	for _, m := range regexp.MustCompile(`\{ id: '([a-z-]+)', surface:`).FindAllStringSubmatch(read("tour.tsx"), -1) {
		steps = append(steps, m[1])
	}
	if !slices.Equal(steps, TourSteps) {
		t.Errorf("web/src/tour.tsx has steps %v, app.TourSteps %v", steps, TourSteps)
	}
	m := regexp.MustCompile(`allHints: HintID\[\] = \[([^\]]*)\]`).FindStringSubmatch(read("guide.tsx"))
	if m == nil {
		t.Fatal("web/src/guide.tsx has no allHints")
	}
	var hints []string
	for _, h := range regexp.MustCompile(`'([a-z-]+)'`).FindAllStringSubmatch(m[1], -1) {
		hints = append(hints, h[1])
	}
	if !slices.Equal(hints, Hints) {
		t.Errorf("web/src/guide.tsx has hints %v, app.Hints %v", hints, Hints)
	}
}
