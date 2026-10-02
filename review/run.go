// Package review runs reviews (docs/plan.md section 6.2): it checks a change
// out in a worktree, starts a locked reviewer session per profile, and keeps
// the drafts they propose through the drafts MCP server.
//
// An agent's only write path is a pending draft. Nothing here reaches a
// forge.
package review

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/terva-sh/vuoro/agent"
	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/review/drafts"
)

// Manager starts, keeps, and closes review runs.
type Manager struct {
	StateDir string
	Repos    *Repos
	// Drivers are the agent tools a reviewer may run on, by name, such as
	// "claude". Only a driver whose lockdown is proven is offered.
	Drivers map[string]agent.Driver
	Drafts  *drafts.Server
	// MCPURL is the drafts server's URL on the loopback server.
	MCPURL   string
	Profiles []Profile

	mu   sync.Mutex
	runs map[string]*Run
}

// Reviewer is one reviewer a run starts: a profile on a tool and a model.
type Reviewer struct {
	Profile string `json:"profile"`
	Tool    string `json:"tool,omitempty"`  // a Drivers key; empty for the only one
	Model   string `json:"model,omitempty"` // empty for the tool's default
}

// Status is where a run or a session stands.
type Status string

const (
	Preparing Status = "preparing" // checking out the worktree
	Working   Status = "working"   // a turn is running
	Idle      Status = "idle"      // waiting for the person
	Ended     Status = "ended"     // the tool's process ended; it can be resumed
	Failed    Status = "failed"
	Closed    Status = "closed"
	Waiting   Status = "waiting" // the synthesizer, until every reviewer has reported
)

// Run is one review of one change at one head.
type Run struct {
	mu     sync.Mutex
	saveMu sync.Mutex // orders the writes of the run record

	ID      string
	Ref     forge.Ref
	Title   string
	WebURL  string
	Head    string // the SHA the run reviewed; it never moves (D15)
	Base    string
	Created time.Time
	Status  Status
	Error   string

	dir      string // the worktree
	repoKey  string
	anchors  lineIndex
	store    *drafts.Store
	sessions []*Session
	path     string
	first    string // the first message, for a session that restarts
}

// Session is one agent session in a run.
type Session struct {
	ID      string
	Profile string
	Tool    string
	Model   string
	Role    drafts.Role
	Status  Status
	Error   string
	Info    *agent.SessionInfo
	Ref     string // what resumes it
	Usage   agent.TurnUsage
	Log     []Entry

	driver agent.Driver
	live   agent.Session
	token  string
	spec   agent.Spec
	// reported is set when the session's first turn ends, however it ends.
	reported bool
}

// Entry is one line of a session's transcript, as the person reads it.
type Entry struct {
	Kind    string    `json:"kind"` // change, user, assistant, tool, denied, error
	CallID  string    `json:"call_id,omitempty"`
	Text    string    `json:"text,omitempty"`
	Tool    string    `json:"tool,omitempty"`
	Input   string    `json:"input,omitempty"`
	Output  string    `json:"output,omitempty"`
	IsError bool      `json:"is_error,omitempty"`
	At      time.Time `json:"at"`
}

// maxToolText bounds a tool's input or output in the transcript.
const maxToolText = 4 << 10

