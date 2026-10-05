package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/review"
	"github.com/terva-sh/vuoro/review/drafts"
	"github.com/terva-sh/vuoro/server"
	"github.com/terva-sh/vuoro/state"
)

type triageForge struct {
	*inboxReader
	ch        forge.Change
	writes    int
	published int
	pending   forge.NewDraft
}

type sharedTriageForge struct{ *triageForge }

func (p *sharedTriageForge) Open(context.Context, forge.Ref) (*forge.Change, error) {
	return &p.ch, nil
}

func (p *triageForge) Open(_ context.Context, ref forge.Ref) (*forge.Change, error) {
	b, _ := json.Marshal(p.ch)
	var ch forge.Change
	json.Unmarshal(b, &ch)
	ch.Ref = ref
	return &ch, nil
}
func (p *triageForge) CreateDraft(_ context.Context, _ forge.Ref, d forge.NewDraft) (forge.Draft, error) {
	p.writes++
	p.pending = d
	return forge.Draft{ID: "private-note", Body: d.Body, ThreadID: d.ThreadID}, nil
}
func (p *triageForge) Submit(context.Context, forge.Ref) error { p.published++; return nil }

func triageApp(t *testing.T) (*App, *triageForge, string) {
	t.Helper()
	t.Setenv("VUORO_TEST_TRIAGE_SECRET", "synthetic-secret")
	ch := forge.Change{Kind: forge.GitLab, Ref: forge.Ref{Host: "lab.test", Project: "team/repo", Number: 1}, Author: forge.User{Username: "me"}, State: forge.Open, Diff: forge.DiffRefs{Head: "head", Base: "base"}, WebURL: "https://lab.test/team/repo/-/merge_requests/1", Threads: []forge.Thread{{ID: "t1", Resolvable: true, Notes: []forge.Note{{ID: "n1", Author: forge.User{Username: "reviewer"}, Body: "Could this race?"}}}}}
	dir := t.TempDir()
	d := drafts.Draft{ID: "d1", ThreadID: "t1", ThreadKey: review.ThreadKey(ch.Threads[0]), Category: drafts.Discuss, Reason: "Need evidence", Description: "Could you share the failing scenario?", Status: drafts.Pending, Revision: 1, Session: "s1", Drafted: "Fake tool", Updated: time.Now()}
	rec := map[string]any{"id": "triage-run", "kind": ch.Kind, "mode": "triage", "ref": ch.Ref, "title": "A change", "web_url": ch.WebURL, "head": "head", "base": "base", "status": "idle", "triage_threads": review.TriageThreads(&ch), "drafts": []drafts.Draft{d}, "sessions": []any{}}
	if err := state.WriteJSON(filepath.Join(dir, "runs", "triage-run", "run.json"), rec); err != nil {
		t.Fatal(err)
	}
	m := &review.Manager{StateDir: dir}
	if err := m.Load(); err != nil {
		t.Fatal(err)
	}
	p := &triageForge{inboxReader: &inboxReader{kind: forge.GitLab, host: "lab.test"}, ch: ch}
	a := &App{Runs: m, Config: &config.Config{Forges: []config.Forge{{Name: "test", Kind: forge.GitLab, URL: "https://lab.test", Username: "ME", TokenEnv: "VUORO_TEST_TRIAGE_SECRET"}}}, Dial: func(string, string, config.Secret) (forge.Provider, error) { return p, nil }}
	return a, p, ch.WebURL
}

func TestAuthorReplyStaysLocalUntilPersonAcceptsThenSubmits(t *testing.T) {
	a, p, url := triageApp(t)
	h := a.Handler()
	base := "/api/runs/triage-run/drafts/d1"
	if code, _ := call(t, h, base+"/edit-reply", map[string]any{"body": "The person's reply", "category": "must_fix"}); code != 200 {
		t.Fatal("local edit failed")
	}
	if p.writes != 0 || p.published != 0 {
		t.Fatal("local edit posted")
	}
	if code, _ := call(t, h, base+"/keep", map[string]any{"url": url}); code != 200 {
		t.Fatal("human acceptance failed")
	}
	if p.writes != 1 || p.published != 0 || p.pending.ThreadID != "t1" || p.pending.Body != "The person's reply" || p.pending.Anchor != nil {
		t.Fatal("reply was not a private draft on its thread")
	}
	d, _, _ := a.Runs.Draft("triage-run", "d1")
	if d.Decision != drafts.Kept || !d.Edited {
		t.Fatal("person's provenance lost")
	}
	if code, _ := call(t, h, base+"/keep", map[string]any{"url": url}); code == 200 || p.writes != 1 {
		t.Fatal("duplicate acceptance posted")
	}
	if code, _ := call(t, h, "/api/changes/submit", map[string]any{"url": url}); code != 200 || p.published != 1 {
		t.Fatal("explicit submit failed")
	}
}

