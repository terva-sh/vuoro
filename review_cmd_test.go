package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/terva-sh/vuoro/agent"
	"github.com/terva-sh/vuoro/app"
	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/forge/gitlab"
	"github.com/terva-sh/vuoro/review"
	"github.com/terva-sh/vuoro/review/drafts"
)

// fakeTool is an agent tool whose sessions propose the given drafts through
// the drafts MCP server and end their turn. With hang set, a turn never ends;
// with fail set, the tool does not start.
type fakeTool struct {
	proposals []map[string]any
	hang      bool
	fail      bool
	// during runs in each turn before it ends.
	during func()
}

func (fakeTool) Name() string                               { return "Fake" }
func (fakeTool) Probe(context.Context) (agent.Probe, error) { return agent.Probe{}, nil }
func (f fakeTool) Start(_ context.Context, spec agent.Spec) (agent.Session, error) {
	if f.fail {
		return nil, errors.New("the fake tool would not start")
	}
	return &fakeSession{tool: f, spec: spec, events: make(chan agent.Event, 16)}, nil
}

type fakeSession struct {
	tool   fakeTool
	spec   agent.Spec
	events chan agent.Event
	once   sync.Once
}

func (s *fakeSession) Ref() string                { return "ref" }
func (s *fakeSession) Events() <-chan agent.Event { return s.events }
func (s *fakeSession) Cancel() error              { return s.Close() }
func (s *fakeSession) Close() error               { s.once.Do(func() { close(s.events) }); return nil }
func (s *fakeSession) Send(context.Context, string) error {
	if s.tool.hang {
		return nil
	}
	go func() {
		for _, p := range s.tool.proposals {
			b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call",
				"params": map[string]any{"name": "propose_draft", "arguments": p}})
			req, _ := http.NewRequest("POST", s.spec.Drafts.Config["url"].(string), bytes.NewReader(b))
			for k, v := range s.spec.Drafts.Config["headers"].(map[string]string) {
				req.Header.Set(k, v)
			}
			if res, err := http.DefaultClient.Do(req); err == nil {
				res.Body.Close()
			}
		}
		if s.tool.during != nil {
			s.tool.during()
		}
		defer func() { recover() }() // the session may have closed meanwhile
		s.events <- agent.Event{Kind: agent.TurnDone, Usage: &agent.TurnUsage{CostUSD: 0.01}}
	}()
	return nil
}

// testCmd is vuoro review over the fake tool, with its state in a temporary
// directory and cfg as the config file.
func testCmd(t *testing.T, tool fakeTool, cfg *config.Config, dial func(string, string, config.Secret) (forge.Provider, error), src func(*app.App, *forge.Change) (review.Source, error)) (reviewCmd, string) {
	t.Helper()
	stateDir := t.TempDir()
	return testCmdIn(stateDir, tool, cfg, dial, src), stateDir
}

// testCmdIn is testCmd with its state in stateDir, so a second command sees
// the runs of the first.
func testCmdIn(stateDir string, tool fakeTool, cfg *config.Config, dial func(string, string, config.Secret) (forge.Provider, error), src func(*app.App, *forge.Change) (review.Source, error)) reviewCmd {
	if src == nil {
		src = (*app.App).Source
	}
	return reviewCmd{
		newApp: func(mcpURL string) (*app.App, *drafts.Server, error) {
			mcp := drafts.NewServer()
			return &app.App{
				StateDir: stateDir, Config: cfg, Dial: dial,
				Tools: map[string]agent.Driver{"fake": tool},
				Runs: &review.Manager{StateDir: stateDir, Repos: &review.Repos{Dir: filepath.Join(stateDir, "repos")},
					Drafts: mcp, MCPURL: mcpURL},
			}, mcp, nil
		},
		source: src,
		poll:   10 * time.Millisecond,
	}
}

// checkout is a repository whose feature branch changes a.go on main.
func checkout(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	git("init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n\nfunc A() int { return 1 }\n"), 0o644)
	git("add", ".")
	git("commit", "-q", "-m", "base")
	git("switch", "-q", "-c", "feature")
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n\nfunc A() int { return 2 }\n"), 0o644)
	git("commit", "-q", "-am", "Return two")
	return dir
}

