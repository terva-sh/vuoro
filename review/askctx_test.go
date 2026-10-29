package review

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/terva-sh/vuoro/forge"
)

// contextFor is an ask session's context tools over a clone of the test
// change, with facts as the forge read.
func contextFor(t *testing.T, facts AskFacts) (*askContext, Source, *Manager) {
	t.Helper()
	m, _, _, src := setup(t, nil)
	if err := m.Repos.Checkout(context.Background(), src, filepath.Join(t.TempDir(), "wt")); err != nil {
		t.Fatal(err)
	}
	return &askContext{repos: m.Repos, key: src.Key, base: src.Base, head: src.Head, facts: func() AskFacts { return facts }}, src, m
}

func callTool(t *testing.T, c *askContext, name string, args map[string]any) (string, bool) {
	t.Helper()
	raw, _ := json.Marshal(args)
	return c.Call(name, raw)
}

func TestTheGitToolsReadTheChangesHistory(t *testing.T) {
	c, src, _ := contextFor(t, AskFacts{})
	if out, isErr := callTool(t, c, "git_log", map[string]any{"max_count": 5}); isErr || !strings.Contains(out, src.Head) || !strings.Contains(out, "\thead") || !strings.Contains(out, src.Base) {
		t.Fatalf("git_log: %v %s", isErr, out)
	}
	if out, isErr := callTool(t, c, "git_log", map[string]any{"rev": "base", "path": "calc.go"}); isErr || strings.Contains(out, src.Head) {
		t.Fatalf("git_log at base: %v %s", isErr, out)
	}
	if out, isErr := callTool(t, c, "git_show", map[string]any{"rev": src.Head[:10]}); isErr || !strings.Contains(out, "-func Add(a, b int) int { return a + b }") {
		t.Fatalf("git_show: %v %s", isErr, out)
	}
	if out, isErr := callTool(t, c, "git_blame", map[string]any{"path": "calc.go", "start_line": 3, "end_line": 3}); isErr || !strings.Contains(out, "a - b") {
		t.Fatalf("git_blame: %v %s", isErr, out)
	}
	if out, isErr := callTool(t, c, "git_diff", map[string]any{"from": "base", "path": "calc.go"}); isErr || !strings.Contains(out, "+func Add(a, b int) int { return a - b }") {
		t.Fatalf("git_diff: %v %s", isErr, out)
	}
	if out, isErr := callTool(t, c, "git_log", map[string]any{"rev": src.Head[:12] + "~1", "max_count": 1}); isErr || !strings.Contains(out, src.Base) {
		t.Fatalf("git_log head~1: %v %s", isErr, out)
	}
}

// Every refusal names its rule and runs nothing: an option dressed as a
// revision, a branch name, a commit the change does not hold, a path outside
// the repository, an unknown argument.
func TestTheGitToolsRefuseWhatTheChangeDoesNotHold(t *testing.T) {
	c, src, _ := contextFor(t, AskFacts{})
	outside := historyCommit(t, src.Remote, "unrelated") // on feature, past the change's head
	historyGit(t, src.Remote, "update-ref", "refs/heads/elsewhere", outside)
	for _, call := range []struct {
		tool string
		args map[string]any
		want string
	}{
		{"git_log", map[string]any{"rev": "--output=/tmp/vuoro-owned"}, "rule rev"},
		{"git_log", map[string]any{"rev": "main"}, "rule rev"},
		{"git_log", map[string]any{"rev": "HEAD"}, "rule rev"},
		{"git_show", map[string]any{"rev": "deadbeefdeadbeef"}, "not a commit"},
		{"git_show", map[string]any{"rev": "head..base"}, "rule rev"},
		{"git_show", map[string]any{"rev": "head", "path": "../../etc/passwd"}, "rule path"},
		{"git_show", map[string]any{"rev": "head", "path": "/etc/passwd"}, "rule path"},
		{"git_diff", map[string]any{"from": "base", "path": ":(glob)**"}, ""},
		{"git_blame", map[string]any{"path": "calc.go", "start_line": 1, "end_line": 5000}, "rule start_line"},
		{"git_log", map[string]any{"max_count": 1, "format": "%H"}, "rule arguments"},
		{"git_show", map[string]any{}, "rule rev"},
		{"rm", map[string]any{}, "rule tool.unknown"},
	} {
		out, isErr := callTool(t, c, call.tool, call.args)
		if call.want == "" {
			continue // a literal path that matches nothing is no refusal, and no glob
		}
		if !isErr || !strings.Contains(out, call.want) {
			t.Errorf("%s %v: %v %q, want a refusal naming %q", call.tool, call.args, isErr, out, call.want)
		}
	}
	if _, err := os.Stat("/tmp/vuoro-owned"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("an option given as a revision wrote a file")
	}
	// A commit fetched into the clone but outside the change's history is
	// refused, though git can read it.
	c2, _, _ := contextFor(t, AskFacts{})
	c2.repos, c2.key = c.repos, c.key
	if out, isErr := callTool(t, c2, "git_show", map[string]any{"rev": outside}); !isErr || !strings.Contains(out, "rule rev") {
		t.Fatalf("a commit outside the change: %v %q", isErr, out)
	}
}

