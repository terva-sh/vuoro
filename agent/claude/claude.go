// Package claude drives Claude Code as a reviewer, over
// `claude -p --input-format stream-json --output-format stream-json`, the
// protocol its own SDKs speak (docs/plan.md section 6.6).
//
// vuoro passes no model credential. Claude Code uses the person's own login
// or key.
package claude

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/terva-sh/vuoro/agent"
)

// MinVersion is the oldest Claude Code whose lockdown was proven
// (spikes/agentdriver).
const MinVersion = "2.1.285"

// readTools are the only built-in tools a reviewer gets.
var readTools = []string{"Read", "Grep", "Glob"}

// editTools are the built-in tools the editor gets (D40). --restricted keeps
// the file tools in its worktree, and the sandbox confines Bash.
var editTools = []string{"Read", "Grep", "Glob", "Edit", "Write", "Bash"}

// ErrLockdown means the session offered a tool the lockdown does not allow.
// The driver ends such a session at once rather than let it run.
var ErrLockdown = errors.New("claude: the session offered a tool outside its lockdown")

// Driver drives the claude binary.
type Driver struct {
	// Path is the binary. Empty means `claude` on PATH.
	Path string
	// ConfigDir is where a session's MCP configuration is written, with mode
	// 0600. Empty means the system temporary directory.
	ConfigDir string
}

var _ agent.Driver = (*Driver)(nil)

func (d *Driver) Name() string { return "Claude Code" }

func (d *Driver) binary() (string, error) {
	name := d.Path
	if name == "" {
		name = "claude"
	}
	p, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("claude: %w: install Claude Code %s or later", agent.ErrMissing, MinVersion)
	}
	return p, nil
}

// Probe finds claude, checks its version, and reads the kind of login it
// will use. It reads only the login method and subscription type from
// `claude auth status`, never the identity beside them.
func (d *Driver) Probe(ctx context.Context) (agent.Probe, error) {
	path, err := d.binary()
	if err != nil {
		return agent.Probe{}, err
	}
	out, err := exec.CommandContext(ctx, path, "--version").Output()
	if err != nil {
		return agent.Probe{}, fmt.Errorf("claude --version: %w", err)
	}
	version := leadingVersion(string(out))
	p := agent.Probe{Path: path, Version: version}
	if !atLeast(version, MinVersion) {
		return p, fmt.Errorf("claude: %w: found %q, need %s or later", agent.ErrTooOld, strings.TrimSpace(string(out)), MinVersion)
	}
	if out, err := exec.CommandContext(ctx, path, "auth", "status").Output(); err == nil {
		var st struct {
			LoggedIn         bool   `json:"loggedIn"`
			AuthMethod       string `json:"authMethod"`
			SubscriptionType string `json:"subscriptionType"`
		}
		if json.Unmarshal(out, &st) == nil && st.LoggedIn {
			p.Account = strings.TrimSpace(st.AuthMethod + " " + st.SubscriptionType)
		}
	}
	if p.Account == "" {
		p.ShellLogin = shellLogin(ctx)
	}
	return p, nil
}

// shellMarker prefixes the one line vuoro reads out of the login shell. An
// interactive shell writes its prompt and escape sequences to stdout as well,
// so the answer is found by this marker rather than by reading the output
// whole.
const shellMarker = "vuoro-shell-login:"

// shellTimeout bounds the login shell, whose profile is the person's own code
// and may block on anything. Running out of time means no evidence, which is
// not the same as a negative, so the hint stays as it was.
const shellTimeout = 3 * time.Second

// shellLogin reports whether the person's login shell holds one of loginVars
// that this process does not. A window started from the Dock or from a login
// item gets launchd's environment, not the shell's, so Claude Code works in a
// terminal and reports no login here.
//
// It asks only whether each variable is set, never what it holds, so no
// credential enters this process, a log line, or the setup JSON. The shell
// runs with -lic because -l reaches a login profile and -i reaches .zshrc and
// .bashrc, where an export is more often written. A shell that fails, hangs,
// refuses those flags, or is not named at all leaves this false, since vuoro
// would rather say nothing than say something it cannot show.
func shellLogin(ctx context.Context) bool {
	if runtime.GOOS == "windows" {
		return false // no $SHELL to ask, and no launchd to lose one to
	}
	sh := os.Getenv("SHELL")
	if sh == "" {
		return false
	}
	var script strings.Builder
	script.WriteString(`echo "` + shellMarker)
	for _, v := range loginVars {
		script.WriteString("${" + v + ":+1}")
	}
	script.WriteString(`"`)
	ctx, cancel := context.WithTimeout(ctx, shellTimeout)
	defer cancel()
	// The exit status is not read. A shell that prints the answer and then
	// exits non-zero, which an interactive one may do, has still answered.
	out, _ := exec.CommandContext(ctx, sh, "-lic", script.String()).Output()
	_, answer, ok := strings.Cut(string(out), shellMarker)
	if !ok {
		return false
	}
	answer, _, _ = strings.Cut(answer, "\n")
	return strings.Contains(answer, "1")
}