var aFinding = map[string]any{"severity": "medium", "title": "Two is wrong", "description": "A should return one.",
	"anchor": map[string]any{"path": "a.go", "start_line": 3}}

func TestReviewALocalRange(t *testing.T) {
	dir := checkout(t)
	cmd, stateDir := testCmd(t, fakeTool{proposals: []map[string]any{aFinding}}, nil, nil, nil)
	var out, errb bytes.Buffer
	code := cmd.run([]string{"-C", dir, "-profile", "code", "main..feature"}, &out, &errb)
	if code != exitFindings {
		t.Fatalf("exit %d, stderr:\n%s\nstdout:\n%s", code, errb.String(), out.String())
	}
	for _, want := range []string{"# Review of the local commits of ", "Title: Return two", "1 finding, drafted by agents.", "## 1. Two is wrong", "- Where: `a.go` line 3"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in:\n%s", want, out.String())
		}
	}
	if !strings.Contains(errb.String(), "s1 reviewer code (fake): idle") {
		t.Errorf("no progress on stderr:\n%s", errb.String())
	}
	// The run is recorded, and its worktree is gone.
	runs, _ := filepath.Glob(filepath.Join(stateDir, "runs", "*", "run.json"))
	if len(runs) != 1 {
		t.Fatalf("%d run records", len(runs))
	}
	if wts, _ := filepath.Glob(filepath.Join(stateDir, "worktrees", "*")); len(wts) != 0 {
		t.Fatalf("worktrees left: %v", wts)
	}
}

func TestReviewJSONShape(t *testing.T) {
	dir := checkout(t)
	cmd, _ := testCmd(t, fakeTool{proposals: []map[string]any{aFinding}}, nil, nil, nil)
	var out, errb bytes.Buffer
	// Flags may follow the range.
	if code := cmd.run([]string{"-C", dir, "-profile", "code", "main..feature", "-json"}, &out, &errb); code != exitFindings {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	var env map[string]any
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("%v in %s", err, out.String())
	}
	for _, k := range []string{"schemaVersion", "kind", "run", "findings", "outcome", "sessions"} {
		if _, ok := env[k]; !ok {
			t.Errorf("no %q in %s", k, out.String())
		}
	}
	if env["schemaVersion"] != 1.0 || env["kind"] != "review-result" || env["outcome"] != "findings" {
		t.Fatalf("envelope %v", env)
	}
	run := env["run"].(map[string]any)
	for _, k := range []string{"id", "kind", "ref", "title", "head", "base", "status", "cost_usd"} {
		if _, ok := run[k]; !ok {
			t.Errorf("run has no %q", k)
		}
	}
	if run["kind"] != review.Local {
		t.Errorf("run kind %v", run["kind"])
	}
	findings := env["findings"].([]any)
	if len(findings) != 1 || findings[0].(map[string]any)["title"] != "Two is wrong" {
		t.Fatalf("findings %v", findings)
	}
	for _, s := range env["sessions"].([]any) {
		if log := s.(map[string]any)["log"]; log != nil {
			t.Fatalf("a transcript is in the envelope: %v", log)
		}
	}
}

func TestReviewExitStatuses(t *testing.T) {
	dir := checkout(t)
	run := func(tool fakeTool, args ...string) (int, string) {
		cmd, _ := testCmd(t, tool, nil, nil, nil)
		var out, errb bytes.Buffer
		code := cmd.run(append([]string{"-C", dir}, args...), &out, &errb)
		return code, out.String() + errb.String()
	}
	if code, out := run(fakeTool{}, "-profile", "code", "main..feature"); code != exitClean || !strings.Contains(out, "No findings.") {
		t.Errorf("clean: %d\n%s", code, out)
	}
	if code, out := run(fakeTool{fail: true}, "-profile", "code", "main..feature"); code != exitFailed || !strings.Contains(out, "failed") {
		t.Errorf("failed tool: %d\n%s", code, out)
	}
	if code, out := run(fakeTool{hang: true}, "-profile", "code", "-timeout", "300ms", "main..feature"); code != exitFailed || !strings.Contains(out, "did not finish") {
		t.Errorf("timeout: %d\n%s", code, out)
	}
	for _, args := range [][]string{
		{"-preset", "a", "-profile", "b"},
		{"-model", "m"},
		{"main", "feature"},
		{"-profile", "code", "feature..main"}, // nothing to review
		{"-preset", "nope", "main..feature"},
		{"-timeout", "0s"},
	} {
		if code, out := run(fakeTool{}, args...); code != exitUsage {
			t.Errorf("%v: %d, want usage\n%s", args, code, out)
		}
	}
	if code, out := run(fakeTool{}, "-profile", "code", "nope..feature"); code != exitFailed {
		t.Errorf("a bad revision: %d\n%s", code, out)
	}
	if code, _ := run(fakeTool{}, "-h"); code != exitClean {
		t.Errorf("-h: %d", code)
	}
}

