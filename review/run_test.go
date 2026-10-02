package review

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/terva-sh/vuoro/agent"
	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/review/drafts"
	"github.com/terva-sh/vuoro/state"
)

// fakeDriver stands in for an agent tool. Its agent reviews by calling the
// drafts server over HTTP, as Claude Code does, with the URL and the bearer
// token from its spec.
type fakeDriver struct {
	mu    sync.Mutex
	specs []agent.Spec
	// act is the agent's turn: it gets the drafts endpoint and the text sent.
	act func(call toolCaller, text string) string
}

type toolCaller func(name string, args map[string]any) (string, bool)

func (d *fakeDriver) Name() string { return "Fake Tool" }
func (d *fakeDriver) Probe(context.Context) (agent.Probe, error) {
	return agent.Probe{Version: "1"}, nil
}

func (d *fakeDriver) Start(_ context.Context, spec agent.Spec) (agent.Session, error) {
	d.mu.Lock()
	d.specs = append(d.specs, spec)
	n := len(d.specs)
	d.mu.Unlock()
	cfg := spec.Drafts.Config
	call := func(name string, args map[string]any) (string, bool) {
		b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call",
			"params": map[string]any{"name": name, "arguments": args}})
		req, _ := http.NewRequest("POST", cfg["url"].(string), bytes.NewReader(b))
		for k, v := range cfg["headers"].(map[string]string) {
			req.Header.Set(k, v)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return err.Error(), true
		}
		defer res.Body.Close()
		var out struct {
			Result struct {
				Content []struct{ Text string } `json:"content"`
				IsError bool                    `json:"isError"`
			} `json:"result"`
		}
		_ = json.NewDecoder(res.Body).Decode(&out)
		if len(out.Result.Content) == 0 {
			return fmt.Sprintf("HTTP %d", res.StatusCode), true
		}
		return out.Result.Content[0].Text, out.Result.IsError
	}
	s := &fakeSession{events: make(chan agent.Event, 64), d: d, call: call, ref: fmt.Sprint("ref-", n)}
	s.events <- agent.Event{Kind: agent.Info, Info: &agent.SessionInfo{Tool: "fake", Model: "m1"}}
	return s, nil
}

type fakeSession struct {
	events chan agent.Event
	d      *fakeDriver
	call   toolCaller
	ref    string
	once   sync.Once
}

func (s *fakeSession) Ref() string                { return s.ref }
func (s *fakeSession) Events() <-chan agent.Event { return s.events }
func (s *fakeSession) Cancel() error              { return s.Close() }
func (s *fakeSession) Close() error {
	s.once.Do(func() { close(s.events) })
	return nil
}

func (s *fakeSession) Send(_ context.Context, text string) error {
	go func() {
		s.d.mu.Lock()
		act := s.d.act
		s.d.mu.Unlock()
		reply := act(s.call, text)
		s.events <- agent.Event{Kind: agent.ToolCall, Tool: &agent.Tool{ID: "c1", Name: "Read", Input: `{"file_path":"calc.go"}`}}
		s.events <- agent.Event{Kind: agent.ToolResult, Tool: &agent.Tool{ID: "c1", Output: "package calc"}}
		s.events <- agent.Event{Kind: agent.Text, Text: reply}
		s.events <- agent.Event{Kind: agent.Usage, Usage: &agent.TurnUsage{Input: 10, Output: 5, CostUSD: 0.25}}
		s.events <- agent.Event{Kind: agent.TurnDone}
	}()
	return nil
}

// remote is a git repository with a main branch and a feature branch that
// changes calc.go, standing in for the forge's.
func remote(t *testing.T) (dir, base, head string) {
	t.Helper()
	dir = t.TempDir()
	git := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(dir, "calc.go"), []byte("package calc\n\nfunc Add(a, b int) int { return a + b }\n"), 0o644)
	git("add", ".")
	git("commit", "-q", "-m", "base")
	base = git("rev-parse", "HEAD")
	git("switch", "-q", "-c", "feature")
	os.WriteFile(filepath.Join(dir, "calc.go"), []byte("package calc\n\nfunc Add(a, b int) int { return a - b }\n"), 0o644)
	git("commit", "-q", "-am", "head")
	head = git("rev-parse", "HEAD")
	return dir, base, head
}

