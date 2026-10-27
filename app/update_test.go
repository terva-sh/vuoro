package app

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/review"
	"github.com/terva-sh/vuoro/server"
	"github.com/terva-sh/vuoro/state"
)

// updatedChange is an app with a ready update run on disk for !7.
func updatedChange(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	rec := map[string]any{"id": "update-run", "kind": forge.GitLab, "mode": review.UpdateMode, "ref": forge.Ref{Host: "lab.test", Project: "team/repo", Number: 7},
		"title": "Subtract", "status": "idle", "head": "aaaa", "sessions": []any{},
		"update": review.Update{Strategy: review.Merge, Source: "feature", Target: "main", Own: true, Result: "bbbb", Stops: []review.Stop{}, Started: time.Now()}}
	if err := state.WriteJSON(filepath.Join(dir, "runs", "update-run", "run.json"), rec); err != nil {
		t.Fatal(err)
	}
	a := &App{Runs: &review.Manager{StateDir: dir}}
	if err := a.Runs.Load(); err != nil {
		t.Fatal(err)
	}
	return a
}

// An update shows on its change, not among the reviews, and its editor is
// driven only through the update routes: the generic ones cannot push its
// work as a stacked change or post anything from it.
func TestAnUpdateRunAnswersOnlyItsOwnRoutes(t *testing.T) {
	a := updatedChange(t)
	var list []review.RunView
	_ = json.Unmarshal(projectCall(t, a, "GET", "/api/runs", "", 200), &list)
	if len(list) != 0 {
		t.Fatal("an update run is listed with the reviews")
	}
	_ = json.Unmarshal(projectCall(t, a, "POST", "/api/changes/updates", `{"url":"https://lab.test/team/repo/-/merge_requests/7"}`, 200), &list)
	if len(list) != 1 || list[0].Update == nil || list[0].Update.Result != "bbbb" {
		t.Fatalf("updates of !7 %+v", list)
	}
	_ = json.Unmarshal(projectCall(t, a, "POST", "/api/changes/updates", `{"url":"https://lab.test/team/repo/-/merge_requests/8"}`, 200), &list)
	if len(list) != 0 {
		t.Fatal("another change lists !7's update")
	}
	projectCall(t, a, "GET", "/api/runs/update-run", "", 200)
	for _, path := range []string{"/api/runs/update-run/editor", "/api/runs/update-run/editor/push", "/api/runs/update-run/editor/suggest",
		"/api/runs/update-run/drafts/d1/keep", "/api/runs/update-run/report"} {
		projectCall(t, a, "POST", path, "{}", 404)
	}
	projectCall(t, a, "POST", "/api/runs/update-run/update/push", `{"url":"https://lab.test/team/repo/-/merge_requests/7","sha":"bbbb"}`, 400)
}

func TestUpdateRoutesStayBehindWindowGuard(t *testing.T) {
	a := updatedChange(t)
	h := server.New(server.Config{Addr: "127.0.0.1:9123", Token: "fixture-window-token", Dist: fstest.MapFS{}, API: a.Handler()})
	for _, route := range []string{"POST /api/changes/update", "POST /api/changes/updates", "GET /api/runs/update-run/update",
		"POST /api/runs/update-run/update/accept", "POST /api/runs/update-run/update/discard", "POST /api/runs/update-run/update/push"} {
		method, path, _ := strings.Cut(route, " ")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(method, "http://127.0.0.1:9123"+path, strings.NewReader("{}")))
		if w.Code != 403 {
			t.Fatalf("%s bypassed window guard", route)
		}
	}
}
