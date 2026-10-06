package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
)

// capsForge is a writer whose capabilities a test sets, and which records
// every write that reaches it.
type capsForge struct {
	caps   forge.Capabilities
	writes []string
}

func (f *capsForge) Capabilities() forge.Capabilities { return f.caps }
func (f *capsForge) Open(_ context.Context, ref forge.Ref) (*forge.Change, error) {
	return &forge.Change{Kind: forge.GitLab, Ref: ref, State: forge.Open, Title: "A change", Author: forge.User{Username: "author"},
		WebURL: "https://lab.test/team/repo/-/merge_requests/1", Diff: forge.DiffRefs{Head: "head", Base: "base"}}, nil
}
func (f *capsForge) CreateDraft(_ context.Context, _ forge.Ref, d forge.NewDraft) (forge.Draft, error) {
	f.writes = append(f.writes, "draft "+d.ThreadID)
	return forge.Draft{ID: "1", Body: d.Body}, nil
}
func (f *capsForge) UpdateDraft(context.Context, forge.Ref, string, string) error {
	f.writes = append(f.writes, "update")
	return nil
}
func (f *capsForge) DeleteDraft(context.Context, forge.Ref, string) error {
	f.writes = append(f.writes, "delete")
	return nil
}
func (f *capsForge) Submit(_ context.Context, _ forge.Ref, v forge.Verdict) error {
	f.writes = append(f.writes, "submit "+string(v))
	return nil
}
func (f *capsForge) Resolve(context.Context, forge.Ref, string, bool) error {
	f.writes = append(f.writes, "resolve")
	return nil
}
func (f *capsForge) Approve(_ context.Context, _ forge.Ref, approve bool) error {
	f.writes = append(f.writes, fmt.Sprint("approve ", approve))
	return nil
}

func capsApp(t *testing.T, caps forge.Capabilities) (http.Handler, *capsForge) {
	t.Helper()
	t.Setenv("VUORO_TEST_CAPS_SECRET", "synthetic-caps-secret")
	f := &capsForge{caps: caps}
	a := &App{StateDir: t.TempDir(), Config: &config.Config{Forges: []config.Forge{
		{Name: "Work", URL: "https://lab.test", Kind: forge.GitLab, Username: "me", TokenEnv: "VUORO_TEST_CAPS_SECRET"},
	}}, Dial: func(string, string, config.Secret) (forge.Provider, error) { return f, nil }}
	return a.Handler(), f
}

const capsURL = "https://lab.test/team/repo/-/merge_requests/1"

// Every write the forge cannot do is refused before it reaches the forge, so
// a control the UI hides cannot be reached by a request either.
func TestAWriteTheForgeCannotDoIsRefused(t *testing.T) {
	h, f := capsApp(t, forge.Capabilities{Drafts: true, Approve: true})
	refused := []struct {
		path string
		body map[string]any
	}{
		{"/api/changes/drafts", map[string]any{"url": capsURL, "body": "me too", "thread_id": "t1"}},
		{"/api/changes/resolve", map[string]any{"url": capsURL, "thread_id": "t1", "resolved": true}},
		{"/api/changes/approve", map[string]any{"url": capsURL, "approve": false}},
		{"/api/changes/submit", map[string]any{"url": capsURL, "verdict": "changes_requested"}},
	}
	for _, c := range refused {
		if code, out := call(t, h, c.path, c.body); code != http.StatusUnprocessableEntity || !strings.Contains(fmt.Sprint(out["error"]), "does not support") {
			t.Errorf("%s %v: %d %v", c.path, c.body, code, out)
		}
	}
	if len(f.writes) != 0 {
		t.Fatalf("refused writes reached the forge: %q", f.writes)
	}

	// What it can do still goes through.
	for _, c := range []map[string]any{{"url": capsURL}, {"url": capsURL, "verdict": "approved"}} {
		if code, out := call(t, h, "/api/changes/submit", c); code != 200 {
			t.Fatalf("submit %v: %d %v", c, code, out)
		}
	}
	if code, out := call(t, h, "/api/changes/submit", map[string]any{"url": capsURL, "verdict": "lgtm"}); code != 400 {
		t.Fatalf("an unknown verdict: %d %v", code, out)
	}
	if got := strings.Join(f.writes, ", "); got != "submit commented, submit approved" {
		t.Fatalf("writes %q", got)
	}

	// A forge with no write capability at all takes no write of any kind.
	h, f = capsApp(t, forge.Capabilities{})
	for p, body := range map[string]map[string]any{
		"/api/changes/drafts":          {"url": capsURL, "body": "x"},
		"/api/changes/drafts/1/update": {"url": capsURL, "body": "x"},
		"/api/changes/drafts/1/delete": {"url": capsURL},
		"/api/changes/submit":          {"url": capsURL},
		"/api/changes/approve":         {"url": capsURL, "approve": true},
	} {
		if code, out := call(t, h, p, body); code != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d %v", p, code, out)
		}
	}
	if len(f.writes) != 0 {
		t.Fatalf("a read-only forge was written to: %q", f.writes)
	}
}

func TestSubmitCarriesThePersonsVerdict(t *testing.T) {
	h, f := capsApp(t, forge.Capabilities{Drafts: true, Reply: true, Resolve: true, Approve: true, Unapprove: true, RequestChanges: true})
	for _, v := range []string{"commented", "approved", "changes_requested"} {
		if code, out := call(t, h, "/api/changes/submit", map[string]any{"url": capsURL, "verdict": v}); code != 200 {
			t.Fatalf("%s: %d %v", v, code, out)
		}
	}
	if got := strings.Join(f.writes, ", "); got != "submit commented, submit approved, submit changes_requested" {
		t.Fatalf("writes %q", got)
	}
}
