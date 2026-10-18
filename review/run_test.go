package review

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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
	// edit is an editor's turn, in its spec's directory and environment.
	edit func(spec agent.Spec, text string) string
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
	call := func(string, map[string]any) (string, bool) { return "no drafts server", true }
	if spec.Drafts != nil {
		call = draftsCaller(spec.Drafts.Config)
	}
	s := &fakeSession{events: make(chan agent.Event, 64), d: d, call: call, ref: fmt.Sprint("ref-", n), spec: spec}
	s.events <- agent.Event{Kind: agent.Info, Info: &agent.SessionInfo{Tool: "fake", Model: "m1"}}
	return s, nil
}

func draftsCaller(cfg map[string]any) toolCaller {
	return func(name string, args map[string]any) (string, bool) {
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
}

type fakeSession struct {
	events chan agent.Event
	d      *fakeDriver
	call   toolCaller
	ref    string
	spec   agent.Spec
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
		act, edit := s.d.act, s.d.edit
		s.d.mu.Unlock()
		var reply string
		if s.spec.Editor != nil {
			reply = edit(s.spec, text)
		} else {
			reply = act(s.call, text)
		}
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
	// No session's goroutine may write the state directory once the test
	// has returned; this runs before the directory is removed.
	t.Cleanup(m.Shutdown)
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
	r, err := m.Start(ch, src, []Reviewer{{Profile: "code"}}, nil, nil)
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
	r, err := m.Start(ch, src, []Reviewer{{Profile: "security"}, {Profile: "documentation"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	wait(t, r, idle)
	defer m.Close(context.Background(), r.ID)
	if len(d.specs) != 2 {
		t.Fatalf("%d sessions", len(d.specs))
	}
	for _, spec := range d.specs {
		if spec.Drafts.Name != "vuoro" || strings.Join(spec.Drafts.Tools, ",") != strings.Join(drafts.ToolNames(drafts.Reviewer), ",") {
			t.Fatalf("drafts server %+v", spec.Drafts)
		}
		if spec.Drafts.Config["url"] != m.MCPURL {
			t.Fatalf("url %v", spec.Drafts.Config["url"])
		}
	}
}

func TestAnUnknownProfileOrToolIsRefused(t *testing.T) {
	m, _, ch, src := setup(t, nil)
	if _, err := m.Start(ch, src, []Reviewer{{Profile: "vibes"}}, nil, nil); err == nil {
		t.Fatal("an unknown profile was accepted")
	}
	if _, err := m.Start(ch, src, []Reviewer{{Profile: "code", Tool: "terva"}}, nil, nil); err == nil {
		t.Fatal("an unavailable tool was accepted")
	}
	if _, err := m.Start(ch, src, nil, nil, nil); err == nil {
		t.Fatal("a run with no reviewers was accepted")
	}
}

func TestAFailedCheckoutFailsTheRun(t *testing.T) {
	m, _, ch, src := setup(t, nil)
	src.Head = strings.Repeat("0", 40)
	r, err := m.Start(ch, src, []Reviewer{{Profile: "code"}}, nil, nil)
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

// A fetch starts git's automatic maintenance, which detaches by default and
// outlives the fetch. On Windows the detached process keeps the clone open
// past a worktree removal or a delete, so vuoro runs it attached.
func TestCheckoutLeavesNoDetachedGit(t *testing.T) {
	m, _, _, src := setup(t, nil)
	trace := filepath.Join(t.TempDir(), "trace.json")
	t.Setenv("GIT_TRACE2_EVENT", trace)
	if err := m.Repos.Checkout(context.Background(), src, filepath.Join(t.TempDir(), "wt")); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(trace)
	if err != nil {
		t.Fatal(err)
	}
	var started bool
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		var e struct {
			Event string   `json:"event"`
			Argv  []string `json:"argv"`
		}
		if json.Unmarshal([]byte(line), &e) != nil || e.Event != "child_start" || !slices.Contains(e.Argv, "maintenance") {
			continue
		}
		started = true
		if slices.Contains(e.Argv, "--detach") {
			t.Fatalf("git ran maintenance detached: %v", e.Argv)
		}
	}
	if !started {
		t.Skip("this git starts no automatic maintenance on fetch")
	}
}

// Closing a run while its checkout is still running stops the checkout and
// waits for it, so no git of the run's outlives the Close, and the run starts
// no session afterwards. The test holds the clone's lock, so the checkout is
// still waiting for it when Close is called.
func TestClosingARunDuringItsCheckoutStartsNothing(t *testing.T) {
	m, d, ch, src := setup(t, func(toolCaller, string) string { return "ok" })
	unlock, err := m.Repos.lock(context.Background(), filepath.Join(m.Repos.Dir, filepath.FromSlash(src.Key)+".git"))
	if err != nil {
		t.Fatal(err)
	}
	r, err := m.Start(ch, src, []Reviewer{{Profile: "code"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	closed := make(chan error, 1)
	go func() { closed <- m.Close(ctx, r.ID) }()
	select {
	case <-r.prepared:
	case <-ctx.Done():
		t.Fatal("Close did not stop the checkout")
	}
	unlock()
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	d.mu.Lock()
	started := len(d.specs)
	d.mu.Unlock()
	if v := r.View(); v.Status != Closed || started != 0 {
		t.Fatalf("status %s, %d sessions started", v.Status, started)
	}
}

// Shutdown stops a checkout still in progress and waits for it, the checkout
// starts no session, and no run starts after it.
func TestShutdownWaitsForACheckoutAndRefusesNewRuns(t *testing.T) {
	m, d, ch, src := setup(t, func(toolCaller, string) string { return "ok" })
	unlock, err := m.Repos.lock(context.Background(), filepath.Join(m.Repos.Dir, filepath.FromSlash(src.Key)+".git"))
	if err != nil {
		t.Fatal(err)
	}
	r, err := m.Start(ch, src, []Reviewer{{Profile: "code"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { m.Shutdown(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Shutdown did not stop the checkout")
	}
	unlock()
	select {
	case <-r.prepared:
	default:
		t.Fatal("Shutdown returned before the checkout did")
	}
	if _, err := m.Start(ch, src, []Reviewer{{Profile: "code"}}, nil, nil); !errors.Is(err, ErrShuttingDown) {
		t.Fatalf("a run started after Shutdown: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.specs) != 0 {
		t.Fatalf("%d sessions started under Shutdown", len(d.specs))
	}
}

func TestShutdownEndsEveryLiveSession(t *testing.T) {
	m, _, ch, src := setup(t, func(toolCaller, string) string { return "ok" })
	r, err := m.Start(ch, src, []Reviewer{{Profile: "code"}}, nil, nil)
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

// gatedDriver holds Start until its gate opens, so a session can start while
// Shutdown runs.
type gatedDriver struct {
	*fakeDriver
	entered, gate chan struct{}
	mu            sync.Mutex
	started       []*fakeSession
}

func (d *gatedDriver) Start(ctx context.Context, spec agent.Spec) (agent.Session, error) {
	d.entered <- struct{}{}
	<-d.gate
	s, err := d.fakeDriver.Start(ctx, spec)
	d.mu.Lock()
	d.started = append(d.started, s.(*fakeSession))
	d.mu.Unlock()
	return s, err
}

// A session that starts once Shutdown has looked at the sessions is closed
// rather than left running, and Shutdown returns only after the run's
// goroutines have written its record for the last time.
func TestShutdownClosesASessionThatStartsUnderItAndWaitsForTheRun(t *testing.T) {
	m, d, ch, src := setup(t, func(toolCaller, string) string { return "ok" })
	g := &gatedDriver{fakeDriver: d, entered: make(chan struct{}), gate: make(chan struct{})}
	m.Drivers = map[string]agent.Driver{"fake": g}
	r, err := m.Start(ch, src, []Reviewer{{Profile: "code"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-g.entered // the checkout is done, and the session is starting
	done := make(chan struct{})
	go func() { m.Shutdown(); close(done) }()
	for !m.down.Load() {
		time.Sleep(time.Millisecond)
	}
	close(g.gate)
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Shutdown did not return")
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.started) != 1 {
		t.Fatalf("%d sessions started", len(g.started))
	}
	closed := make(chan struct{})
	go func() {
		for range g.started[0].events { // the Info event Start queued
		}
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("the session that started under Shutdown was left running")
	}
	// Nothing writes the run's record once Shutdown has returned.
	path := r.path
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if after, err := os.Stat(path); err != nil || !after.ModTime().Equal(before.ModTime()) {
		t.Fatalf("the run's record was written after Shutdown returned: %v", err)
	}
}

func TestARunOutlivesARestartAndItsConversationResumes(t *testing.T) {
	m, d, ch, src := setup(t, func(call toolCaller, _ string) string {
		call("propose_draft", map[string]any{"severity": "low", "title": "first", "description": "d",
			"anchor": map[string]any{"path": "calc.go", "start_line": 3}})
		return "done"
	})
	r, err := m.Start(ch, src, []Reviewer{{Profile: "code"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	wait(t, r, idle)
	m.Shutdown()
	wait(t, r, func(v RunView) bool { return v.Sessions[0].Status == Ended })

	// A new vuoro, on the same state directory.
	m2 := &Manager{StateDir: m.StateDir, Repos: m.Repos, Drivers: m.Drivers, Drafts: drafts.NewServer(), MCPURL: m.MCPURL}
	if err := m2.Load(); err != nil {
		t.Fatal(err)
	}
	r2 := m2.Get(r.ID)
	if r2 == nil {
		t.Fatal("the run did not come back")
	}
	if v := r2.View(); v.Sessions[0].Status != Ended || len(v.Drafts) != 1 || len(v.Sessions[0].Log) == 0 {
		t.Fatalf("reloaded %+v", v)
	}
	var refusal string
	d.mu.Lock()
	d.act = func(call toolCaller, _ string) string {
		refusal, _ = call("propose_draft", map[string]any{"severity": "low", "title": "off the diff", "description": "d",
			"anchor": map[string]any{"path": "calc.go", "start_line": 40}})
		call("propose_draft", map[string]any{"severity": "low", "title": "second", "description": "d"})
		return "resumed"
	}
	d.mu.Unlock()
	// The reloaded run serves its drafts from the new server, so point the
	// fake at it.
	hs := httptest.NewServer(m2.Drafts)
	defer hs.Close()
	m2.MCPURL = hs.URL + "/mcp/"
	if err := m2.Say(r.ID, "s1", "anything else?"); err != nil {
		t.Fatal(err)
	}
	v := wait(t, r2, func(v RunView) bool { return v.Sessions[0].Status == Idle })
	d.mu.Lock()
	last := d.specs[len(d.specs)-1]
	d.mu.Unlock()
	if last.Resume != "ref-1" {
		t.Fatalf("the session started fresh, resume %q", last.Resume)
	}
	if !strings.Contains(refusal, "anchor.notInDiff") {
		t.Fatalf("the reloaded run stopped checking anchors: %q", refusal)
	}
	if len(v.Drafts) != 2 || v.Drafts[1].ID != "d2" {
		t.Fatalf("drafts %+v", v.Drafts)
	}
	m2.Close(context.Background(), r.ID)
}

func TestReviewersRunInParallelAndTheSynthesizerMergesAfterThem(t *testing.T) {
	var mu sync.Mutex
	inFlight, most, reviewersDone := 0, 0, 0
	var synthSaw string
	m, _, ch, src := setup(t, func(call toolCaller, text string) string {
		if strings.Contains(text, "Merge their drafts") {
			mu.Lock()
			done := reviewersDone
			mu.Unlock()
			if done != 2 {
				return "started early"
			}
			synthSaw, _ = call("list_drafts", nil)
			var list struct{ Drafts []drafts.Draft }
			json.Unmarshal([]byte(synthSaw), &list)
			var ids []string
			for _, d := range list.Drafts {
				ids = append(ids, d.ID)
			}
			call("propose_draft", map[string]any{"severity": "high", "title": "Add subtracts", "description": "Both reviewers.",
				"anchor": map[string]any{"path": "calc.go", "start_line": 3}, "sources": ids})
			return "Merged.\nSet aside: none"
		}
		mu.Lock()
		inFlight++
		most = max(most, inFlight)
		mu.Unlock()
		time.Sleep(200 * time.Millisecond) // long enough to overlap
		call("propose_draft", map[string]any{"severity": "high", "title": "Add subtracts", "description": "d"})
		mu.Lock()
		inFlight--
		reviewersDone++
		mu.Unlock()
		return "done"
	})
	r, err := m.Start(ch, src, []Reviewer{{Profile: "code"}, {Profile: "documentation"}}, &Reviewer{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// A launched session is idle for a moment before its first message is
	// sent, so the wait is for its first turn to have completed.
	v := wait(t, r, func(v RunView) bool {
		return len(v.Sessions) == 3 && v.Sessions[2].Status == Idle && v.Sessions[2].FirstComplete
	})
	if most != 2 {
		t.Fatalf("at most %d reviewers ran at once", most)
	}
	s := v.Sessions[2]
	if s.Role != "synthesizer" || s.Log[len(s.Log)-1].Text != "Merged.\nSet aside: none" {
		t.Fatalf("synthesizer %+v", s)
	}
	var merged []drafts.Draft
	for _, d := range v.Drafts {
		if d.Session == "s3" {
			merged = append(merged, d)
		}
	}
	if len(merged) == 0 || len(merged[0].Sources) != 2 {
		t.Fatalf("merged %+v, synthesizer saw %s", merged, synthSaw)
	}
	m.Close(context.Background(), r.ID)
}

func TestAVerifierMarksEveryFindingAfterTheSynthesizerAndIsSentBackOnce(t *testing.T) {
	var mu sync.Mutex
	var verifierSaw []string
	m, d, ch, src := setup(t, func(call toolCaller, text string) string {
		switch {
		case strings.HasPrefix(text, "Check these"):
			mu.Lock()
			verifierSaw = append(verifierSaw, text)
			mu.Unlock()
			if _, isErr := call("propose_draft", map[string]any{"severity": "low", "title": "x", "description": "y"}); !isErr {
				return "a verifier proposed a draft"
			}
			// It marks one finding and stops, so vuoro sends it back.
			call("rate_draft", map[string]any{"id": "d3", "level": "confirmed", "reason": "calc.go:3 returns a - b"})
			return "Marked one."
		case strings.HasPrefix(text, "These findings have no mark yet"):
			call("rate_draft", map[string]any{"id": "d4", "level": "doubtful", "reason": "nothing in calc.go divides"})
			return "Marked the rest."
		case strings.HasPrefix(text, "The reviewers have finished"):
			call("propose_draft", map[string]any{"severity": "high", "title": "Add subtracts", "description": "Both.", "sources": []string{"d1", "d2"}})
			call("propose_draft", map[string]any{"severity": "low", "title": "Divide by zero", "description": "One.", "sources": []string{"d2"}})
			return "Merged.\nSet aside: none"
		}
		call("propose_draft", map[string]any{"severity": "high", "title": "Add subtracts", "description": "d"})
		return "done"
	})
	// The run is finished once, when the verifier's second turn ends: not
	// between the reviewers and the synthesizer, nor before the verifier is
	// sent back.
	finished := make(chan RunView, 8)
	m.Finished = func(v RunView) { finished <- v }
	r, err := m.Start(ch, src, []Reviewer{{Profile: "code"}, {Profile: "documentation"}}, &Reviewer{}, &Reviewer{Model: "opus"})
	if err != nil {
		t.Fatal(err)
	}
	v := wait(t, r, func(v RunView) bool {
		return len(v.Sessions) == 4 && v.Sessions[3].Status == Idle && len(v.Sessions[3].Log) >= 4
	})
	select {
	case f := <-finished:
		if f.ID != r.ID || f.Status != Idle || !f.ReviewComplete || len(f.Sessions) != 4 || len(f.Sessions[3].Log) < 4 {
			t.Fatalf("finished early: %s, complete %v, %d sessions", f.Status, f.ReviewComplete, len(f.Sessions))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the run never said it finished")
	}
	select {
	case f := <-finished:
		t.Fatalf("finished twice: %+v", f.Status)
	case <-time.After(100 * time.Millisecond):
	}
	if s := v.Sessions[3]; s.Role != "verifier" || s.Model != "opus" {
		t.Fatalf("verifier %+v", s)
	}
	marks := map[string]drafts.Level{}
	for _, d := range v.Drafts {
		if d.Confidence != nil {
			marks[d.ID] = d.Confidence.Level
		}
	}
	// It marked the synthesizer's two findings, not the reviewers' drafts.
	if len(marks) != 2 || marks["d3"] != drafts.Confirmed || marks["d4"] != drafts.Doubtful {
		t.Fatalf("marks %v", marks)
	}
	if len(verifierSaw) != 1 || !strings.Contains(verifierSaw[0], `<finding id="d3"`) || strings.Contains(verifierSaw[0], `<finding id="d1"`) {
		t.Fatalf("the verifier was sent %q", verifierSaw)
	}
	d.mu.Lock()
	tools := map[string]string{}
	for i, spec := range d.specs {
		tools[fmt.Sprint(i)] = strings.Join(spec.Drafts.Tools, ",")
	}
	d.mu.Unlock()
	if tools["3"] != "list_drafts,rate_draft" || tools["2"] != "list_drafts,propose_draft,revise_draft,withdraw_draft" {
		t.Fatalf("allowlists %v", tools)
	}
	// The person's mark wins, and the record keeps both.
	if err := m.Override(r.ID, "d4", drafts.Confirmed, "seen in staging"); err != nil {
		t.Fatal(err)
	}
	m2 := &Manager{StateDir: m.StateDir, Drafts: m.Drafts}
	if err := m2.Load(); err != nil {
		t.Fatal(err)
	}
	back := m2.Get(r.ID).View()
	for _, d := range back.Drafts {
		if d.ID == "d4" && (d.Override == nil || d.Override.Level != drafts.Confirmed || d.Confidence.Level != drafts.Doubtful) {
			t.Fatalf("after a reload %+v / %+v", d.Override, d.Confidence)
		}
	}
	if back.Sessions[3].Role != "verifier" {
		t.Fatalf("reloaded role %q", back.Sessions[3].Role)
	}
	m.Close(context.Background(), r.ID)
}

func TestAskingForASuggestionStartsASmallSessionForThatDraftAlone(t *testing.T) {
	m, d, ch, src := setup(t, func(call toolCaller, text string) string {
		if strings.HasPrefix(text, "Write the suggested change for draft d1") {
			if !strings.Contains(text, "calc.go lines 3–3") || strings.Contains(text, "<diff>") {
				return "the request named no range, or carried the diff"
			}
			if out, isErr := call("revise_draft", map[string]any{"id": "d1", "severity": "low", "title": "x", "description": "d"}); !isErr {
				return "revised: " + out
			}
			call("set_suggestion", map[string]any{"id": "d1", "suggestion": "func Add(a, b int) int { return a + b }"})
			return "Suggested."
		}
		call("propose_draft", map[string]any{"severity": "high", "title": "Add subtracts", "description": "d",
			"anchor": map[string]any{"path": "calc.go", "start_line": 3}})
		call("propose_draft", map[string]any{"severity": "low", "title": "Old line", "description": "d",
			"anchor": map[string]any{"path": "calc.go", "side": "base", "start_line": 3}})
		return "done"
	})
	r, err := m.Start(ch, src, []Reviewer{{Profile: "code", Tool: "fake", Model: "m1"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	wait(t, r, idle)
	if err := m.AskSuggestion(r.ID, "d2"); err == nil {
		t.Fatal("a suggestion was asked of a base-side draft")
	}
	if err := m.AskSuggestion(r.ID, "d1"); err != nil {
		t.Fatal(err)
	}
	v := wait(t, r, func(v RunView) bool { return idle(v) && v.Drafts[0].Suggestion != nil && len(v.Sessions) == 2 })
	if *v.Drafts[0].Suggestion != "func Add(a, b int) int { return a + b }" || v.Drafts[0].Severity != "high" {
		t.Fatalf("draft %+v", v.Drafts[0])
	}
	g := v.Sessions[1]
	if g.Role != "suggester" || g.Target != "d1" || g.Model != "m1" || g.Profile != "suggestion" {
		t.Fatalf("suggestion session %+v", g)
	}
	// Nothing was sent to the drafting session.
	for _, e := range v.Sessions[0].Log {
		if e.Kind == "user" {
			t.Fatalf("the drafting session was sent %q", e.Text)
		}
	}
	d.mu.Lock()
	spec := d.specs[len(d.specs)-1]
	d.mu.Unlock()
	if spec.Resume != "" || !slices.Equal(spec.Drafts.Tools, []string{"read_draft", "set_suggestion"}) || !strings.Contains(spec.SystemPrompt, "set_suggestion") {
		t.Fatalf("suggestion spec resume %q tools %q", spec.Resume, spec.Drafts.Tools)
	}
	if !v.ReviewComplete {
		t.Fatal("a suggestion session made the review incomplete")
	}
	m2 := &Manager{StateDir: m.StateDir, Repos: m.Repos, Drivers: m.Drivers, Drafts: m.Drafts, MCPURL: m.MCPURL}
	if err := m2.Load(); err != nil {
		t.Fatal(err)
	}
	if back := m2.Get(r.ID).View().Sessions[1]; back.Role != "suggester" || back.Target != "d1" {
		t.Fatalf("reloaded %+v", back)
	}
	m.Decide(r.ID, "d1", drafts.Kept, "41", false)
	if err := m.AskSuggestion(r.ID, "d1"); err == nil {
		t.Fatal("a suggestion was asked of a kept draft")
	}
	m.Close(context.Background(), r.ID)
}

func TestForgejoForkHeadRunsWithoutTheSourceBranchAndSurvivesReload(t *testing.T) {
	m, _, ch, src := setup(t, func(call toolCaller, text string) string {
		if _, failed := call("propose_draft", map[string]any{"severity": "high", "title": "Add subtracts", "description": "Use addition.", "anchor": map[string]any{"path": "calc.go", "start_line": 3}}); failed {
			return "draft refused"
		}
		return "Found one problem."
	})
	// The target repository exposes a fork head only as its pull ref. There is
	// no feature branch left to fetch, so using source_branch would fail.
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", src.Remote}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git: %v\n%s", err, out)
		}
	}
	git("update-ref", "refs/pull/7/head", ch.Diff.Head)
	git("switch", "-q", "main")
	git("branch", "-D", "feature")
	ch.Kind = forge.Forgejo
	ch.Ref.Host = "forgejo.example"
	ch.WebURL = "https://forgejo.example/g/calc/pulls/7"
	src.Key = "forgejo.example/g/calc"
	src.Refspecs = []string{"+refs/pull/7/head:refs/vuoro/head", "+refs/heads/main:refs/vuoro/target/main"}
	r, err := m.Start(ch, src, []Reviewer{{Profile: "code"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	v := wait(t, r, idle)
	if v.Status != Idle || v.Kind != forge.Forgejo || v.Head != ch.Diff.Head || len(v.Drafts) != 1 {
		t.Fatalf("fork review: %+v", v)
	}
	b, err := os.ReadFile(filepath.Join(r.dir, "calc.go"))
	if err != nil || !strings.Contains(string(b), "a - b") {
		t.Fatal("fork head was not checked out")
	}
	r.save()
	loaded := &Manager{StateDir: m.StateDir}
	if err := loaded.Load(); err != nil {
		t.Fatal(err)
	}
	if restored := loaded.Get(r.ID); restored == nil || restored.View().Kind != forge.Forgejo || len(restored.View().Drafts) != 1 {
		t.Fatal("forge kind or drafts lost on reload")
	}
	if err := m.Close(context.Background(), r.ID); err != nil {
		t.Fatal(err)
	}
}

// Checking the tools again replaces the drivers while runs may be starting.
// Under -race, which CI runs on darwin, an unguarded map would be reported.
func TestDriversCanBeReplacedWhileRunsStart(t *testing.T) {
	m := &Manager{}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for range 200 {
			m.SetDrivers(map[string]agent.Driver{"fake": nil}, "fake")
		}
	}()
	go func() {
		defer wg.Done()
		for range 200 {
			m.driver("fake")
		}
	}()
	wg.Wait()
	if name, _, err := m.driver(""); err != nil || name != "fake" {
		t.Fatalf("the only driver: %q %v", name, err)
	}
}

// A reviewer that names no tool is the common case, not an edge: it is what
// every shipped preset uses and what the presets page calls "Any tool
// available". Before D63 it resolved only while exactly one driver was
// registered, so the second agent tool would have broken all of them at
// once.
func TestAReviewerThatNamesNoToolResolvesWithSeveralRegistered(t *testing.T) {
	both := func() map[string]agent.Driver {
		return map[string]agent.Driver{"claude": &fakeDriver{}, "terva": &fakeDriver{}}
	}
	for name, c := range map[string]struct {
		drivers map[string]agent.Driver
		pick    string
		want    string
	}{
		"the pick wins":            {both(), "terva", "terva"},
		"and it is not map order":  {both(), "claude", "claude"},
		"one driver needs no pick": {map[string]agent.Driver{"claude": &fakeDriver{}}, "", "claude"},
		"a pick of one registered": {map[string]agent.Driver{"claude": &fakeDriver{}}, "claude", "claude"},
	} {
		t.Run(name, func(t *testing.T) {
			m := &Manager{}
			m.SetDrivers(c.drivers, c.pick)
			// Several times, because the bug this replaces was a map range.
			for range 20 {
				if got, _, err := m.driver(""); err != nil || got != c.want {
					t.Fatalf("driver(\"\") = %q, %v; want %q", got, err, c.want)
				}
			}
		})
	}
}

// A pick naming a driver that is gone is not trusted, and neither failure
// says `no agent tool named ""`, which reads as though the preset named the
// empty string.
func TestAReviewerThatNamesNoToolFailsClearlyWithNothingToPick(t *testing.T) {
	for name, c := range map[string]struct {
		drivers map[string]agent.Driver
		pick    string
	}{
		"no tool at all":     {nil, ""},
		"a stale pick":       {map[string]agent.Driver{"claude": &fakeDriver{}, "terva": &fakeDriver{}}, "gone"},
		"several, no pick":   {map[string]agent.Driver{"claude": &fakeDriver{}, "terva": &fakeDriver{}}, ""},
		"a pick but no tool": {nil, "claude"},
	} {
		t.Run(name, func(t *testing.T) {
			m := &Manager{}
			m.SetDrivers(c.drivers, c.pick)
			_, _, err := m.driver("")
			if err == nil {
				t.Fatal("a reviewer with no tool resolved with nothing to resolve it to")
			}
			if strings.Contains(err.Error(), `named ""`) {
				t.Errorf("the message blames the preset for an empty name: %v", err)
			}
		})
	}
}

// Start records the tool it resolved, never the empty name the preset held.
// That is why the reload in launch never has to pick again, and the day
// somebody records rv.Tool instead, every run with no tool pinned stops
// coming back after a restart.
func TestARunWithNoToolPinnedRecordsTheToolItResolved(t *testing.T) {
	m, _, ch, src := setup(t, func(toolCaller, string) string { return "nothing to report" })
	m.SetDrivers(map[string]agent.Driver{"fake": m.Drivers["fake"], "other": &fakeDriver{}}, "fake")
	r, err := m.Start(ch, src, []Reviewer{{Profile: "code"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	wait(t, r, idle)
	if got := r.View().Sessions[0].Tool; got != "fake" {
		t.Fatalf("the session records %q, want the resolved tool", got)
	}
	m.Shutdown()

	// The reload resolves the recorded name, so it needs no pick of its own.
	m2 := &Manager{StateDir: m.StateDir, Repos: m.Repos, Drafts: drafts.NewServer(), MCPURL: m.MCPURL}
	m2.SetDrivers(map[string]agent.Driver{"fake": &fakeDriver{}, "other": &fakeDriver{}}, "")
	if err := m2.Load(); err != nil {
		t.Fatal(err)
	}
	back := m2.Get(r.ID)
	if back == nil {
		t.Fatal("the run did not come back")
	}
	if got := back.View().Sessions[0].Tool; got != "fake" {
		t.Fatalf("the reloaded session names %q", got)
	}
}

func TestAReviewAgainFollowsTheEarlierOneAndKeepsWhatThePersonDecided(t *testing.T) {
	var told string
	m, _, ch, src := setup(t, func(call toolCaller, text string) string {
		if !strings.Contains(text, "<earlier-finding") {
			call("propose_draft", map[string]any{"severity": "low", "title": "Name the sum", "description": "d"})
			call("propose_draft", map[string]any{"severity": "high", "title": "Add subtracts", "description": "d",
				"anchor": map[string]any{"path": "calc.go", "start_line": 3}})
			return "two"
		}
		told = text
		if _, isErr := call("propose_draft", map[string]any{"severity": "low", "title": "Name the sum, still", "description": "d", "earlier": "d1"}); isErr {
			return "the restated draft was refused"
		}
		call("propose_draft", map[string]any{"severity": "medium", "title": "Something new", "description": "d"})
		return "again"
	})
	first, err := m.Start(ch, src, []Reviewer{{Profile: "code", Tool: "fake"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	wait(t, first, idle)
	if err := m.Decide(first.ID, "d1", drafts.Dropped, "", false); err != nil {
		t.Fatal(err)
	}

	other := *ch
	other.Ref.Number = 8
	if _, err := m.Follow(&other, src, []Reviewer{{Profile: "code", Tool: "fake"}}, nil, nil, first.ID); err == nil {
		t.Fatal("a review of another change followed this one")
	}
	again, err := m.Follow(ch, src, []Reviewer{{Profile: "code", Tool: "fake"}}, nil, nil, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	v := wait(t, again, idle)
	for _, want := range []string{
		"This change was reviewed before, at " + ch.Diff.Head,
		`<earlier-finding id="d1" severity="low" where="the change as a whole" standing="dismissed" agent-written="true">`,
		`id="d2" severity="high" where="calc.go:3-3 (head)" standing="open"`,
	} {
		if !strings.Contains(told, want) {
			t.Fatalf("the reviewers were not told %q:\n%s", want, told)
		}
	}
	if len(v.Drafts) != 2 || v.Drafts[0].Earlier != "d1" || v.Drafts[0].Decision != drafts.Dropped || !v.Drafts[0].Carried || v.Drafts[1].Decision != "" {
		t.Fatalf("drafts %+v", v.Drafts)
	}
	if v.Follows != first.ID || len(v.Earlier) != 2 || v.Earlier[0].Standing != drafts.StandingDismissed {
		t.Fatalf("the run does not follow the earlier one: %+v %+v", v.Follows, v.Earlier)
	}
	if fv := first.View(); !slices.Equal(fv.FollowedBy, []string{again.ID}) {
		t.Fatalf("the earlier run does not link to the one following it: %v", fv.FollowedBy)
	}

	// The links and the earlier findings outlive a restart.
	m.Shutdown()
	m2 := &Manager{StateDir: m.StateDir, Repos: m.Repos, Drivers: m.Drivers, Drafts: drafts.NewServer(), MCPURL: m.MCPURL}
	if err := m2.Load(); err != nil {
		t.Fatal(err)
	}
	if v := m2.Get(again.ID).View(); v.Follows != first.ID || len(v.Earlier) != 2 {
		t.Fatalf("reloaded follower %+v %+v", v.Follows, v.Earlier)
	}
	if v := m2.Get(first.ID).View(); !slices.Equal(v.FollowedBy, []string{again.ID}) {
		t.Fatalf("reloaded earlier run %v", v.FollowedBy)
	}
}