var versionRE = regexp.MustCompile(`\d+\.\d+\.\d+`)

func leadingVersion(s string) string { return versionRE.FindString(s) }

// atLeast reports whether version a is at least b; both are x.y.z.
func atLeast(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	if len(pa) != 3 || len(pb) != 3 {
		return false
	}
	for i := range 3 {
		x, err1 := strconv.Atoi(pa[i])
		y, err2 := strconv.Atoi(pb[i])
		if err1 != nil || err2 != nil {
			return false
		}
		if x != y {
			return x > y
		}
	}
	return true
}

// args is the reviewer lockdown, proven in spikes/agentdriver. Each flag's
// reason is in docs/plan.md section 6.6.
func args(spec agent.Spec, mcpConfig, settings string, allowed []string, sessionArgs []string) []string {
	tools := readTools
	if spec.Editor != nil {
		tools = editTools
	}
	a := []string{
		"-p",
		"--input-format", "stream-json",
		"--output-format", "stream-json",
		"--verbose",
		"--include-partial-messages",
		"--tools", strings.Join(tools, ","),
		"--restricted",
		"--strict-mcp-config",
		"--mcp-config", mcpConfig,
		"--permission-mode", "dontAsk",
		"--permission-prompts", "none",
		"--allowedTools", strings.Join(allowed, ","),
		"--setting-sources", "",
		"--disable-slash-commands",
		"--system-prompt", spec.SystemPrompt,
	}
	if settings != "" {
		a = append(a, "--settings", settings)
	}
	if spec.Model != "" {
		a = append(a, "--model", spec.Model)
	}
	return append(a, sessionArgs...)
}

// Start starts a locked reviewer session in spec.Dir.
func (d *Driver) Start(ctx context.Context, spec agent.Spec) (agent.Session, error) {
	path, err := d.binary()
	if err != nil {
		return nil, err
	}
	if spec.Dir == "" {
		return nil, errors.New("claude: a session needs a working directory")
	}

	allowed := slices.Clone(readTools)
	if spec.Editor != nil {
		if spec.Drafts != nil {
			return nil, errors.New("claude: the editor gets no drafts server")
		}
		allowed = slices.Clone(editTools)
	}
	servers := map[string]any{}
	if spec.Drafts != nil {
		servers[spec.Drafts.Name] = spec.Drafts.Config
		for _, t := range spec.Drafts.Tools {
			allowed = append(allowed, "mcp__"+spec.Drafts.Name+"__"+t)
		}
	}
	cfgPath, err := writeConfig(d.ConfigDir, map[string]any{"mcpServers": servers})
	if err != nil {
		return nil, err
	}
	env := childEnv(os.Environ())
	settings := ""
	envFile := ""
	if spec.Editor != nil {
		// Claude Code's sandbox sets git's configuration for a command itself,
		// and drops GIT_CONFIG_GLOBAL, so the editor's variables are exported
		// again by the file it sources before each command.
		envFile, err = writeEnvFile(d.ConfigDir, spec.Editor.Env)
		var sb map[string]any
		if err == nil {
			sb, err = sandboxSettings(spec.Dir, *spec.Editor, envFile)
		}
		if err == nil {
			settings, err = writeConfig(d.ConfigDir, sb)
		}
		if err != nil {
			os.Remove(cfgPath)
			if envFile != "" {
				os.Remove(envFile)
			}
			return nil, err
		}
		env = append(editorEnv(os.Environ(), spec.Editor.Env), "CLAUDE_ENV_FILE="+envFile)
	}
	cleanup := func() {
		os.Remove(cfgPath)
		if settings != "" {
			os.Remove(settings)
		}
		if envFile != "" {
			os.Remove(envFile)
		}
	}

	ref := spec.Resume
	sessionArgs := []string{"--resume", ref}
	if ref == "" {
		ref = newUUID()
		sessionArgs = []string{"--session-id", ref}
	}

	cmd := exec.Command(path, args(spec, cfgPath, settings, allowed, sessionArgs)...)
	cmd.Dir = spec.Dir
	cmd.Env = env
	var stderr tail
	cmd.Stderr = &stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cleanup()
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cleanup()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		cleanup()
		return nil, fmt.Errorf("claude: start: %w", err)
	}
	s := &session{
		ref:     ref,
		cmd:     cmd,
		stdin:   stdin,
		stderr:  &stderr,
		cleanup: cleanup,
		allowed: allowed,
		events:  make(chan agent.Event, 256),
		exited:  make(chan struct{}),
	}
	go s.read(stdout)
	return s, nil
}

