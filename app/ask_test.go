package app

import (
	"encoding/json"
	"net/http/httptest"
	"os"
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

// askedChange is an app with a conversation and an older summary about !7
// on disk, and a prompts directory of the person's.
func askedChange(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	ref := forge.Ref{Host: "lab.test", Project: "team/repo", Number: 7}
	for id, mode := range map[string]string{"ask-run": review.AskMode, "summary-run": "summary"} {
		rec := map[string]any{"id": id, "kind": forge.GitLab, "mode": mode, "ref": ref, "title": "Subtract", "status": "idle", "head": "bbbb",
			"created": time.Now().UTC(), "sessions": []any{}, "summary": review.SummaryRange{Scope: "change", From: "aaaa", To: "bbbb"}}
		if mode == review.AskMode {
			rec["ask"] = review.Ask{Title: "Why subtract?", Question: "Why subtract?"}
		}
		if err := state.WriteJSON(filepath.Join(dir, "runs", id, "run.json"), rec); err != nil {
			t.Fatal(err)
		}
	}
	prompts := filepath.Join(dir, "prompts")
	if err := os.MkdirAll(prompts, 0o700); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(prompts, "deps.md"), []byte("---\ntitle: Dependency impact\n---\nWhat do the new versions change?\n"), 0o600)
	os.WriteFile(filepath.Join(prompts, "broken.md"), []byte("---\nscope: all\n---\nHi\n"), 0o600)
	a := &App{Runs: &review.Manager{StateDir: dir}, PromptsDir: prompts}
	if err := a.Runs.Load(); err != nil {
		t.Fatal(err)
	}
	return a
}

// A conversation shows on its change and not among the reviews, and answers
// only the routes that read, talk to, compare, or close it.
func TestAConversationAnswersOnlyItsOwnRoutes(t *testing.T) {
	a := askedChange(t)
	var list []review.RunView
	_ = json.Unmarshal(projectCall(t, a, "GET", "/api/runs", "", 200), &list)
	for _, v := range list {
		if v.Mode == review.AskMode {
			t.Fatal("a conversation is listed with the reviews")
		}
	}
	_ = json.Unmarshal(projectCall(t, a, "POST", "/api/changes/asks", `{"url":"https://lab.test/team/repo/-/merge_requests/7"}`, 200), &list)
	if len(list) != 2 || list[0].Ask == nil || list[1].Ask == nil {
		t.Fatalf("asks of !7 %+v", list)
	}
	prompts := map[string]bool{}
	for _, v := range list {
		prompts[v.Ask.Prompt] = true
	}
	if !prompts[review.Explain] || !prompts[""] {
		t.Fatalf("the summary is not listed as explain: %+v", prompts)
	}
	_ = json.Unmarshal(projectCall(t, a, "POST", "/api/changes/asks", `{"url":"https://lab.test/team/repo/-/merge_requests/8"}`, 200), &list)
	if len(list) != 0 {
		t.Fatal("another change lists !7's conversations")
	}
	projectCall(t, a, "GET", "/api/runs/ask-run", "", 200)
	for _, path := range []string{"/api/runs/ask-run/editor", "/api/runs/ask-run/editor/push", "/api/runs/ask-run/editor/suggest",
		"/api/runs/ask-run/drafts/d1/keep", "/api/runs/ask-run/report", "/api/runs/ask-run/update/push"} {
		projectCall(t, a, "POST", path, "{}", 404)
	}
	projectCall(t, a, "POST", "/api/asks", `{"url":"https://lab.test/team/repo/-/merge_requests/7","prompt":"nope"}`, 400)
}

func TestAskRoutesStayBehindWindowGuard(t *testing.T) {
	a := askedChange(t)
	h := server.New(server.Config{Addr: "127.0.0.1:9123", Token: "fixture-window-token", Dist: fstest.MapFS{}, API: a.Handler()})
	for _, route := range []string{"POST /api/asks", "POST /api/changes/asks", "POST /api/runs/ask-run/sessions/s1/messages", "POST /api/runs/ask-run/head"} {
		method, path, _ := strings.Cut(route, " ")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(method, "http://127.0.0.1:9123"+path, strings.NewReader("{}")))
		if w.Code != 403 {
			t.Fatalf("%s bypassed window guard", route)
		}
	}
}