func TestReviewInterrupt(t *testing.T) {
	dir := checkout(t)
	cmd, _ := testCmd(t, fakeTool{hang: true}, nil, nil, nil)
	sig := make(chan os.Signal, 1)
	cmd.interrupt = sig
	go func() { time.Sleep(200 * time.Millisecond); sig <- os.Interrupt }()
	var out, errb bytes.Buffer
	if code := cmd.run([]string{"-C", dir, "-profile", "code", "-json", "main..feature"}, &out, &errb); code != exitInterrupted {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), `"outcome": "interrupted"`) {
		t.Fatalf("outcome in %s", out.String())
	}
}

// TestReviewQuickPresetIsTheDefault runs the config's quick preset when no
// reviewer is named, rather than its first.
func TestReviewQuickPresetIsTheDefault(t *testing.T) {
	dir := checkout(t)
	cfg := &config.Config{Presets: []config.Preset{
		{Name: "standard", Reviewers: []config.Reviewer{{Profile: "code"}, {Profile: "security"}}},
		{Name: "quick", Reviewers: []config.Reviewer{{Profile: "documentation"}}},
	}}
	cmd, _ := testCmd(t, fakeTool{}, cfg, nil, nil)
	var out, errb bytes.Buffer
	if code := cmd.run([]string{"-C", dir, "main..feature"}, &out, &errb); code != exitClean {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "reviewer documentation on fake") || strings.Contains(out.String(), "security") {
		t.Fatalf("not the quick preset:\n%s", out.String())
	}
}

// TestReviewOnAFreshInstall is the command line's half of D61's default.
// With no configuration file, vuoro ships a preset named quick and the first
// preset is quick, so the command line and the window reach the same one.
// app.TestPlanOnAFreshInstall is the window's half.
func TestReviewOnAFreshInstall(t *testing.T) {
	dir := checkout(t)
	cmd, _ := testCmd(t, fakeTool{}, nil, nil, nil)
	var out, errb bytes.Buffer
	if code := cmd.run([]string{"-C", dir, "main..feature"}, &out, &errb); code != exitClean {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "reviewer code on fake") {
		t.Fatalf("not quick's code reviewer:\n%s", out.String())
	}
	if strings.Contains(out.String(), "security") || strings.Contains(out.String(), "documentation") {
		t.Fatalf("more than quick ran:\n%s", out.String())
	}
}

// TestReviewKeepsAnExistingConfigurationsDefault is the fourth criterion of
// TKT-01M55VT3H26HX25TB8864NPG6J. vuoro now ships a preset named quick, and
// the command line must keep matching that name against the person's own
// presets rather than the merged list. Otherwise someone whose configuration
// names presets but no quick would silently start running the shipped one.
func TestReviewKeepsAnExistingConfigurationsDefault(t *testing.T) {
	dir := checkout(t)
	cfg := &config.Config{Presets: []config.Preset{
		{Name: "mine", Reviewers: []config.Reviewer{{Profile: "documentation"}}},
	}}
	cmd, _ := testCmd(t, fakeTool{}, cfg, nil, nil)
	var out, errb bytes.Buffer
	if code := cmd.run([]string{"-C", dir, "main..feature"}, &out, &errb); code != exitClean {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "reviewer documentation on fake") {
		t.Fatalf("not their own first preset:\n%s", out.String())
	}
	if strings.Contains(out.String(), "reviewer code on fake") {
		t.Fatalf("the shipped quick preset displaced theirs:\n%s", out.String())
	}
}