// writeConfig writes the MCP configuration to a new file of mode 0600, since
// a server's entry can carry a credential that must not go in argv.
func writeConfig(dir string, cfg map[string]any) (string, error) {
	b, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, "vuoro-claude-mcp-*.json")
	if err != nil {
		return "", fmt.Errorf("claude: mcp config: %w", err)
	}
	if err := f.Chmod(0o600); err == nil {
		_, err = f.Write(b)
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("claude: mcp config: %w", err)
	}
	return f.Name(), nil
}

var envName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// writeEnvFile writes vars as a shell file that exports each one, for
// CLAUDE_ENV_FILE.
func writeEnvFile(dir string, vars []string) (string, error) {
	var b strings.Builder
	for _, kv := range vars {
		k, v, _ := strings.Cut(kv, "=")
		if !envName.MatchString(k) {
			return "", fmt.Errorf("claude: editor variable %q is not a name", k)
		}
		fmt.Fprintf(&b, "export %s='%s'\n", k, strings.ReplaceAll(v, "'", `'\''`))
	}
	f, err := os.CreateTemp(dir, "vuoro-claude-env-*.sh")
	if err != nil {
		return "", fmt.Errorf("claude: editor env: %w", err)
	}
	if err := f.Chmod(0o600); err == nil {
		_, err = f.WriteString(b.String())
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("claude: editor env: %w", err)
	}
	return f.Name(), nil
}

