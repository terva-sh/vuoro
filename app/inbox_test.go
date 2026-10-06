package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/inbox"
	"github.com/terva-sh/vuoro/server"
)

type inboxReader struct {
	kind, host string
	approved   bool
	err        error
	reads      int
}

func (p *inboxReader) Capabilities() forge.Capabilities {
	gl := p.kind == forge.GitLab
	return forge.Capabilities{Drafts: gl, Reply: gl, Resolve: gl, Approve: gl, Unapprove: gl, RequestChanges: gl, Suggestions: gl}
}
func (p *inboxReader) Discover(context.Context, string) ([]forge.Discovered, error) {
	if p.err != nil {
		return nil, p.err
	}
	return []forge.Discovered{{Ref: forge.Ref{Host: p.host, Project: "team/repo", Number: 1}, Roles: []forge.Role{forge.ReviewerRole}}}, nil
}
func (p *inboxReader) Open(ctx context.Context, ref forge.Ref) (*forge.Change, error) {
	return p.ReadFacts(ctx, ref)
}
func (p *inboxReader) ReadFacts(context.Context, forge.Ref) (*forge.Change, error) {
	p.reads++
	state := forge.Unreviewed
	ch := &forge.Change{Kind: p.kind, Ref: forge.Ref{Host: p.host, Project: "team/repo", Number: 1}, State: forge.Open, WebURL: "https://" + p.host + "/team/repo/pulls/1", Title: "A change", Author: forge.User{Username: "author"}, Diff: forge.DiffRefs{Head: "head"}, Requests: []forge.ReviewRequest{{Reviewer: "me", At: time.Now().Add(-time.Hour)}}}
	if p.approved {
		state = forge.Approved
		ch.Reviews = []forge.Review{{Reviewer: "me", Verdict: forge.VerdictApproved, At: time.Now(), SHA: "head"}}
	}
	if p.kind == forge.GitLab {
		ch.WebURL = "https://" + p.host + "/team/repo/-/merge_requests/1"
	}
	ch.Reviewers = []forge.Reviewer{{User: forge.User{Username: "me"}, State: state}}
	return ch, nil
}
func (p *inboxReader) Approve(context.Context, forge.Ref, bool) error { p.approved = true; return nil }
func (*inboxReader) CreateDraft(context.Context, forge.Ref, forge.NewDraft) (forge.Draft, error) {
	return forge.Draft{}, forge.ErrUnsupported
}
func (*inboxReader) UpdateDraft(context.Context, forge.Ref, string, string) error {
	return forge.ErrUnsupported
}
func (*inboxReader) DeleteDraft(context.Context, forge.Ref, string) error {
	return forge.ErrUnsupported
}
func (*inboxReader) Submit(context.Context, forge.Ref, forge.Verdict) error {
	return forge.ErrUnsupported
}
func (*inboxReader) Resolve(context.Context, forge.Ref, string, bool) error {
	return forge.ErrUnsupported
}

