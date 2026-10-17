package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/terva-sh/vuoro/agent"
)

// The tests run without the network. The test binary stands in for claude:
// TestMain sees fakeEnv and replays a recorded stream-json transcript, one
// turn for each message it reads, as claude does.
const (
	fakeEnv       = "VUORO_FAKE_CLAUDE"            // the recording to replay
	fakeArgsEnv   = "VUORO_FAKE_CLAUDE_ARGS"       // where to write the argv it got
	fakeVerEnv    = "VUORO_FAKE_CLAUDE_VERSION"    // what --version prints
	fakeHangEnv   = "VUORO_FAKE_CLAUDE_HANG"       // stop mid-turn and wait
	fakeNoReadEnv = "VUORO_FAKE_CLAUDE_NO_READ"    // block before reading stdin
	fakeOutEnv    = "VUORO_FAKE_CLAUDE_SIGNED_OUT" // auth status reports no login
)

func TestMain(m *testing.M) {
	if rec, ok := os.LookupEnv(fakeEnv); ok {
		fakeClaude(rec)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func fakeClaude(rec string) {
	args := os.Args[1:]
	switch {
	case len(args) == 1 && args[0] == "--version":
		v := os.Getenv(fakeVerEnv)
		if v == "" {
			v = MinVersion
		}
		os.Stdout.WriteString(v + " (Claude Code)\n")
		return
	case len(args) == 2 && args[0] == "auth" && args[1] == "status":
		if os.Getenv(fakeOutEnv) == "1" {
			// What claude prints with no credential anywhere: exit 1 too.
			os.Stdout.WriteString(`{"loggedIn":false,"authMethod":"none","apiProvider":"firstParty"}` + "\n")
			os.Exit(1)
		}
		os.Stdout.WriteString(`{"loggedIn":true,"authMethod":"claude.ai","email":"someone@example.invalid","subscriptionType":"team"}` + "\n")
		return
	}
	if p := os.Getenv(fakeArgsEnv); p != "" {
		b, _ := json.Marshal(args)
		_ = os.WriteFile(p, b, 0o600)
	}
	if os.Getenv(fakeNoReadEnv) == "1" {
		time.Sleep(time.Hour)
		return
	}
	turns := recordedTurns(rec)
	in := bufio.NewScanner(os.Stdin)
	for i := 0; in.Scan() && i < len(turns); i++ {
		for _, line := range turns[i] {
			if os.Getenv(fakeHangEnv) == "1" && strings.Contains(line, `"text_delta"`) {
				os.Stdout.WriteString(line + "\n")
				time.Sleep(time.Hour) // until the driver stops the process
			}
			os.Stdout.WriteString(line + "\n")
		}
	}
}

// recordedTurns splits a recording into turns, each ending at its result.
func recordedTurns(path string) [][]string {
	b, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	var turns [][]string
	var cur []string
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		cur = append(cur, line)
		if strings.HasPrefix(line, `{"type":"result"`) || strings.Contains(line, `"type":"result"`) && strings.Contains(line, `"total_cost_usd"`) {
			turns = append(turns, cur)
			cur = nil
		}
	}
	return turns
}

// fake returns a driver whose claude replays the named recording.
func fake(t *testing.T, recording string) (*Driver, string) {
	t.Helper()
	rec, err := filepath.Abs(filepath.Join("testdata", recording))
	if err != nil {
		t.Fatal(err)
	}
	argsFile := filepath.Join(t.TempDir(), "args.json")
	t.Setenv(fakeEnv, rec)
	t.Setenv(fakeArgsEnv, argsFile)
	return &Driver{Path: os.Args[0], ConfigDir: t.TempDir()}, argsFile
}

// turn sends text and collects events up to the turn's end.
func turn(t *testing.T, s agent.Session, text string) []agent.Event {
	t.Helper()
	if err := s.Send(context.Background(), text); err != nil {
		t.Fatal(err)
	}
	return collect(t, s)
}

func collect(t *testing.T, s agent.Session) []agent.Event {
	t.Helper()
	var evs []agent.Event
	timeout := time.After(10 * time.Second)
	for {
		select {
		case ev, ok := <-s.Events():
			if !ok {
				return evs
			}
			evs = append(evs, ev)
			if ev.Kind == agent.TurnDone {
				return evs
			}
		case <-timeout:
			t.Fatalf("no TurnDone after 10s; got %d events", len(evs))
		}
	}
}

func kinds(evs []agent.Event, k agent.Kind) []agent.Event {
	var out []agent.Event
	for _, ev := range evs {
		if ev.Kind == k {
			out = append(out, ev)
		}
	}
	return out
}

var drafts = &agent.MCPServer{
	Name:   "vuoro",
	Config: map[string]any{"type": "http", "url": "http://127.0.0.1:1/mcp", "headers": map[string]string{"Authorization": "Bearer secret-in-a-file"}},
	Tools:  []string{"propose_draft"},
}

func TestATwoTurnConversationStreams(t *testing.T) {
	d, argsFile := fake(t, "review.jsonl")
	s, err := d.Start(context.Background(), agent.Spec{Dir: t.TempDir(), SystemPrompt: "review", Model: "haiku", Drafts: drafts})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	first := turn(t, s, "Review calc.go.")
	info := kinds(first, agent.Info)
	if len(info) != 1 || info[0].Info.Tool != "claude" || info[0].Info.Version != "2.1.285" || info[0].Info.Account != "login" {
		t.Fatalf("info: %+v", info)
	}
	var text strings.Builder
	for _, ev := range kinds(first, agent.Text) {
		text.WriteString(ev.Text)
	}
	if text.Len() == 0 {
		t.Error("no streamed text in the first turn")
	}
	calls := kinds(first, agent.ToolCall)
	if !slices.ContainsFunc(calls, func(ev agent.Event) bool { return ev.Tool.Name == "mcp__vuoro__propose_draft" }) {
		t.Errorf("no draft proposed: %+v", calls)
	}
	if n := len(kinds(first, agent.ToolResult)); n != len(calls) {
		t.Errorf("%d calls, %d results", len(calls), n)
	}
	u := kinds(first, agent.Usage)
	if len(u) != 1 || u[0].Usage.CostUSD <= 0 || u[0].Usage.Output <= 0 {
		t.Fatalf("usage: %+v", u)
	}
	if done := first[len(first)-1]; done.Err != nil || done.Cancelled {
		t.Fatalf("first turn ended %+v", done)
	}

	second := turn(t, s, "Run a shell command.")
	if done := second[len(second)-1]; done.Kind != agent.TurnDone || done.Err != nil {
		t.Fatalf("second turn ended %+v", done)
	}
	// total_cost_usd runs for the whole process, so the second turn's cost
	// is the difference, not the running total.
	// In this recording the second turn cost far less than the first, so a
	// running total would show up as more.
	if u2 := kinds(second, agent.Usage)[0].Usage.CostUSD; u2 <= 0 || u2 >= u[0].Usage.CostUSD {
		t.Errorf("second turn cost %v, first %v: a running total, not the turn's", u2, u[0].Usage.CostUSD)
	}

	// The lockdown reached the process.
	var got []string
	b, _ := os.ReadFile(argsFile)
	_ = json.Unmarshal(b, &got)
	for _, want := range [][]string{
		{"--tools", "Read,Grep,Glob"},
		{"--restricted"},
		{"--strict-mcp-config"},
		{"--permission-mode", "dontAsk"},
		{"--permission-prompts", "none"},
		{"--allowedTools", "Read,Grep,Glob,mcp__vuoro__propose_draft"},
		{"--setting-sources", ""},
		{"--disable-slash-commands"},
		{"--model", "haiku"},
		{"--session-id", s.Ref()},
	} {
		if !containsRun(got, want) {
			t.Errorf("argv lacks %q: %q", want, got)
		}
	}
	// The MCP config, which can carry a credential, goes in the configured
	// directory, never in argv. Unix enforces mode 0600; Windows uses ACLs
	// rather than the Unix permission bits reported by os.Stat.
	for _, a := range got {
		if strings.Contains(a, "secret-in-a-file") {
			t.Fatal("a credential reached argv")
		}
	}
	cfg := got[slices.Index(got, "--mcp-config")+1]
	fi, err := os.Stat(cfg)
	if err != nil {
		t.Fatalf("mcp config: %v", err)
	}
	if !fi.Mode().IsRegular() || filepath.Dir(cfg) != d.ConfigDir {
		t.Error("mcp config is not a regular file in the configured directory")
	}
	if runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Errorf("mcp config mode %v", fi.Mode().Perm())
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	for range s.Events() {
	}
	if _, err := os.Stat(cfg); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("mcp config still there after Close: %v", err)
	}
}