func setup(t *testing.T, act func(toolCaller, string) string) (*Manager, *fakeDriver, *forge.Change, Source) {
	t.Helper()
	dir, base, head := remote(t)
	srv := drafts.NewServer()
	hs := httptest.NewServer(srv)
	t.Cleanup(hs.Close)
	stateDir := t.TempDir()
	d := &fakeDriver{act: act}
	m := &Manager{
		StateDir: stateDir,
		Repos:    &Repos{Dir: filepath.Join(stateDir, "repos")},
		Drivers:  map[string]agent.Driver{"fake": d},
		Drafts:   srv,
		MCPURL:   hs.URL + "/mcp/",
	}
	ch := &forge.Change{
		Ref:   forge.Ref{Host: "gitlab.example", Project: "g/calc", Number: 7},
		Title: "Fix Add", Diff: forge.DiffRefs{Base: base, Head: head, Start: base},
		Files: []forge.FileDiff{{OldPath: "calc.go", NewPath: "calc.go",
			Patch: "@@ -1,3 +1,3 @@\n package calc\n \n-func Add(a, b int) int { return a + b }\n+func Add(a, b int) int { return a - b }\n"}},
	}
	src := Source{Remote: dir, Key: "gitlab.example/g/calc", Head: head, Base: base,
		Refspecs: []string{"+refs/heads/feature:refs/vuoro/head", "+refs/heads/main:refs/vuoro/target"}}
	return m, d, ch, src
}

