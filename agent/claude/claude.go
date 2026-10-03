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
	"regexp"
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

// ErrLockdown means the session offered a tool the lockdown does not allow.
// The driver ends such a session at once rather than let it run.
var ErrLockdown = errors.New("claude: the session offered a tool outside the reviewer lockdown")

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
	return p, nil
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
func args(spec agent.Spec, mcpConfig string, allowed []string, sessionArgs []string) []string {
	a := []string{
		"-p",
		"--input-format", "stream-json",
		"--output-format", "stream-json",
		"--verbose",
		"--include-partial-messages",
		"--tools", strings.Join(readTools, ","),
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

	ref := spec.Resume
	sessionArgs := []string{"--resume", ref}
	if ref == "" {
		ref = newUUID()
		sessionArgs = []string{"--session-id", ref}
	}

	cmd := exec.Command(path, args(spec, cfgPath, allowed, sessionArgs)...)
	cmd.Dir = spec.Dir
	cmd.Env = childEnv(os.Environ())
	var stderr tail
	cmd.Stderr = &stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		os.Remove(cfgPath)
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		os.Remove(cfgPath)
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		os.Remove(cfgPath)
		return nil, fmt.Errorf("claude: start: %w", err)
	}
	s := &session{
		ref:     ref,
		cmd:     cmd,
		stdin:   stdin,
		stderr:  &stderr,
		cfgPath: cfgPath,
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
	cfgPath string
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
	defer os.Remove(s.cfgPath)

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