func containsRun(args, run []string) bool {
	for i := range args {
		if i+len(run) <= len(args) && slices.Equal(args[i:i+len(run)], run) {
			return true
		}
	}
	return false
}

func TestCancelStopsTheProcessAndEndsTheTurnAsCancelled(t *testing.T) {
	d, _ := fake(t, "review.jsonl")
	t.Setenv(fakeHangEnv, "1")
	s, err := d.Start(context.Background(), agent.Spec{Dir: t.TempDir(), SystemPrompt: "review", Drafts: drafts})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Send(context.Background(), "Review calc.go."); err != nil {
		t.Fatal(err)
	}
	for ev := range s.Events() { // wait until the turn is streaming
		if ev.Kind == agent.TurnDone {
			t.Fatalf("the turn ended before it streamed: %+v", ev)
		}
		if ev.Kind == agent.Text {
			break
		}
	}
	if err := s.Cancel(); err != nil {
		t.Fatal(err)
	}
	evs := collect(t, s)
	if done := evs[len(evs)-1]; done.Kind != agent.TurnDone || !done.Cancelled || done.Err != nil {
		t.Fatalf("turn ended %+v", done)
	}
	if _, ok := <-s.Events(); ok {
		t.Fatal("events still open after cancel")
	}
	if err := s.Send(context.Background(), "again"); err == nil {
		t.Fatal("Send after Cancel succeeded")
	}
	ps := s.(*session).cmd.ProcessState
	if ps == nil {
		t.Fatal("the process was not reaped")
	}
}