// childEnv passes the person's environment through, so Claude Code finds its
// own login, and drops the markers that tell it that it is nested inside
// another Claude Code session.
func childEnv(env []string) []string {
	var out []string
	for _, kv := range env {
		if strings.HasPrefix(kv, "CLAUDECODE=") || strings.HasPrefix(kv, "CLAUDE_CODE_ENTRYPOINT=") {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// editorEnv is the editor's environment: what Claude Code needs to start and
// find its own login, and extra, and nothing else the person's environment
// holds, such as a forge token. Claude Code keeps its login variables, and
// scrubs them from every command it runs.
func editorEnv(env, extra []string) []string {
	keep := []string{"PATH", "HOME", "USER", "LOGNAME", "SHELL", "LANG", "LC_ALL", "LC_CTYPE", "TERM", "TMPDIR",
		"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_BASE_URL", "CLAUDE_CODE_OAUTH_TOKEN", "CLAUDE_CONFIG_DIR"}
	var out []string
	for _, kv := range env {
		k, _, _ := strings.Cut(kv, "=")
		if slices.Contains(keep, k) {
			out = append(out, kv)
		}
	}
	out = append(out, "CLAUDE_CODE_SUBPROCESS_ENV_SCRUB=1")
	return append(out, extra...)
}

// loginVars are the model credentials Claude Code may hold, which no command
// it runs may see.
var loginVars = []string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "CLAUDE_CODE_OAUTH_TOKEN"}

// sandboxSettings is the editor's sandbox (D40), passed with --settings, the
// one settings source the session loads. Commands read and write only dir and
// the caches, never the home directory, reach only the named domains, and
// never run unsandboxed. Without a working sandbox the session does not
// start.
func sandboxSettings(dir string, e agent.Editor, envFile string) (map[string]any, error) {
	abs := func(p string) (string, error) {
		p, err := filepath.Abs(p)
		if err != nil {
			return "", err
		}
		if r, err := filepath.EvalSymlinks(p); err == nil {
			p = r
		}
		return p, nil
	}
	root, err := abs(dir)
	if err != nil {
		return nil, fmt.Errorf("claude: editor worktree: %w", err)
	}
	readable, writable := []string{root}, []string{}
	for _, c := range e.Caches {
		p, err := abs(c)
		if err != nil {
			return nil, fmt.Errorf("claude: editor cache: %w", err)
		}
		readable, writable = append(readable, p), append(writable, p)
	}
	if envFile != "" { // the file the sandboxed shell sources, read-only
		p, err := abs(envFile)
		if err != nil {
			return nil, fmt.Errorf("claude: editor env: %w", err)
		}
		readable = append(readable, p)
	}
	var envVars []map[string]string
	for _, v := range loginVars {
		envVars = append(envVars, map[string]string{"name": v, "mode": "deny"})
	}
	domains := e.Domains
	if domains == nil {
		domains = []string{}
	}
	return map[string]any{
		"sandbox": map[string]any{
			"enabled":                  true,
			"allowUnsandboxedCommands": false,
			"failIfUnavailable":        true,
			"filesystem": map[string]any{
				"denyRead":   []string{"~/"},
				"allowRead":  readable,
				"allowWrite": writable,
			},
			"network":     map[string]any{"allowedDomains": domains},
			"credentials": map[string]any{"envVars": envVars},
		},
		"permissions": map[string]any{"deny": []string{"WebFetch", "WebSearch"}},
	}, nil
}

func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

type session struct {
	ref     string
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stderr  *tail
	cleanup func() // removes the session's config files
	allowed []string
	events  chan agent.Event
	exited  chan struct{}

	mu        sync.Mutex
	busy      bool
	cancelled bool
	closed    bool
	lastCost  float64
}

func (s *session) Ref() string                { return s.ref }
func (s *session) Events() <-chan agent.Event { return s.events }

func (s *session) Send(ctx context.Context, text string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	switch {
	case s.closed || s.cancelled:
		s.mu.Unlock()
		return errors.New("claude: the session has ended")
	case s.busy:
		s.mu.Unlock()
		return errors.New("claude: a turn is already running")
	}
	msg, err := json.Marshal(map[string]any{
		"type":    "user",
		"message": map[string]any{"role": "user", "content": text},
	})
	if err != nil {
		s.mu.Unlock()
		return err
	}
	s.busy = true
	s.mu.Unlock()
	// Pipe writes may block while the tool starts. Cancel must be able to
	// take mu and stop the process, which releases that write.
	if _, err := s.stdin.Write(append(msg, '\n')); err != nil {
		s.mu.Lock()
		s.busy = false
		s.mu.Unlock()
		return fmt.Errorf("claude: send: %w", err)
	}
	return nil
}

// Cancel stops the subprocess. stream-json gives no way to abort one turn and
// keep the process, so the session ends, and Spec.Resume continues it.
func (s *session) Cancel() error {
	s.mu.Lock()
	s.cancelled = true
	s.mu.Unlock()
	return s.stop()
}

func (s *session) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()
	s.stdin.Close()
	select {
	case <-s.exited:
	case <-time.After(5 * time.Second):
		_ = s.stop()
	}
	return nil
}

// stop interrupts the process, and kills it if it has not gone in two
// seconds.
func (s *session) stop() error {
	if s.cmd.Process == nil {
		return nil
	}
	_ = s.cmd.Process.Signal(os.Interrupt)
	select {
	case <-s.exited:
	case <-time.After(2 * time.Second):
		_ = s.cmd.Process.Kill()
		<-s.exited
	}
	return nil
}

func (s *session) emit(ev agent.Event) {
	ev.At = time.Now()
	s.events <- ev
}

// read turns the stream into events until the process ends.
func (s *session) read(stdout io.Reader) {
	defer close(s.events)
	defer s.cleanup()

	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	locked := false
	for sc.Scan() && !locked {
		var ev streamEvent
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			continue // not a protocol line
		}
		locked = s.handle(ev)
	}
	if locked {
		_ = s.cmd.Process.Kill()
	}
	_ = s.cmd.Wait()
	close(s.exited)

	s.mu.Lock()
	busy, cancelled, closed := s.busy, s.cancelled, s.closed
	s.busy = false
	s.mu.Unlock()
	switch {
	case locked:
		s.emit(agent.Event{Kind: agent.TurnDone, Err: ErrLockdown})
	case busy && cancelled:
		s.emit(agent.Event{Kind: agent.TurnDone, Cancelled: true})
	case busy || !closed && !cancelled:
		s.emit(agent.Event{Kind: agent.TurnDone, Err: fmt.Errorf("claude exited: %s", s.stderr.String())})
	}
}