// Start begins a review of ch. It returns once the run exists, and checks
// out the worktree and starts the reviewers in the background. With synth
// set, a synthesizer on that tool and model merges the reviewers' drafts once
// every reviewer has reported; its Profile is ignored.
func (m *Manager) Start(ch *forge.Change, src Source, reviewers []Reviewer, synth *Reviewer) (*Run, error) {
	if len(reviewers) == 0 {
		return nil, errors.New("review: a run needs at least one reviewer")
	}
	sessions := make([]*Session, 0, len(reviewers))
	for i, rv := range reviewers {
		p, ok := m.profile(rv.Profile)
		if !ok {
			return nil, fmt.Errorf("review: no profile named %q", rv.Profile)
		}
		name, d, err := m.driver(rv.Tool)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, &Session{
			ID: fmt.Sprintf("s%d", i+1), Profile: p.Name, Tool: name, Model: rv.Model,
			Role: drafts.Reviewer, Status: Preparing, driver: d,
			spec: agent.Spec{SystemPrompt: systemPrompt(p), Model: rv.Model},
		})
	}
	if synth != nil {
		name, d, err := m.driver(synth.Tool)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, &Session{
			ID: fmt.Sprintf("s%d", len(sessions)+1), Profile: "synthesis", Tool: name, Model: synth.Model,
			Role: drafts.Synthesizer, Status: Waiting, driver: d,
			spec: agent.Spec{SystemPrompt: synthesisPrompt(), Model: synth.Model},
		})
	}

	id := newRunID()
	r := &Run{
		ID: id, Ref: ch.Ref, Title: ch.Title, WebURL: ch.WebURL,
		Head: ch.Diff.Head, Base: ch.Diff.Base, Created: time.Now().UTC(), Status: Preparing,
		dir:      filepath.Join(m.StateDir, "worktrees", id),
		repoKey:  src.Key,
		store:    drafts.NewStore(),
		sessions: sessions,
		path:     filepath.Join(m.StateDir, "runs", id, "run.json"),
		first:    firstMessage(ch),
	}
	r.anchors = indexLines(ch.Files)
	r.store.CheckAnchor = r.anchors.check
	m.mu.Lock()
	if m.runs == nil {
		m.runs = map[string]*Run{}
	}
	m.runs[id] = r
	m.mu.Unlock()
	r.save()

	go m.prepare(r, src)
	return r, nil
}

func (m *Manager) prepare(r *Run, src Source) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	if err := m.Repos.Checkout(ctx, src, r.dir); err != nil {
		r.mu.Lock()
		r.Status, r.Error = Failed, err.Error()
		for _, s := range r.sessions {
			s.Status = Failed
		}
		r.mu.Unlock()
		r.save()
		return
	}
	r.mu.Lock()
	r.Status = Working
	r.mu.Unlock()
	for _, s := range r.sessions {
		if s.Role == drafts.Reviewer {
			go m.begin(r, s, r.first)
		}
	}
}

// begin starts a session and sends it its first message.
func (m *Manager) begin(r *Run, s *Session, first string) {
	if err := m.launch(r, s, ""); err != nil {
		r.mu.Lock()
		s.reported = true
		synth := r.readyToSynthesize()
		r.mu.Unlock()
		m.synthesize(r, synth)
		return
	}
	m.send(r, s, first, false)
}

// readyToSynthesize returns the waiting synthesizer once every reviewer has
// reported, and marks it started. r.mu is held.
func (r *Run) readyToSynthesize() *Session {
	var synth *Session
	for _, s := range r.sessions {
		switch {
		case s.Role == drafts.Synthesizer && s.Status == Waiting:
			synth = s
		case s.Role == drafts.Reviewer && !s.reported:
			return nil
		}
	}
	if synth != nil {
		synth.Status = Preparing
	}
	return synth
}

func (m *Manager) synthesize(r *Run, synth *Session) {
	if synth == nil {
		return
	}
	r.mu.Lock()
	text := synthesisMessage(r)
	r.mu.Unlock()
	go m.begin(r, synth, text)
}

// launch starts s's tool, fresh or resumed from resume.
func (m *Manager) launch(r *Run, s *Session, resume string) error {
	r.mu.Lock()
	if s.driver == nil { // a run reloaded from its record
		_, d, err := m.driver(s.Tool)
		if err != nil {
			s.Status, s.Error = Failed, err.Error()
			r.mu.Unlock()
			return err
		}
		s.driver = d
	}
	if s.token == "" {
		s.token = m.Drafts.Register(r.store, drafts.Session{ID: s.ID, Role: s.Role, Drafted: s.drafted()})
	}
	spec := s.spec
	spec.Dir = r.dir
	spec.Resume = resume
	spec.Drafts = &agent.MCPServer{
		Name: "vuoro",
		Config: map[string]any{
			"type":    "http",
			"url":     m.MCPURL,
			"headers": map[string]string{"Authorization": "Bearer " + s.token},
		},
		Tools: drafts.ToolNames(),
	}
	r.mu.Unlock()

	live, err := s.driver.Start(context.Background(), spec)
	r.mu.Lock()
	if err != nil {
		s.Status, s.Error = Failed, err.Error()
		r.mu.Unlock()
		r.save()
		return err
	}
	s.live, s.Ref, s.Status, s.Error = live, live.Ref(), Idle, ""
	r.mu.Unlock()
	go m.pump(r, s, live)
	return nil
}