// recordingGitLab serves merge request !3991 for reading, and records any
// other request as a write.
type recordingGitLab struct {
	mu     sync.Mutex
	writes []string
}

func (g *recordingGitLab) handler(t *testing.T) http.Handler {
	gql, err := os.ReadFile("forge/gitlab/testdata/mr3991.json")
	if err != nil {
		t.Fatal(err)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/graphql":
			w.Write(gql)
		case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/diffs"):
			w.Write([]byte(`[{"old_path":"a.go","new_path":"a.go","diff":"@@ -1,3 +1,3 @@\n package a\n \n-func A() int { return 1 }\n+func A() int { return 2 }\n"}]`))
		case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/draft_notes"):
			w.Write([]byte(`[]`))
		default:
			g.mu.Lock()
			g.writes = append(g.writes, r.Method+" "+r.URL.EscapedPath())
			g.mu.Unlock()
			http.Error(w, `{"message":"refused"}`, http.StatusForbidden)
		}
	})
}

// TestReviewOfAChangePostsNothing reviews a GitLab merge request with a
// token configured, and proves that nothing reaches the forge but reads.
func TestReviewOfAChangePostsNothing(t *testing.T) {
	g := &recordingGitLab{}
	forgeSrv := httptest.NewServer(g.handler(t))
	defer forgeSrv.Close()
	host := strings.TrimPrefix(forgeSrv.URL, "http://")
	t.Setenv("VUORO_TEST_TOKEN", "glpat-test")
	cfg := &config.Config{Forges: []config.Forge{{Name: "fake", Kind: "gitlab", URL: "https://" + host, Username: "me", TokenEnv: "VUORO_TEST_TOKEN"}}}
	dial := func(string, string, config.Secret) (forge.Provider, error) {
		tok, _ := (&config.Forge{TokenEnv: "VUORO_TEST_TOKEN"}).Token(context.Background())
		return gitlab.New(forgeSrv.URL, tok)
	}
	dir := checkout(t)
	src := func(_ *app.App, ch *forge.Change) (review.Source, error) {
		lt, err := review.LocalChange(context.Background(), "", dir, "main..feature")
		if err != nil {
			return review.Source{}, err
		}
		s := lt.Source
		s.Key = "fake/cli"
		return s, nil
	}
	cmd, _ := testCmd(t, fakeTool{proposals: []map[string]any{aFinding, {"severity": "low", "title": "General", "description": "About it all."}}}, cfg, dial, src)
	mrURL := (&url.URL{Scheme: "https", Host: host, Path: "/gitlab-org/cli/-/merge_requests/3991"}).String()
	var out, errb bytes.Buffer
	code := cmd.run([]string{"-profile", "code", mrURL}, &out, &errb)
	if code != exitFindings {
		t.Fatalf("exit %d: %s\n%s", code, errb.String(), out.String())
	}
	if !strings.Contains(out.String(), "2 findings") || !strings.Contains(out.String(), "Change: https://") {
		t.Fatalf("output:\n%s", out.String())
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.writes) != 0 {
		t.Fatalf("vuoro review wrote to the forge: %q", g.writes)
	}
}