func wait(t *testing.T, r *Run, ok func(RunView) bool) RunView {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if v := r.View(); ok(v) {
			return v
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("the run did not settle: %+v", r.View())
	return RunView{}
}

func idle(v RunView) bool { return v.Status == Idle || v.Status == Failed }

func TestAReviewDraftsFindingsAndAnInvalidOneIsSentBack(t *testing.T) {
	var refusal string
	var sawWorktree bool
	m, d, ch, src := setup(t, func(call toolCaller, text string) string {
		if !strings.Contains(text, "<diff>") || !strings.Contains(text, "-func Add(a, b int) int { return a + b }") {
			return "no diff in the first message"
		}
		// Line 9 is not in the diff, so the draft comes back with the rule.
		out, isErr := call("propose_draft", map[string]any{"severity": "high", "title": "Add subtracts",
			"description": "Add returns a - b.", "anchor": map[string]any{"path": "calc.go", "start_line": 9}})
		if !isErr {
			return "an anchor outside the diff was accepted"
		}
		refusal = out
		if _, isErr := call("propose_draft", map[string]any{"severity": "high", "title": "Add subtracts",
			"description": "Add returns a - b.", "anchor": map[string]any{"path": "calc.go", "start_line": 3}}); isErr {
			return "the corrected draft was refused"
		}
		return "Found one problem."
	})
	r, err := m.Start(ch, src, []Reviewer{{Profile: "code"}})
	if err != nil {
		t.Fatal(err)
	}
	dir := r.dir
	v := wait(t, r, idle)
	if v.Status != Idle {
		t.Fatalf("run %s: %s", v.Status, v.Error)
	}
	b, err := os.ReadFile(filepath.Join(dir, "calc.go"))
	sawWorktree = err == nil && strings.Contains(string(b), "a - b")
	if !sawWorktree {
		t.Fatalf("the worktree is not at the head: %q %v", b, err)
	}
	if !strings.Contains(refusal, "rule anchor.notInDiff at anchor/start_line") || !strings.Contains(refusal, "1-3") {
		t.Fatalf("refusal %q", refusal)
	}
	if len(v.Drafts) != 1 || v.Drafts[0].Anchor.StartLine != 3 || v.Drafts[0].Drafted != "Fake Tool · default model" {
		t.Fatalf("drafts %+v", v.Drafts)
	}
	s := v.Sessions[0]
	if s.Info == nil || s.Info.Model != "m1" || s.Usage.CostUSD != 0.25 || v.CostUSD != 0.25 {
		t.Fatalf("session %+v", s)
	}
	if n := len(s.Log); n != 3 || s.Log[0].Kind != "change" || s.Log[1].Output != "package calc" || s.Log[n-1].Text != "Found one problem." {
		t.Fatalf("log %+v", s.Log)
	}
	spec := d.specs[0]
	if spec.Dir != dir || !strings.Contains(spec.SystemPrompt, "Your focus: Code") || spec.Drafts == nil {
		t.Fatalf("spec %+v", spec)
	}

	// The person talks to it, and it answers in the same session.
	d.mu.Lock()
	d.act = func(toolCaller, string) string { return "Because a - b is not a sum." }
	d.mu.Unlock()
	if err := m.Say(r.ID, "s1", "why?"); err != nil {
		t.Fatal(err)
	}
	v = wait(t, r, func(v RunView) bool {
		l := v.Sessions[0].Log
		return v.Status == Idle && l[len(l)-1].Text == "Because a - b is not a sum."
	})

	// Closing removes the worktree and keeps the drafts in the record.
	if err := m.Close(context.Background(), r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("the worktree is still there: %v", err)
	}
	var rec RunView
	if err := state.ReadJSON(filepath.Join(m.StateDir, "runs", r.ID, "run.json"), &rec); err != nil {
		t.Fatal(err)
	}
	if rec.Status != Closed || len(rec.Drafts) != 1 {
		t.Fatalf("record %+v", rec)
	}
	// A closed run's sessions can no longer write.
	if err := m.Say(r.ID, "s1", "more"); err == nil {
		t.Fatal("a closed run accepted a message")
	}
}

func TestAReviewerOnlyEverGetsTheDraftsServer(t *testing.T) {
	m, d, ch, src := setup(t, func(toolCaller, string) string { return "ok" })
	r, err := m.Start(ch, src, []Reviewer{{Profile: "security"}, {Profile: "documentation"}})
	if err != nil {
		t.Fatal(err)
	}
	wait(t, r, idle)
	defer m.Close(context.Background(), r.ID)
	if len(d.specs) != 2 {
		t.Fatalf("%d sessions", len(d.specs))
	}
	for _, spec := range d.specs {
		if spec.Drafts.Name != "vuoro" || strings.Join(spec.Drafts.Tools, ",") != strings.Join(drafts.ToolNames(), ",") {
			t.Fatalf("drafts server %+v", spec.Drafts)
		}
		if spec.Drafts.Config["url"] != m.MCPURL {
			t.Fatalf("url %v", spec.Drafts.Config["url"])
		}
	}
}

func TestAnUnknownProfileOrToolIsRefused(t *testing.T) {
	m, _, ch, src := setup(t, nil)
	if _, err := m.Start(ch, src, []Reviewer{{Profile: "vibes"}}); err == nil {
		t.Fatal("an unknown profile was accepted")
	}
	if _, err := m.Start(ch, src, []Reviewer{{Profile: "code", Tool: "terva"}}); err == nil {
		t.Fatal("an unavailable tool was accepted")
	}
	if _, err := m.Start(ch, src, nil); err == nil {
		t.Fatal("a run with no reviewers was accepted")
	}
}

func TestAFailedCheckoutFailsTheRun(t *testing.T) {
	m, _, ch, src := setup(t, nil)
	src.Head = strings.Repeat("0", 40)
	r, err := m.Start(ch, src, []Reviewer{{Profile: "code"}})
	if err != nil {
		t.Fatal(err)
	}
	v := wait(t, r, idle)
	if v.Status != Failed || !strings.Contains(v.Error, "did not arrive") {
		t.Fatalf("%s %q", v.Status, v.Error)
	}
	src.Key = "../escape"
	if err := m.Repos.Checkout(context.Background(), src, t.TempDir()+"/x"); err == nil {
		t.Fatal("a key outside the repos directory was accepted")
	}
}

func TestShutdownEndsEveryLiveSession(t *testing.T) {
	m, _, ch, src := setup(t, func(toolCaller, string) string { return "ok" })
	r, err := m.Start(ch, src, []Reviewer{{Profile: "code"}})
	if err != nil {
		t.Fatal(err)
	}
	wait(t, r, idle)
	m.Shutdown()
	v := wait(t, r, func(v RunView) bool { return v.Sessions[0].Status == Ended })
	if v.Status == Closed {
		t.Fatal("shutdown closed the run")
	}
	m.Close(context.Background(), r.ID)
}