func (s *Session) drafted() string {
	model := s.Model
	if model == "" {
		model = "default model"
	}
	tool := s.Tool
	if s.driver != nil {
		tool = s.driver.Name()
	}
	return tool + " · " + model
}

// send starts a turn on s. A session whose process ended is resumed first.
func (m *Manager) send(r *Run, s *Session, text string, fromPerson bool) error {
	r.mu.Lock()
	live, ref := s.live, s.Ref
	switch {
	case r.Status == Closed:
		r.mu.Unlock()
		return errors.New("review: the run is closed")
	case s.Status == Working:
		r.mu.Unlock()
		return errors.New("review: the session is still working; wait, or stop it")
	}
	r.mu.Unlock()
	if live == nil {
		if ref == "" {
			return errors.New("review: the session never started")
		}
		if err := m.launch(r, s, ref); err != nil {
			return err
		}
		r.mu.Lock()
		live = s.live
		r.mu.Unlock()
	}
	r.mu.Lock()
	kind := "user"
	if !fromPerson {
		kind = "change" // the change, as vuoro sent it
	}
	s.Log = append(s.Log, Entry{Kind: kind, Text: text, At: time.Now().UTC()})
	s.Status = Working
	r.settle()
	r.mu.Unlock()
	if err := live.Send(context.Background(), text); err != nil {
		r.mu.Lock()
		s.Status, s.Error = Failed, err.Error()
		r.mu.Unlock()
		r.save()
		return err
	}
	return nil
}

// pump records a session's events until its process ends.
func (m *Manager) pump(r *Run, s *Session, live agent.Session) {
	var synth *Session
	for ev := range live.Events() {
		save := false
		r.mu.Lock()
		at := ev.At
		if at.IsZero() {
			at = time.Now().UTC()
		}
		switch ev.Kind {
		case agent.Info:
			s.Info = ev.Info
			save = true
		case agent.Text:
			if n := len(s.Log); n > 0 && s.Log[n-1].Kind == "assistant" {
				s.Log[n-1].Text += ev.Text
			} else {
				s.Log = append(s.Log, Entry{Kind: "assistant", Text: ev.Text, At: at})
			}
		case agent.ToolCall:
			s.Log = append(s.Log, Entry{Kind: "tool", CallID: ev.Tool.ID, Tool: ev.Tool.Name, Input: clip(ev.Tool.Input), At: at})
		case agent.ToolResult:
			for i := len(s.Log) - 1; i >= 0; i-- {
				if e := &s.Log[i]; e.Kind == "tool" && e.CallID == ev.Tool.ID {
					e.Output, e.IsError = clip(ev.Tool.Output), ev.Tool.IsError
					break
				}
			}
		case agent.Denied:
			s.Log = append(s.Log, Entry{Kind: "denied", Tool: ev.Tool.Name, Input: clip(ev.Tool.Input), At: at})
		case agent.Usage:
			u := ev.Usage
			s.Usage.Input += u.Input
			s.Usage.Output += u.Output
			s.Usage.CacheRead += u.CacheRead
			s.Usage.CacheWrite += u.CacheWrite
			s.Usage.CostUSD += u.CostUSD
		case agent.TurnDone:
			s.Status = Idle
			if ev.Err != nil && !ev.Cancelled {
				s.Log = append(s.Log, Entry{Kind: "error", Text: ev.Err.Error(), At: at})
			}
			s.reported = true
			synth = r.readyToSynthesize()
			r.settle()
			save = true
		}
		r.mu.Unlock()
		if save {
			r.save()
		}
		m.synthesize(r, synth)
		synth = nil
	}
	r.mu.Lock()
	if s.live == live {
		s.live = nil
		if s.Status != Closed && s.Status != Failed {
			s.Status = Ended
		}
	}
	s.reported = true
	synth = r.readyToSynthesize()
	r.settle()
	r.mu.Unlock()
	r.save()
	m.synthesize(r, synth)
}

