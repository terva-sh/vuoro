// Package agent is the interface vuoro drives agent tools through, and the
// events they report (docs/plan.md sections 6.1 and 6.6).
//
// vuoro contains no agent. Each tool the person already has runs as a
// subprocess over its own protocol, behind a Driver. Only the drivers under
// agent/ know a tool's protocol, and nothing else in vuoro knows which tool
// runs.
package agent

import (
	"context"
	"errors"
	"time"
)

// A Driver starts sessions with one agent tool.
type Driver interface {
	// Name is the tool's name, as the person sees it.
	Name() string
	// Probe finds the tool and says whether vuoro can drive it. It fails with
	// an error wrapping ErrMissing or ErrTooOld, saying which version is
	// needed, when vuoro cannot.
	Probe(ctx context.Context) (Probe, error)
	// Start starts a reviewer session. The tool's lockdown is the driver's
	// job, and a driver refuses to start one it cannot prove.
	Start(ctx context.Context, spec Spec) (Session, error)
}

// Probe is what a driver found.
type Probe struct {
	Path    string // the binary
	Version string // as the tool reports it
	// Account is the kind of login or key the tool will use, where the tool
	// says, such as "claude.ai team". It never holds an identity.
	Account string
	// ShellLogin says the tool reported no login here, and the person's
	// login shell holds a credential variable that this process does not.
	// A window started from the Dock or from a login item gets launchd's
	// environment and not the shell's, so the tool works in a terminal and
	// not under vuoro. A driver sets this only on the signed-out path, and
	// it asks whether the variables are set, never what they hold.
	ShellLogin bool
}

// ErrMissing means the tool is not installed, or not on PATH.
var ErrMissing = errors.New("not found")

// ErrTooOld means the tool is older than the driver supports.
var ErrTooOld = errors.New("too old")

// ErrUnauthenticated means the tool's own login is unavailable to its session.
var ErrUnauthenticated = errors.New("review tool could not authenticate")

// Spec is a reviewer session to start.
type Spec struct {
	Dir          string // the worktree the reviewer reads, and nothing else
	SystemPrompt string
	Model        string // empty for the tool's default
	// Drafts is the one MCP server the session may use: vuoro's own, with
	// the drafts tools of the session's role, or for a conversation about a
	// change its read-only context tools (section 6.11). Nil starts a
	// session with no MCP tool at all.
	Drafts *MCPServer
	// Resume is the Ref of an earlier session to continue, or empty.
	Resume string
	// Editor, when set, starts the editor (D17, D40) rather than a reviewer:
	// it may edit files and run commands, inside Dir and its sandbox.
	Editor *Editor
}

// Editor is what an editor session may reach beyond its worktree, Dir. A
// driver confines its file tools to Dir, runs its commands in an operating
// system sandbox with no reads or writes of the home directory, and refuses
// to start one it cannot sandbox.
type Editor struct {
	// Caches are directories outside Dir that its commands may read and
	// write, such as build and module caches.
	Caches []string
	// Domains are the only hosts its commands may reach, such as package
	// registries. A forge is never one of them.
	Domains []string
	// Env is added to the environment, which otherwise holds only what the
	// tool needs to start. It holds no credential.
	Env []string
}

// MCPServer names an MCP server and the tools on it that a session may call.
type MCPServer struct {
	Name string
	// Config is the server's entry in the tool's MCP configuration, such as
	// {"type":"http","url":...,"headers":...}. It can carry a credential, so
	// a driver passes it in a file of mode 0600, never in argv.
	Config map[string]any
	Tools  []string
}

// A Session is one conversation with one tool. One turn runs at a time.
type Session interface {
	// Ref is the handle Spec.Resume takes to continue this conversation.
	Ref() string
	// Send starts a turn. Its events arrive on Events, ending with TurnDone.
	// A turn that ends with no Usage event may have spent what was never
	// reported, so a driver that knows a turn spent nothing says so with a
	// zero Usage.
	Send(ctx context.Context, text string) error
	// Events carries every event of the session, and is closed when the
	// session ends.
	Events() <-chan Event
	// Cancel stops the turn in flight. It ends with TurnDone, Cancelled set,
	// and the session ends; Spec.Resume continues it.
	Cancel() error
	// Close ends the session.
	Close() error
}

// Kind says what an Event is.
type Kind int

const (
	Info       Kind = iota + 1 // the session started: Event.Info
	Text                       // assistant text, streamed: Event.Text
	ToolCall                   // the agent called a tool: Event.Tool
	ToolResult                 // the call finished: Event.Tool
	Denied                     // the tool was refused by the lockdown: Event.Tool
	Usage                      // this turn's tokens and cost: Event.Usage; zero when the turn spent nothing
	TurnDone                   // the turn ended: Event.Err, Event.Cancelled
)

// Event is one thing a session reported, normalised across tools.
type Event struct {
	Kind      Kind
	Info      *SessionInfo
	Text      string
	Tool      *Tool
	Usage     *TurnUsage
	Err       error // on TurnDone, why the turn failed
	Cancelled bool  // on TurnDone
	At        time.Time
}

// SessionInfo is what a run records about where the person's code went.
type SessionInfo struct {
	Tool    string   `json:"tool"`
	Version string   `json:"version"`
	Model   string   `json:"model"`
	Account string   `json:"account"`
	Tools   []string `json:"tools"` // the tools the session was offered
}

// Tool is one tool call, its result, or its refusal.
type Tool struct {
	ID      string
	Name    string
	Input   string // JSON
	Output  string
	IsError bool
}

// TurnUsage is one turn's tokens and cost, as the tool reports them.
type TurnUsage struct {
	Input      int64   `json:"input"`
	Output     int64   `json:"output"`
	CacheRead  int64   `json:"cache_read"`
	CacheWrite int64   `json:"cache_write"`
	CostUSD    float64 `json:"cost_usd"`
}