// TestReviewMCPServerIsGuarded reaches the command's drafts MCP server while
// a run works, and is refused without the loopback Host or a session's own
// credential.
func TestReviewMCPServerIsGuarded(t *testing.T) {
	dir := checkout(t)
	cmd, _ := testCmd(t, fakeTool{hang: true}, nil, nil, nil)
	urls := make(chan string, 1)
	inner := cmd.newApp
	cmd.newApp = func(mcpURL string) (*app.App, *drafts.Server, error) {
		urls <- mcpURL
		return inner(mcpURL)
	}
	sig := make(chan os.Signal, 1)
	cmd.interrupt = sig
	done := make(chan int, 1)
	go func() {
		var out, errb bytes.Buffer
		done <- cmd.run([]string{"-C", dir, "-profile", "code", "main..feature"}, &out, &errb)
	}()
	mcpURL := <-urls
	time.Sleep(100 * time.Millisecond)
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	post := func(mut func(*http.Request)) int {
		req, _ := http.NewRequest("POST", mcpURL, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		mut(req)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}
	if code := post(func(r *http.Request) { r.Host = "evil.example:80" }); code != http.StatusForbidden {
		t.Errorf("a foreign Host: %d", code)
	}
	if code := post(func(r *http.Request) { r.Header.Set("Origin", "http://evil.example") }); code != http.StatusForbidden {
		t.Errorf("a foreign Origin: %d", code)
	}
	if code := post(func(*http.Request) {}); code < 400 {
		t.Errorf("no session credential: %d", code)
	}
	if code := post(func(r *http.Request) { r.Header.Set("Authorization", "Bearer not-a-session") }); code < 400 {
		t.Errorf("a made-up credential: %d", code)
	}
	sig <- os.Interrupt
	if code := <-done; code != exitInterrupted {
		t.Fatalf("exit %d", code)
	}
}

// reviewJSON runs cmd with args and -json, and returns the envelope.
func reviewJSON(t *testing.T, cmd reviewCmd, want int, args ...string) reviewResult {
	t.Helper()
	var out, errb bytes.Buffer
	if code := cmd.run(append(args, "-json"), &out, &errb); code != want {
		t.Fatalf("%v: exit %d, want %d\n%s", args, code, want, errb.String())
	}
	var res reviewResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("%v in %s", err, out.String())
	}
	return res
}

func TestReviewFollowsAnEarlierRun(t *testing.T) {
	dir := checkout(t)
	stateDir := t.TempDir()
	first := reviewJSON(t, testCmdIn(stateDir, fakeTool{proposals: []map[string]any{aFinding}}, nil, nil, nil), exitFindings,
		"-C", dir, "-profile", "code", "main..feature")
	earlierID := first.Findings[0].ID

	// A finished run the follow has nothing to do with. Its record carries
	// a field vuoro does not write, so a save of it would show.
	other := reviewJSON(t, testCmdIn(stateDir, fakeTool{}, nil, nil, nil), exitClean,
		"-C", checkout(t), "-profile", "code", "main..feature")
	otherPath := filepath.Join(stateDir, "runs", other.Run.ID, "run.json")
	b, _ := os.ReadFile(otherPath)
	bystander := append(bytes.TrimSuffix(bytes.TrimSpace(b), []byte("}")), []byte(`,"written_by_a_window":true}`)...)
	os.WriteFile(otherPath, bystander, 0o600)

	again := map[string]any{"earlier": earlierID}
	for k, v := range aFinding {
		again[k] = v
	}
	// While the run goes on, a window changes the earlier run's record.
	firstPath := filepath.Join(stateDir, "runs", first.Run.ID, "run.json")
	windowEdit := func() {
		var rec map[string]any
		b, _ := os.ReadFile(firstPath)
		json.Unmarshal(b, &rec)
		rec["written_by_a_window"] = true
		b, _ = json.Marshal(rec)
		os.WriteFile(firstPath, b, 0o600)
	}
	next := reviewJSON(t, testCmdIn(stateDir, fakeTool{proposals: []map[string]any{again, again}, during: windowEdit}, nil, nil, nil), exitFindings,
		"-C", dir, "-profile", "code", "-follows", first.Run.ID, "main..feature")
	if next.Run.Follows != first.Run.ID {
		t.Fatalf("follows %q, want %q", next.Run.Follows, first.Run.ID)
	}
	if len(next.Earlier) != 1 || next.Earlier[0].ID != earlierID || next.Earlier[0].Standing != drafts.StandingOpen {
		t.Fatalf("earlier %+v", next.Earlier)
	}
	if len(next.Findings) != 2 || next.Findings[0].Earlier != earlierID {
		t.Fatalf("findings %+v", next.Findings)
	}

	// The reviewers were told, and the earlier run links forward.
	var rec struct {
		First      string   `json:"first_message"`
		FollowedBy []string `json:"followed_by"`
	}
	b, _ = os.ReadFile(filepath.Join(stateDir, "runs", next.Run.ID, "run.json"))
	json.Unmarshal(b, &rec)
	if !strings.Contains(rec.First, `<earlier-finding id="`+earlierID+`"`) {
		t.Errorf("the first message does not list the earlier finding:\n%s", rec.First)
	}
	b, _ = os.ReadFile(firstPath)
	json.Unmarshal(b, &rec)
	if len(rec.FollowedBy) != 1 || rec.FollowedBy[0] != next.Run.ID {
		t.Errorf("the earlier run is followed by %v", rec.FollowedBy)
	}
	if !bytes.Contains(b, []byte(`"written_by_a_window":true`)) {
		t.Errorf("the command wrote its stale copy of the followed run over the window's change:\n%s", b)
	}
	if b, _ := os.ReadFile(otherPath); !bytes.Equal(b, bystander) {
		t.Errorf("following one run rewrote another's record:\n%s", b)
	}

	var out, errb bytes.Buffer
	// Two drafts restate the one earlier finding, which counts once.
	if code := testCmdIn(stateDir, fakeTool{proposals: []map[string]any{again, again}}, nil, nil, nil).run(
		[]string{"-C", dir, "-profile", "code", "-follows", first.Run.ID, "main..feature"}, &out, &errb); code != exitFindings {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	for _, want := range []string{
		"It follows run `" + first.Run.ID + "`, which had 1 finding; 1 is raised again here.",
		"- Raised before: finding `" + earlierID + "` of run `" + first.Run.ID + "`, which nobody had decided",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in:\n%s", want, out.String())
		}
	}
}