// settle sets the run's status from its sessions'. r.mu is held.
func (r *Run) settle() {
	if r.Status == Closed || r.Status == Failed || r.Status == Preparing {
		return
	}
	r.Status = Idle
	for _, s := range r.sessions {
		if s.Status == Working || s.Status == Preparing {
			r.Status = Working
		}
	}
}

func clip(s string) string {
	if len(s) <= maxToolText {
		return s
	}
	return s[:maxToolText] + "\n… (cut)"
}

// Say sends the person's message to one session of a run.
func (m *Manager) Say(runID, sessionID, text string) error {
	r, s, err := m.find(runID, sessionID)
	if err != nil {
		return err
	}
	return m.send(r, s, text, true)
}

// Stop cancels the turn in flight on one session. The session can be talked
// to again, which resumes it.
func (m *Manager) Stop(runID, sessionID string) error {
	r, s, err := m.find(runID, sessionID)
	if err != nil {
		return err
	}
	r.mu.Lock()
	live := s.live
	r.mu.Unlock()
	if live == nil {
		return nil
	}
	return live.Cancel()
}

func (m *Manager) find(runID, sessionID string) (*Run, *Session, error) {
	r := m.Get(runID)
	if r == nil {
		return nil, nil, ErrNoRun
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.sessions {
		if s.ID == sessionID {
			return r, s, nil
		}
	}
	return nil, nil, ErrNoRun
}

// ErrNoRun means no run or session has that ID.
var ErrNoRun = errors.New("review: no such run or session")

// Get returns a run, or nil.
func (m *Manager) Get(id string) *Run {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.runs[id]
}

// Runs returns every run, newest first.
func (m *Manager) Runs() []*Run {
	m.mu.Lock()
	out := make([]*Run, 0, len(m.runs))
	for _, r := range m.runs {
		out = append(out, r)
	}
	m.mu.Unlock()
	slices.SortFunc(out, func(a, b *Run) int { return b.Created.Compare(a.Created) })
	return out
}

// Close ends a run: its sessions stop, their drafts tokens are turned away,
// and the worktree is removed. The drafts stay, for the person to post.
func (m *Manager) Close(ctx context.Context, id string) error {
	r := m.Get(id)
	if r == nil {
		return ErrNoRun
	}
	r.mu.Lock()
	r.Status = Closed
	var live []agent.Session
	for _, s := range r.sessions {
		if s.live != nil {
			live = append(live, s.live)
		}
		s.live = nil
		s.Status = Closed
		if s.token != "" {
			m.Drafts.Unregister(s.token)
			s.token = ""
		}
	}
	r.mu.Unlock()
	for _, l := range live {
		_ = l.Close()
	}
	err := m.Repos.Remove(ctx, r.repoKey, r.dir)
	if _, statErr := os.Stat(r.dir); errors.Is(statErr, os.ErrNotExist) {
		err = nil
	}
	r.save()
	return err
}

// Shutdown ends every live session, as vuoro exits, so no agent outlives it.
// Runs stay open, and their sessions are resumable from their records.
func (m *Manager) Shutdown() {
	for _, r := range m.Runs() {
		r.mu.Lock()
		var live []agent.Session
		for _, s := range r.sessions {
			if s.live != nil {
				live = append(live, s.live)
			}
		}
		r.mu.Unlock()
		for _, l := range live {
			_ = l.Close()
		}
		r.save()
	}
}

func (m *Manager) profile(name string) (Profile, bool) {
	for _, p := range slices.Concat(m.Profiles, Builtin) {
		if p.Name == name {
			return p, true
		}
	}
	return Profile{}, false
}

func (m *Manager) driver(name string) (string, agent.Driver, error) {
	if name == "" && len(m.Drivers) == 1 {
		for n, d := range m.Drivers {
			return n, d, nil
		}
	}
	d, ok := m.Drivers[name]
	if !ok {
		return "", nil, fmt.Errorf("review: no agent tool named %q is available", name)
	}
	return name, d, nil
}

func newRunID() string {
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	return time.Now().UTC().Format("20060102-150405") + "-" + hex.EncodeToString(b)
}