// streamEvent is the part of a stream-json line the driver reads.
type streamEvent struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`

	// system/init
	Version string   `json:"claude_code_version"`
	Model   string   `json:"model"`
	Tools   []string `json:"tools"`
	KeySrc  string   `json:"apiKeySource"`

	// stream_event
	Event *struct {
		Delta *struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"delta"`
	} `json:"event"`

	// assistant, user
	Message *struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`

	// result
	IsError bool    `json:"is_error"`
	Result  string  `json:"result"`
	CostUSD float64 `json:"total_cost_usd"`
	Usage   *struct {
		Input      int64 `json:"input_tokens"`
		Output     int64 `json:"output_tokens"`
		CacheRead  int64 `json:"cache_read_input_tokens"`
		CacheWrite int64 `json:"cache_creation_input_tokens"`
	} `json:"usage"`
	Denials []struct {
		Tool  string          `json:"tool_name"`
		ID    string          `json:"tool_use_id"`
		Input json.RawMessage `json:"tool_input"`
	} `json:"permission_denials"`
}

type block struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	IsError   bool            `json:"is_error"`
	Content   json.RawMessage `json:"content"`
}

// handle emits the events for one line, and reports whether the session broke
// the lockdown.
func (s *session) handle(ev streamEvent) (locked bool) {
	switch ev.Type {
	case "system":
		if ev.Subtype != "init" {
			return false
		}
		for _, t := range ev.Tools {
			if !slices.Contains(s.allowed, t) {
				return true
			}
		}
		s.emit(agent.Event{Kind: agent.Info, Info: &agent.SessionInfo{
			Tool: "claude", Version: ev.Version, Model: ev.Model,
			Account: keySource(ev.KeySrc), Tools: ev.Tools,
		}})
	case "stream_event":
		if ev.Event != nil && ev.Event.Delta != nil && ev.Event.Delta.Type == "text_delta" {
			s.emit(agent.Event{Kind: agent.Text, Text: ev.Event.Delta.Text})
		}
	case "assistant":
		for _, b := range blocks(ev) {
			if b.Type == "tool_use" {
				s.emit(agent.Event{Kind: agent.ToolCall, Tool: &agent.Tool{ID: b.ID, Name: b.Name, Input: string(b.Input)}})
			}
		}
	case "user":
		for _, b := range blocks(ev) {
			if b.Type == "tool_result" {
				s.emit(agent.Event{Kind: agent.ToolResult, Tool: &agent.Tool{ID: b.ToolUseID, Output: contentText(b.Content), IsError: b.IsError}})
			}
		}
	case "result":
		for _, d := range ev.Denials {
			s.emit(agent.Event{Kind: agent.Denied, Tool: &agent.Tool{ID: d.ID, Name: d.Tool, Input: string(d.Input)}})
		}
		s.mu.Lock()
		cost := ev.CostUSD - s.lastCost // total_cost_usd runs for the whole process
		s.lastCost = ev.CostUSD
		s.busy = false
		s.mu.Unlock()
		u := &agent.TurnUsage{CostUSD: cost}
		if ev.Usage != nil {
			u.Input, u.Output, u.CacheRead, u.CacheWrite = ev.Usage.Input, ev.Usage.Output, ev.Usage.CacheRead, ev.Usage.CacheWrite
		}
		s.emit(agent.Event{Kind: agent.Usage, Usage: u})
		var err error
		if ev.IsError || ev.Subtype != "success" {
			err = fmt.Errorf("claude: turn ended %s: %s", ev.Subtype, ev.Result)
			if strings.Contains(strings.ToLower(ev.Result), "not logged in") {
				err = agent.ErrUnauthenticated
			}
		}
		s.emit(agent.Event{Kind: agent.TurnDone, Err: err})
	}
	return false
}

func blocks(ev streamEvent) []block {
	if ev.Message == nil {
		return nil
	}
	var bs []block
	_ = json.Unmarshal(ev.Message.Content, &bs) // a plain string has no blocks
	return bs
}

// contentText flattens a tool result's content, a string or a list of text
// blocks.
func contentText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var parts []struct {
		Text string `json:"text"`
	}
	_ = json.Unmarshal(raw, &parts)
	var b strings.Builder
	for _, p := range parts {
		b.WriteString(p.Text)
	}
	return b.String()
}

// keySource names the kind of credential from init's apiKeySource. "none"
// means the tool's own login, which Probe describes further.
func keySource(src string) string {
	if src == "" || src == "none" {
		return "login"
	}
	return "api key (" + src + ")"
}

// tail keeps the last 4 KiB written to it, for an error message.
type tail struct {
	mu  sync.Mutex
	buf []byte
}

func (t *tail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if len(t.buf) > 4096 {
		t.buf = t.buf[len(t.buf)-4096:]
	}
	return len(p), nil
}

func (t *tail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	s := strings.TrimSpace(string(t.buf))
	if s == "" {
		return "no output on stderr"
	}
	return s
}