func TestASessionOfferingAnotherToolIsEndedAtOnce(t *testing.T) {
	d, _ := fake(t, "lockdown.jsonl") // its init offers Bash
	s, err := d.Start(context.Background(), agent.Spec{Dir: t.TempDir(), SystemPrompt: "review"})
	if err != nil {
		t.Fatal(err)
	}
	evs := turn(t, s, "Use Bash.")
	done := evs[len(evs)-1]
	if !errors.Is(done.Err, ErrLockdown) {
		t.Fatalf("turn ended %+v, want ErrLockdown", done)
	}
	for _, ev := range evs {
		if ev.Kind == agent.ToolCall || ev.Kind == agent.Text || ev.Kind == agent.Info {
			t.Fatalf("an event got through after the breach: %+v", ev)
		}
	}
}

func TestADeniedCallIsReported(t *testing.T) {
	d, _ := fake(t, "denied.jsonl")
	s, err := d.Start(context.Background(), agent.Spec{Dir: t.TempDir(), SystemPrompt: "review"})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	denied := kinds(turn(t, s, "Use Bash."), agent.Denied)
	if len(denied) != 1 || denied[0].Tool.Name != "Bash" || !strings.Contains(denied[0].Tool.Input, "canary") {
		t.Fatalf("denied: %+v", denied)
	}
}

func TestResumeContinuesTheNamedSession(t *testing.T) {
	d, argsFile := fake(t, "review.jsonl")
	s, err := d.Start(context.Background(), agent.Spec{Dir: t.TempDir(), SystemPrompt: "review", Drafts: drafts, Resume: "11111111-2222-4333-8444-555555555555"})
	if err != nil {
		t.Fatal(err)
	}
	turn(t, s, "again")
	s.Close()
	var got []string
	b, _ := os.ReadFile(argsFile)
	_ = json.Unmarshal(b, &got)
	if !containsRun(got, []string{"--resume", "11111111-2222-4333-8444-555555555555"}) || slices.Contains(got, "--session-id") {
		t.Fatalf("argv %q", got)
	}
	if s.Ref() != "11111111-2222-4333-8444-555555555555" {
		t.Fatalf("ref %q", s.Ref())
	}
}