func TestAuthorReplyRejectsStaleAndMismatchedForgeSnapshots(t *testing.T) {
	for _, change := range []string{"head", "resolved", "edited note", "other change", "closed"} {
		t.Run(change, func(t *testing.T) {
			a, p, url := triageApp(t)
			switch change {
			case "head":
				p.ch.Diff.Head = "new"
			case "resolved":
				p.ch.Threads[0].Resolved = true
			case "edited note":
				p.ch.Threads[0].Notes[0].Body = "A changed undated note"
			case "other change":
				url = strings.Replace(url, "requests/1", "requests/2", 1)
			case "closed":
				p.ch.State = forge.Closed
			}
			code, _ := call(t, a.Handler(), "/api/runs/triage-run/drafts/d1/keep", map[string]any{"url": url})
			if code == 200 || p.writes != 0 || p.published != 0 {
				t.Fatal("stale or unrelated reply reached forge")
			}
		})
	}
}

func TestTriageLocalDiscardAndAuthorRouteChecks(t *testing.T) {
	a, p, url := triageApp(t)
	if err := a.Runs.EditReply("triage-run", "d1", "My local wording", drafts.Nit); err != nil {
		t.Fatal(err)
	}
	code, _ := call(t, a.Handler(), "/api/runs/triage-run/drafts/d1/discard-reply", map[string]any{})
	if code != 200 || p.writes != 0 {
		t.Fatal("local discard reached forge")
	}
	d, _, _ := a.Runs.Draft("triage-run", "d1")
	if !d.Edited || d.Description != "My local wording" {
		t.Fatal("discard lost the person's reply provenance")
	}
	p.ch.Author.Username = "other"
	if code, _ := call(t, a.Handler(), "/api/triage", map[string]any{"url": url}); code == 200 || code == 201 {
		t.Fatal("triaged another author's change")
	}
	if code, out := call(t, a.Handler(), "/api/runs/triage-run/editor", map[string]any{"url": url, "items": []string{"d1"}}); code == 200 || !strings.Contains(fmt.Sprint(out["error"]), "your own change") {
		t.Fatalf("an editor on another author's change: %d %v", code, out)
	}
}

func TestTriageNormalizesConfiguredBotsWithoutMutatingProvider(t *testing.T) {
	a, p, url := triageApp(t)
	a.Config.Forges[0].Bots = []string{"REVIEWER"}
	a.Dial = func(string, string, config.Secret) (forge.Provider, error) { return &sharedTriageForge{p}, nil }
	ch, err := a.open(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	if !ch.Threads[0].Notes[0].Author.Bot || p.ch.Threads[0].Notes[0].Author.Bot || len(review.TriageThreads(ch)) != 0 {
		t.Fatal("configured bot appeared as human or mutated provider data")
	}
}

func TestAuthorReplyRefusesOversizedOverrideBeforeWriting(t *testing.T) {
	a, p, url := triageApp(t)
	code, _ := call(t, a.Handler(), "/api/runs/triage-run/drafts/d1/keep", map[string]any{"url": url, "body": strings.Repeat("x", 20001)})
	if code == 200 || p.writes != 0 {
		t.Fatal("oversized reply reached forge")
	}
}

func TestAuthorReplyRequiresSavedLocalEditsBeforeAcceptance(t *testing.T) {
	a, p, url := triageApp(t)
	base := "/api/runs/triage-run/drafts/d1"
	code, _ := call(t, a.Handler(), base+"/keep", map[string]any{"url": url, "body": "An unsaved override"})
	if code == 200 || p.writes != 0 {
		t.Fatal("unsaved override reached forge")
	}
	if err := a.Runs.EditReply("triage-run", "d1", "My saved wording", drafts.Discuss); err != nil {
		t.Fatal(err)
	}
	code, _ = call(t, a.Handler(), base+"/keep", map[string]any{"url": url, "body": "My saved wording"})
	if code != 200 || p.pending.Body != "My saved wording" {
		t.Fatal("saved wording was not accepted")
	}
	restored := &review.Manager{StateDir: a.Runs.StateDir}
	if err := restored.Load(); err != nil {
		t.Fatal(err)
	}
	d, _, _ := restored.Draft("triage-run", "d1")
	if !d.Edited || d.Description != p.pending.Body || d.Decision != drafts.Kept {
		t.Fatal("accepted wording was not persisted")
	}
}

func TestTriageRoutesRequireTheWindowCredential(t *testing.T) {
	a, _, _ := triageApp(t)
	h := server.New(server.Config{Addr: "127.0.0.1:9123", Token: "synthetic-window-token", Dist: fstest.MapFS{}, API: a.Handler()})
	for _, path := range []string{"/api/triage", "/api/runs/triage-run/editor", "/api/runs/triage-run/editor/push", "/api/runs/triage-run/drafts/d1/edit-reply", "/api/runs/triage-run/drafts/d1/discard-reply", "/api/runs/triage-run/drafts/d1/keep"} {
		for _, mode := range []string{"missing", "agent", "origin", "host"} {
			r := httptest.NewRequest("POST", "http://127.0.0.1:9123"+path, strings.NewReader(`{}`))
			switch mode {
			case "agent":
				r.Header.Set("Authorization", "Bearer synthetic-agent-token")
			case "origin":
				r.Header.Set("X-Vuoro-Token", "synthetic-window-token")
				r.Header.Set("Origin", "https://evil.test")
			case "host":
				r.Header.Set("X-Vuoro-Token", "synthetic-window-token")
				r.Host = "evil.test:9123"
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != 403 {
				t.Fatalf("%s %s: %d", path, mode, w.Code)
			}
		}
	}
}