func TestReviewRefusesARunItCannotFollow(t *testing.T) {
	dir, other := checkout(t), checkout(t)
	stateDir := t.TempDir()
	first := reviewJSON(t, testCmdIn(stateDir, fakeTool{proposals: []map[string]any{aFinding}}, nil, nil, nil), exitFindings,
		"-C", dir, "-profile", "code", "main..feature")

	// A run that is not a review: an Ask conversation about the same commits.
	var rec map[string]any
	b, _ := os.ReadFile(filepath.Join(stateDir, "runs", first.Run.ID, "run.json"))
	json.Unmarshal(b, &rec)
	rec["id"], rec["mode"] = "askrun", review.AskMode
	os.MkdirAll(filepath.Join(stateDir, "runs", "askrun"), 0o700)
	b, _ = json.Marshal(rec)
	os.WriteFile(filepath.Join(stateDir, "runs", "askrun", "run.json"), b, 0o600)

	askPath := filepath.Join(stateDir, "runs", "askrun", "run.json")
	firstPath := filepath.Join(stateDir, "runs", first.Run.ID, "run.json")
	// A field vuoro does not write, so a save of the review would show.
	b, _ = os.ReadFile(firstPath)
	b = append(bytes.TrimSuffix(bytes.TrimSpace(b), []byte("}")), []byte(`,"written_by_a_window":true}`)...)
	os.WriteFile(firstPath, b, 0o600)
	askBefore, _ := os.ReadFile(askPath)
	firstBefore, _ := os.ReadFile(firstPath)
	for _, c := range []struct{ dir, follows, want string }{
		{dir, "nope", "no finished run nope to follow"},
		{dir, "askrun", "run askrun is not a review"},
		{other, first.Run.ID, "run " + first.Run.ID + " reviewed another change or checkout"},
	} {
		var out, errb bytes.Buffer
		code := testCmdIn(stateDir, fakeTool{}, nil, nil, nil).run([]string{"-C", c.dir, "-profile", "code", "-follows", c.follows, "main..feature"}, &out, &errb)
		if code != exitUsage || !strings.Contains(errb.String(), c.want) {
			t.Errorf("-follows %s from %s: exit %d, want usage saying %q\n%s", c.follows, c.dir, code, c.want, errb.String())
		}
	}
	if runs, _ := filepath.Glob(filepath.Join(stateDir, "runs", "*", "run.json")); len(runs) != 2 {
		t.Errorf("a refused follow started a run: %d records", len(runs))
	}
	// A refused run was read, and is left as it was.
	if b, _ := os.ReadFile(askPath); !bytes.Equal(b, askBefore) {
		t.Errorf("refusing to follow it rewrote the Ask run's record:\n%s", b)
	}
	if b, _ := os.ReadFile(firstPath); !bytes.Equal(b, firstBefore) {
		t.Errorf("refusing to follow it rewrote the review's record:\n%s", b)
	}
}