func TestProbe(t *testing.T) {
	d, _ := fake(t, "review.jsonl")
	p, err := d.Probe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if p.Version != MinVersion || p.Account != "claude.ai team" {
		t.Fatalf("probe %+v", p)
	}
	if strings.Contains(p.Account, "@") {
		t.Fatal("the account kind carries an identity")
	}

	t.Setenv(fakeVerEnv, "2.0.9")
	_, err = d.Probe(context.Background())
	if !errors.Is(err, agent.ErrTooOld) || !strings.Contains(err.Error(), "2.0.9") || !strings.Contains(err.Error(), MinVersion) {
		t.Fatalf("old claude: %v", err)
	}

	missing := &Driver{Path: filepath.Join(t.TempDir(), "no-claude-here")}
	_, err = missing.Probe(context.Background())
	if !errors.Is(err, agent.ErrMissing) || !strings.Contains(err.Error(), MinVersion) {
		t.Fatalf("missing claude: %v", err)
	}
}

// fakeShell writes a stand-in login shell. It records the argv it was given,
// which is how a test proves no credential value was passed to it, and then
// runs body. Evaluating the script directly sources none of the person's own
// profile, so the test stays hermetic.
func fakeShell(t *testing.T, body string) (shell, log string) {
	t.Helper()
	dir := t.TempDir()
	log = filepath.Join(dir, "argv.log")
	shell = filepath.Join(dir, "fakesh")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"" + log + "\"\n" + body + "\n"
	if err := os.WriteFile(shell, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return shell, log
}

// TestShellLoginNamesTheVariableAndNeverItsValue is the Dock launch: the
// process has no credential, the person's login shell does, and vuoro must
// establish that without ever reading what the shell holds
// (TKT-01M55VRM16QASJ8CER793CR9NF).
func TestShellLoginNamesTheVariableAndNeverItsValue(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows has no $SHELL to ask")
	}
	// Not a credential. It stands for one, so that the test can prove it
	// never reached the command line.
	const standIn = "stand-in-value-9fbe"
	shell, log := fakeShell(t, `eval "$2"`)
	t.Setenv("SHELL", shell)
	t.Setenv("ANTHROPIC_API_KEY", standIn)

	if !shellLogin(context.Background()) {
		t.Error("a key set in the login shell was not seen")
	}
	asked, err := os.ReadFile(log)
	if err != nil {
		t.Fatalf("the shell was never asked: %v", err)
	}
	if !strings.Contains(string(asked), "ANTHROPIC_API_KEY") {
		t.Errorf("the shell was asked %q, which names no variable", asked)
	}
	if strings.Contains(string(asked), standIn) {
		t.Errorf("the value reached the command line: %q", asked)
	}
}

// TestShellLoginSaysNothingItCannotShow covers every way the shell can fail
// to answer. None of them may turn into a claim about the person's setup,
// because absence of evidence is not evidence here.
func TestShellLoginSaysNothingItCannotShow(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows has no $SHELL to ask")
	}
	hang, _ := fakeShell(t, "sleep 30")
	silent, _ := fakeShell(t, "exit 0")
	noisy, _ := fakeShell(t, "printf 'a prompt, and an escape sequence\\n'; eval \"$2\"")
	grumpy, _ := fakeShell(t, "eval \"$2\"; exit 1")
	absent := filepath.Join(t.TempDir(), "no-shell-here")

	for name, c := range map[string]struct {
		shell string
		set   bool // the stand-in variable is set in the environment
		wait  time.Duration
		want  bool
	}{
		"no SHELL at all":               {shell: "", set: true},
		"SHELL names nothing":           {shell: absent, set: true},
		"the shell blocks":              {shell: hang, set: true, wait: 100 * time.Millisecond},
		"the shell answers nothing":     {shell: silent, set: true},
		"nothing is set in the shell":   {shell: noisy, set: false},
		"a prompt precedes the answer":  {shell: noisy, set: true, want: true},
		"the shell answers, then fails": {shell: grumpy, set: true, want: true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("SHELL", c.shell)
			for _, v := range loginVars {
				t.Setenv(v, "") // empty is unset to ${v:+1}
			}
			if c.set {
				t.Setenv("ANTHROPIC_AUTH_TOKEN", "stand-in-value")
			}
			ctx := context.Background()
			if c.wait > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, c.wait)
				defer cancel()
			}
			if got := shellLogin(ctx); got != c.want {
				t.Errorf("shellLogin = %v, want %v", got, c.want)
			}
		})
	}
}