func inboxApp(t *testing.T) (*App, map[string]*inboxReader) {
	t.Helper()
	t.Setenv("VUORO_TEST_INBOX_SECRET", "synthetic-inbox-secret")
	ps := map[string]*inboxReader{"lab.test": {kind: forge.GitLab, host: "lab.test"}, "jo.test": {kind: forge.Forgejo, host: "jo.test"}}
	a := &App{StateDir: t.TempDir(), Config: &config.Config{Forges: []config.Forge{
		{Name: "Work", URL: "https://lab.test", Kind: forge.GitLab, Username: "me", TokenEnv: "VUORO_TEST_INBOX_SECRET"},
		{Name: "Internal", URL: "https://jo.test", Kind: forge.Forgejo, Username: "me", TokenEnv: "VUORO_TEST_INBOX_SECRET"},
	}}, Dial: func(kind, base string, _ config.Secret) (forge.Provider, error) {
		return ps[strings.TrimPrefix(base, "https://")], nil
	}}
	return a, ps
}
func inboxResult(t *testing.T, a *App, method, path string, body string) inbox.Display {
	t.Helper()
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
	if w.Code != 200 {
		t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
	}
	var out inbox.Display
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}
func TestInboxWithoutToolsTicketsOrRunsAndHumanWriteRefresh(t *testing.T) {
	a, ps := inboxApp(t)
	got := inboxResult(t, a, "POST", "/api/inbox/refresh", "{}")
	if len(got.Mine) != 2 || len(got.Accounts) != 2 {
		t.Fatal("mixed inbox requires unrelated integrations")
	}
	ps["jo.test"].err = &forge.APIError{Status: 429, Message: "synthetic-inbox-secret"}
	got = inboxResult(t, a, "POST", "/api/inbox/refresh", "{}")
	if len(got.Mine) != 2 || got.Accounts[1].Error == "" || strings.Contains(got.Accounts[1].Error, "synthetic-inbox-secret") {
		t.Fatal("account error erased results or exposed credentials")
	}
	code, _ := call(t, a.Handler(), "/api/changes/approve", map[string]any{"url": "https://lab.test/team/repo/-/merge_requests/1"})
	if code != 200 {
		t.Fatalf("human approval %d", code)
	}
	a.background.Wait() // the inbox catches up after the reply
	got = inboxResult(t, a, "GET", "/api/inbox", "")
	if len(got.Mine) != 1 || len(got.Theirs) != 1 || got.Theirs[0].Kind != forge.GitLab {
		t.Fatal("human action did not refresh derived turn")
	}
}
func TestInboxSnoozeBelongsToConfiguredMembership(t *testing.T) {
	a, _ := inboxApp(t)
	got := inboxResult(t, a, "POST", "/api/inbox/refresh", "{}")
	ref := got.Mine[0].Ref
	b, _ := json.Marshal(map[string]any{"ref": ref, "until": time.Now().Add(time.Hour)})
	got = inboxResult(t, a, "POST", "/api/inbox/snooze", string(b))
	if len(got.Snoozed) != 1 || len(got.Mine) != 1 {
		t.Fatal("snooze not applied")
	}
	b, _ = json.Marshal(map[string]any{"ref": ref, "until": time.Time{}})
	if len(inboxResult(t, a, "POST", "/api/inbox/snooze", string(b)).Snoozed) != 0 {
		t.Fatal("restore failed")
	}
	for _, body := range []string{`{"ref":{"host":"elsewhere.test","project":"team/repo","number":1},"until":"2099-01-01T00:00:00Z"}`, `{"ref":{"host":"lab.test","project":"team/repo","number":1},"until":"2000-01-01T00:00:00Z"}`} {
		w := httptest.NewRecorder()
		a.Handler().ServeHTTP(w, httptest.NewRequest("POST", "/api/inbox/snooze", strings.NewReader(body)))
		if w.Code != 400 {
			t.Fatal("invalid snooze accepted")
		}
	}
}
func TestInboxRoutesStayBehindWindowGuard(t *testing.T) {
	a, _ := inboxApp(t)
	h := server.New(server.Config{Addr: "127.0.0.1:9123", Token: "fixture-window-token", Dist: fstest.MapFS{}, API: a.Handler()})
	for _, path := range []string{"/api/inbox", "/api/inbox/refresh", "/api/inbox/snooze"} {
		method := "POST"
		if path == "/api/inbox" {
			method = "GET"
		}
		for _, mode := range []string{"missing-token", "other-origin", "rebound-host", "agent-token"} {
			r := httptest.NewRequest(method, "http://127.0.0.1:9123"+path, strings.NewReader("{}"))
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
				t.Fatalf("%s %s bypassed window guard", path, mode)
			}
		}
	}
}
func TestFailedConfigCannotMasqueradeAsEmptyInbox(t *testing.T) {
	a := &App{ConfigErr: errors.New("synthetic-sensitive-value")}
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/api/inbox", nil))
	if w.Code == 200 || strings.Contains(w.Body.String(), "synthetic-sensitive-value") {
		t.Fatal("bad configuration appeared empty or exposed raw error")
	}
}
