package app

import (
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/terva-sh/vuoro/review"
	"github.com/terva-sh/vuoro/review/drafts"
	"github.com/terva-sh/vuoro/server"
	"github.com/terva-sh/vuoro/state"
)

func TestHeadRouteReadsFreshIdentityAndReportsUnavailableWithoutWriting(t *testing.T) {
	a, p, url := triageApp(t)
	h := a.Handler()
	path := "/api/runs/triage-run/head"
	code, b := call(t, h, path, map[string]any{"url": url})
	if code != 200 || b["relation"] != "same" {
		t.Fatalf("same: %d %v", code, b)
	}
	p.ch.Diff.Head = "a-new-head"
	code, b = call(t, h, path, map[string]any{"url": url})
	if code != 200 || b["relation"] != "unavailable" || b["current_head"] != "a-new-head" || b["commits_behind"] != nil {
		t.Fatalf("unknown: %d %v", code, b)
	}
	code, _ = call(t, h, path, map[string]any{"url": strings.Replace(url, "requests/1", "requests/2", 1)})
	if code == 200 || p.writes != 0 || p.published != 0 {
		t.Fatal("wrong-change comparison or forge write")
	}
	if a.Runs.Get("triage-run").Head != "head" {
		t.Fatal("status changed pinned head")
	}
}

func TestHeadRouteRequiresWindowCredential(t *testing.T) {
	a, _, _ := triageApp(t)
	h := server.New(server.Config{Addr: "127.0.0.1:9123", Token: "synthetic-window", Dist: fstest.MapFS{}, API: a.Handler()})
	for _, mode := range []string{"missing", "agent", "origin", "host"} {
		r := httptest.NewRequest("POST", "http://127.0.0.1:9123/api/runs/triage-run/head", strings.NewReader(`{}`))
		switch mode {
		case "agent":
			r.Header.Set("Authorization", "Bearer synthetic-agent")
		case "origin":
			r.Header.Set("X-Vuoro-Token", "synthetic-window")
			r.Header.Set("Origin", "https://evil.test")
		case "host":
			r.Header.Set("X-Vuoro-Token", "synthetic-window")
			r.Host = "evil.test"
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatalf("%s: %d", mode, w.Code)
		}
	}
}

func TestFindingKeepChecksFreshChangeIdentityAndHead(t *testing.T) {
	// Seed a regular finding through the run record used by the existing app
	// fixture. No agent or forge network is involved.
	for _, mode := range []string{"head", "change"} {
		t.Run(mode, func(t *testing.T) {
			a, p, url := triageApp(t)
			r := a.Runs.Get("triage-run")
			v := r.View()
			v.Mode = ""
			v.Threads = nil
			d := v.Drafts[0]
			d.ThreadID = ""
			d.ThreadKey = ""
			d.Title = "A finding"
			d.Anchor = &drafts.Anchor{Path: "a.go", Side: drafts.Head, StartLine: 1, EndLine: 1}
			v.Drafts = []drafts.Draft{d}
			if err := state.WriteJSON(filepath.Join(a.Runs.StateDir, "runs", "triage-run", "run.json"), v); err != nil {
				t.Fatal(err)
			}
			m := &review.Manager{StateDir: a.Runs.StateDir}
			if err := m.Load(); err != nil {
				t.Fatal(err)
			}
			a.Runs = m
			if mode == "head" {
				p.ch.Diff.Head = "new"
			} else {
				url = strings.Replace(url, "requests/1", "requests/2", 1)
			}
			code, _ := call(t, a.Handler(), "/api/runs/triage-run/drafts/d1/keep", map[string]any{"url": url})
			if code == 200 || p.writes != 0 {
				t.Fatal("stale or wrong-change anchored finding posted")
			}
		})
	}
}