// TestProbeAsksTheShellOnlyWhenSignedOut holds the cost down and keeps the
// person's profile out of the common path: a tool that reports a login is
// never a reason to run their shell.
func TestProbeAsksTheShellOnlyWhenSignedOut(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows has no $SHELL to ask")
	}
	shell, log := fakeShell(t, `eval "$2"`)
	t.Setenv("SHELL", shell)
	t.Setenv("ANTHROPIC_API_KEY", "stand-in-value")
	d, _ := fake(t, "review.jsonl")

	p, err := d.Probe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if p.Account == "" {
		t.Fatal("the fake claude is meant to report a login")
	}
	if p.ShellLogin {
		t.Error("a signed-in tool reported the shell's login as well")
	}
	if _, err := os.Stat(log); !errors.Is(err, os.ErrNotExist) {
		t.Error("the login shell was run while the tool was signed in")
	}

	t.Setenv(fakeOutEnv, "1")
	switch p, err = d.Probe(context.Background()); {
	case err != nil:
		t.Fatal(err)
	case p.Account != "":
		t.Errorf("account %q, want none", p.Account)
	case !p.ShellLogin:
		t.Error("the key in the login shell was not reported")
	}
}

func TestVersionsCompare(t *testing.T) {
	for _, c := range []struct {
		a, b string
		ok   bool
	}{
		{"2.1.285", "2.1.285", true},
		{"2.1.286", "2.1.285", true},
		{"2.2.0", "2.1.285", true},
		{"10.0.0", "2.1.285", true},
		{"2.1.99", "2.1.285", false},
		{"1.9.999", "2.1.285", false},
		{"", "2.1.285", false},
	} {
		if got := atLeast(c.a, c.b); got != c.ok {
			t.Errorf("atLeast(%q, %q) = %v", c.a, c.b, got)
		}
	}
}

func TestTheChildIsNotToldItIsNested(t *testing.T) {
	env := childEnv([]string{"HOME=/h", "CLAUDECODE=1", "CLAUDE_CODE_ENTRYPOINT=cli", "PATH=/bin"})
	if !slices.Equal(env, []string{"HOME=/h", "PATH=/bin"}) {
		t.Fatalf("env %q", env)
	}
}

func TestLoginFailureIsReportedWithoutProviderText(t *testing.T) {
	s := &session{events: make(chan agent.Event, 4)}
	s.handle(streamEvent{Type: "result", Subtype: "error_during_execution", IsError: true, Result: "Not logged in · Please run /login"})
	<-s.events // usage
	done := <-s.events
	if !errors.Is(done.Err, agent.ErrUnauthenticated) {
		t.Fatalf("login error: %v", done.Err)
	}
}

func TestCancelInterruptsABlockedInputWrite(t *testing.T) {
	d, _ := fake(t, "review.jsonl")
	t.Setenv(fakeNoReadEnv, "1")
	s, err := d.Start(context.Background(), agent.Spec{Dir: t.TempDir(), SystemPrompt: "review", Drafts: drafts})
	if err != nil {
		t.Fatal(err)
	}
	sent := make(chan error, 1)
	go func() { sent <- s.Send(context.Background(), strings.Repeat("large input ", 1<<20)) }()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		ss := s.(*session)
		ss.mu.Lock()
		busy := ss.busy
		ss.mu.Unlock()
		if busy {
			break
		}
		time.Sleep(time.Millisecond)
	}
	cancelled := make(chan error, 1)
	go func() { cancelled <- s.Cancel() }()
	select {
	case err := <-cancelled:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Cancel blocked behind stdin Write")
	}
	select {
	case <-sent:
	case <-time.After(time.Second):
		t.Fatal("stdin write survived process cancellation")
	}
	if s.(*session).cmd.ProcessState == nil {
		t.Fatal("cancelled process was not reaped")
	}
}