// An answer past the cap is refused with the reason, and nothing of it sent.
func TestAGitAnswerPastTheCapIsRefused(t *testing.T) {
	m, _, _, src := setup(t, nil)
	os.WriteFile(filepath.Join(src.Remote, "big.txt"), []byte(strings.Repeat("a line of the big file\n", 10000)), 0o600)
	historyGit(t, src.Remote, "switch", "-q", "feature")
	historyGit(t, src.Remote, "add", "big.txt")
	historyGit(t, src.Remote, "commit", "-qm", "Big")
	src.Head = historyGit(t, src.Remote, "rev-parse", "HEAD")
	if err := m.Repos.Checkout(context.Background(), src, filepath.Join(t.TempDir(), "wt")); err != nil {
		t.Fatal(err)
	}
	c := &askContext{repos: m.Repos, key: src.Key, base: src.Base, head: src.Head, facts: func() AskFacts { return AskFacts{} }}
	out, isErr := callTool(t, c, "git_show", map[string]any{"rev": "head"})
	if !isErr || !strings.Contains(out, "larger than 128 KiB") || strings.Contains(out, "a line of the big file") {
		t.Fatalf("past the cap: %v %.200q", isErr, out)
	}
	if out, isErr := callTool(t, c, "git_show", map[string]any{"rev": "head", "path": "calc.go"}); isErr {
		t.Fatalf("narrowed to a path: %q", out)
	}
}

// No context tool changes the clone: its refs and objects are as they were.
func TestTheContextToolsWriteNothing(t *testing.T) {
	c, src, m := contextFor(t, AskFacts{})
	gitDir := filepath.Join(m.Repos.Dir, filepath.FromSlash(src.Key)+".git")
	state := func() string {
		return historyGit(t, gitDir, "for-each-ref") + "\n" + historyGit(t, gitDir, "count-objects", "-v")
	}
	before := state()
	for _, name := range ContextToolNames() {
		args := map[string]any{}
		switch name {
		case "git_show":
			args["rev"] = "head"
		case "git_blame":
			args = map[string]any{"path": "calc.go", "start_line": 1, "end_line": 3}
		case "git_diff":
			args["from"] = "base"
		}
		callTool(t, c, name, args)
	}
	if after := state(); after != before {
		t.Fatalf("the clone changed:\n%s\n---\n%s", before, after)
	}
}

type refuseTransport struct{ t *testing.T }

func (r refuseTransport) RoundTrip(*http.Request) (*http.Response, error) {
	r.t.Error("a context tool sent a request")
	return nil, errors.New("no requests")
}

// change, discussion, and pipeline answer from the forge read they were
// given, and send no request of their own.
func TestTheChangeToolsAnswerFromTheForgeRead(t *testing.T) {
	at := time.Date(2026, 10, 29, 9, 0, 0, 0, time.UTC)
	facts := AskFacts{Title: "Bump deps", Description: "Moves to v2", Author: "dev", State: "opened", Source: "bump", Target: "main",
		Labels: []string{"deps"}, Pipeline: "failed", ReadAt: at, Threads: []forge.Thread{
			{ID: "t1", Resolvable: true, Anchor: &forge.Anchor{Path: "calc.go", NewLine: 3}, Notes: []forge.Note{
				{Author: forge.User{Username: "ana"}, Body: "Why subtract?"},
				{Author: forge.User{Username: "gitlab"}, Body: "changed this line", System: true},
			}},
			{ID: "t2", Resolvable: true, Resolved: true, Notes: []forge.Note{{Author: forge.User{Username: "bo"}, Body: "Done"}}},
		}}
	c, _, _ := contextFor(t, facts)
	saved := http.DefaultTransport
	http.DefaultTransport = refuseTransport{t}
	defer func() { http.DefaultTransport = saved }()

	out, isErr := callTool(t, c, "change", nil)
	if isErr || !strings.Contains(out, `"description":"Moves to v2"`) || strings.Contains(out, "Why subtract?") {
		t.Fatalf("change: %v %s", isErr, out)
	}
	out, isErr = callTool(t, c, "discussion", nil)
	if isErr || !strings.Contains(out, "Why subtract?") || !strings.Contains(out, `"line":3`) || strings.Contains(out, "changed this line") || !strings.Contains(out, "Done") {
		t.Fatalf("discussion: %v %s", isErr, out)
	}
	if out, _ = callTool(t, c, "discussion", map[string]any{"unresolved_only": true}); strings.Contains(out, "Done") {
		t.Fatalf("unresolved only: %s", out)
	}
	out, isErr = callTool(t, c, "pipeline", nil)
	if isErr || !strings.Contains(out, `"status":"failed"`) || !strings.Contains(out, "not its jobs") {
		t.Fatalf("pipeline: %v %s", isErr, out)
	}
}

// A follow-up's fresher forge read replaces the facts, for that change only.
func TestAFollowUpGivesTheConversationAFresherRead(t *testing.T) {
	m, _, ch, src := setup(t, func(toolCaller, string) string { return "ok" })
	ch.Description = "first read"
	r, err := m.StartAsk(ch, src, Reviewer{Tool: "fake"}, AskInput{Question: "Why?"})
	if err != nil {
		t.Fatal(err)
	}
	wait(t, r, func(v RunView) bool { return v.Status == Idle })
	if r.facts().Description != "first read" {
		t.Fatalf("facts %+v", r.facts())
	}
	later := *ch
	later.Description = "second read"
	if err := m.SetAskFacts(r.ID, &later); err != nil {
		t.Fatal(err)
	}
	if r.facts().Description != "second read" {
		t.Fatal("the fresher read was not kept")
	}
	other := later
	other.Ref.Number++
	if err := m.SetAskFacts(r.ID, &other); err == nil {
		t.Fatal("another change's read replaced the facts")
	}
	loaded := &Manager{StateDir: m.StateDir}
	m.Shutdown()
	if err := loaded.Load(); err != nil {
		t.Fatal(err)
	}
	if loaded.Get(r.ID).facts().Description != "second read" {
		t.Fatal("the facts did not outlive a restart")
	}
}
