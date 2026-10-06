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
	"strings"
	"sync"
	"sync/atomic"
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
	// EditorDomains are the hosts an editor's commands may reach; nil for
	// DefaultEditorDomains.
	EditorDomains []string

	mu   sync.Mutex
	runs map[string]*Run
	// down is set by Shutdown, after which a checkout that finishes starts
	// no session.
	down atomic.Bool
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
	Waiting   Status = "waiting" // a later stage, until the one before it has reported
	Cancelled Status = "cancelled"
)

// Run is one review of one change at one head.
type Run struct {
	mu     sync.Mutex
	saveMu sync.Mutex // orders the writes of the run record

	ID      string
	Kind    string
	Mode    string
	Threads []forge.Thread
	Ref     forge.Ref
	Title   string
	WebURL  string
	Head    string // the SHA the run reviewed; it never moves (D15)
	Base    string
	Created time.Time
	Status  Status
	Error   string
	Summary *SummaryRange

	dir           string // the worktree
	repoKey       string
	anchors       lineIndex
	store         *drafts.Store
	sessions      []*Session
	path          string
	first         string // the first message, for a session that restarts
	summaryCancel context.CancelFunc
	summaryDone   chan struct{}
	// stopPrepare ends the checkout a new run starts with, and prepared is
	// closed once it has returned, so nothing of the run's git outlives a
	// Close or a Shutdown. Both are nil for a run read from its record.
	stopPrepare    context.CancelFunc
	prepared       chan struct{}
	summaryStopped atomic.Bool
	editor         *EditorState
	who            Identity // the editor's git identity
	origin         string   // a local range's checkout, where its fixes go (D42)
}

// Session is one agent session in a run.
type Session struct {
	ID            string
	Profile       string
	Tool          string
	Model         string
	Role          drafts.Role
	Status        Status
	Error         string
	Info          *agent.SessionInfo
	Ref           string // what resumes it
	Usage         agent.TurnUsage
	Log           []Entry
	FirstComplete bool // successful initial turn, not inferred from process status

	driver agent.Driver
	live   agent.Session
	token  string
	spec   agent.Spec
	// reported is set when the session's first turn ends, however it ends.
	reported bool
	// nudged is set once a verifier has been sent back for findings it
	// left unmarked, so it is sent back once at most.
	nudged bool
	// nudging is set from the decision to send a verifier back until the
	// message is sent, so the run does not read as settled in between.
	nudging bool
	// Target is the draft a suggestion session writes a suggestion for.
	Target string
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
// every reviewer has reported. With verifier set, a verifier marks each
// finding the person works from once the reviewers and any synthesizer have
// reported (D31). The Profile of either is ignored.
func (m *Manager) Start(ch *forge.Change, src Source, reviewers []Reviewer, synth, verifier *Reviewer) (*Run, error) {
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
	if verifier != nil {
		name, d, err := m.driver(verifier.Tool)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, &Session{
			ID: fmt.Sprintf("s%d", len(sessions)+1), Profile: "verification", Tool: name, Model: verifier.Model,
			Role: drafts.Verifier, Status: Waiting, driver: d,
			spec: agent.Spec{SystemPrompt: verificationPrompt(), Model: verifier.Model},
		})
	}

	return m.start(ch, src, sessions, "", firstMessage(ch), nil)
}

func (m *Manager) start(ch *forge.Change, src Source, sessions []*Session, mode, first string, threads []forge.Thread) (*Run, error) {
	id := newRunID()
	r := &Run{
		ID: id, Kind: ch.Kind, Mode: mode, Threads: threads, Ref: ch.Ref, Title: ch.Title, WebURL: ch.WebURL,
		Head: ch.Diff.Head, Base: ch.Diff.Base, Created: time.Now().UTC(), Status: Preparing,
		dir:      filepath.Join(m.StateDir, "worktrees", id),
		repoKey:  src.Key,
		store:    drafts.NewStore(),
		sessions: sessions,
		path:     filepath.Join(m.StateDir, "runs", id, "run.json"),
		first:    first,
	}
	if ch.Kind == Local {
		r.origin = src.Remote
	}
	r.anchors = indexLines(ch.Files)
	r.store.CheckAnchor = r.anchors.check
	r.store.CheckSuggestion = r.checkSuggestion
	if mode == "triage" {
		r.bindThreads()
	}
	m.mu.Lock()
	if m.runs == nil {
		m.runs = map[string]*Run{}
	}
	m.runs[id] = r
	m.mu.Unlock()
	r.save()

	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	r.stopPrepare, r.prepared = cancel, make(chan struct{})
	go m.prepare(ctx, r, src)
	return r, nil
}

func (m *Manager) prepare(ctx context.Context, r *Run, src Source) {
	defer close(r.prepared)
	defer r.stopPrepare()
	if err := m.Repos.Checkout(ctx, src, r.dir); err != nil {
		r.mu.Lock()
		if r.Status == Closed { // stopped by Close, which records the run
			r.mu.Unlock()
			return
		}
		r.Status, r.Error = Failed, err.Error()
		for _, s := range r.sessions {
			s.Status = Failed
		}
		r.mu.Unlock()
		r.save()
		return
	}
	r.mu.Lock()
	// A run closed while its checkout ran, or one vuoro is exiting under,
	// starts nothing.
	if r.Status == Closed || m.down.Load() {
		r.mu.Unlock()
		return
	}
	r.Status = Working
	r.mu.Unlock()
	for _, s := range r.sessions {
		if s.Role == drafts.Reviewer || s.Role == drafts.Triage {
			go m.begin(r, s, r.first)
		}
	}
}

// begin starts a session and sends it its first message.
func (m *Manager) begin(r *Run, s *Session, first string) {
	if err := m.launch(r, s, ""); err != nil {
		r.mu.Lock()
		s.reported = true
		next := r.ready()
		r.mu.Unlock()
		m.advance(r, next)
		return
	}
	m.send(r, s, first, false)
}

// ready returns the waiting sessions whose stage has come, and marks them
// started. A synthesizer waits for every reviewer to report, and a verifier
// for every reviewer and synthesizer. r.mu is held.
func (r *Run) ready() []*Session {
	reviewing, synthesizing := false, false
	for _, s := range r.sessions {
		if !s.reported {
			reviewing = reviewing || s.Role == drafts.Reviewer
			synthesizing = synthesizing || s.Role == drafts.Synthesizer
		}
	}
	var out []*Session
	for _, s := range r.sessions {
		if s.Status != Waiting {
			continue
		}
		if s.Role == drafts.Synthesizer && !reviewing || s.Role == drafts.Verifier && !reviewing && !synthesizing {
			s.Status = Preparing
			out = append(out, s)
		}
	}
	return out
}

// advance starts the sessions whose stage has come.
func (m *Manager) advance(r *Run, next []*Session) {
	for _, s := range next {
		r.mu.Lock()
		text := synthesisMessage(r)
		if s.Role == drafts.Verifier {
			text = verificationMessage(r)
		}
		r.mu.Unlock()
		go m.begin(r, s, text)
	}
}

// shown are the findings the person works from: the synthesizer's drafts when
// it drafted any, and otherwise every pending draft. The UI picks the same.
// r.mu is held.
func (r *Run) shown() []drafts.Draft {
	synth := map[string]bool{}
	for _, s := range r.sessions {
		if s.Role == drafts.Synthesizer {
			synth[s.ID] = true
		}
	}
	return shownDrafts(r.store.All(), synth)
}

// Shown are the findings the person works from in v, as the window picks
// them.
func Shown(v RunView) []drafts.Draft {
	synth := map[string]bool{}
	for _, s := range v.Sessions {
		if s.Role == roleNames[drafts.Synthesizer] {
			synth[s.ID] = true
		}
	}
	return shownDrafts(v.Drafts, synth)
}

// shownDrafts are the pending drafts of the synthesizer sessions in synth
// when there are any, and otherwise every pending draft.
func shownDrafts(all []drafts.Draft, synth map[string]bool) []drafts.Draft {
	var pending, merged []drafts.Draft
	for _, d := range all {
		if d.Status != drafts.Pending {
			continue
		}
		pending = append(pending, d)
		if synth[d.Session] {
			merged = append(merged, d)
		}
	}
	if len(merged) > 0 {
		return merged
	}
	return pending
}

// checkSuggestion checks a suggestion against the file it replaces lines of,
// in the worktree, which is the head. A file it cannot read is not checked;
// the anchor check has already placed the lines in the diff.
func (r *Run) checkSuggestion(a drafts.Anchor, suggestion string) error {
	root, err := os.OpenRoot(r.dir)
	if err != nil {
		return nil
	}
	defer root.Close()
	b, err := root.ReadFile(a.Path)
	if err != nil {
		return nil
	}
	return drafts.Boundary(strings.Split(string(b), "\n"), a, suggestion)
}

// unmarked are the shown findings no verifier has marked. r.mu is held.
func (r *Run) unmarked() []string {
	var ids []string
	for _, d := range r.shown() {
		if d.Confidence == nil {
			ids = append(ids, d.ID)
		}
	}
	return ids
}

// launch starts s's tool, fresh or resumed from resume.
func (m *Manager) launch(r *Run, s *Session, resume string) error {
	if r.Mode == "summary" {
		return errors.New("review: summaries cannot resume or receive drafts tools")
	}
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
	if s.Role == drafts.Editor {
		if !r.editor.Active() || r.editor.Session != s.ID {
			r.mu.Unlock()
			return errors.New("review: this editor's work was discarded")
		}
		spec := m.editorSpec(r, s.spec)
		spec.Resume = resume
		r.mu.Unlock()
		return m.started(r, s, spec)
	}
	if s.token == "" {
		s.token = m.Drafts.Register(r.store, drafts.Session{ID: s.ID, Role: s.Role, Drafted: s.drafted(), Target: s.Target})
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
		Tools: drafts.ToolNames(s.Role),
	}
	r.mu.Unlock()
	return m.started(r, s, spec)
}

// started starts s's tool with spec.
func (m *Manager) started(r *Run, s *Session, spec agent.Spec) error {
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
	case r.Status == Closed && s.Role != drafts.Editor: // an editor outlives its review's close
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
	var next []*Session
	for ev := range live.Events() {
		save := false
		nudge := ""
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
			if !s.reported {
				s.FirstComplete = ev.Err == nil && !ev.Cancelled
			}
			s.reported = true
			next = r.ready()
			// A verifier that stopped short is sent back once for the rest.
			if missing := r.unmarked(); s.Role == drafts.Verifier && !s.nudged && !ev.Cancelled && len(missing) > 0 {
				s.nudged, s.nudging = true, true
				nudge = "These findings have no mark yet. Check each against the code and mark it with rate_draft: " + strings.Join(missing, ", ")
			}
			r.settle()
			save = true
		}
		r.mu.Unlock()
		if save {
			r.save()
		}
		m.advance(r, next)
		next = nil
		if nudge != "" {
			go func(text string) {
				_ = m.send(r, s, text, false)
				r.mu.Lock()
				s.nudging = false
				r.mu.Unlock()
			}(nudge)
		}
	}
	r.mu.Lock()
	if s.live == live {
		s.live = nil
		if s.Status != Closed && s.Status != Failed {
			s.Status = Ended
		}
	}
	s.reported = true
	next = r.ready()
	r.settle()
	r.mu.Unlock()
	r.save()
	m.advance(r, next)
}

// Settled says the run has nothing left to do without the person: it
// failed or closed, or every session has reported its first turn and none
// is working or waiting for its stage.
func (r *Run) Settled() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch r.Status {
	case Failed, Closed, Cancelled:
		return true
	case Preparing:
		return false
	}
	for _, s := range r.sessions {
		if !s.reported || s.nudging || s.Status == Working || s.Status == Preparing || s.Status == Waiting {
			return false
		}
	}
	return true
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
	if r.Mode == "summary" {
		return errors.New("review: start a new summary to select a new range")
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
	if r.Mode == "summary" {
		return m.stopSummary(r)
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

// Draft returns one draft of a run, and the head the run reviewed.
func (m *Manager) Draft(runID, id string) (drafts.Draft, string, error) {
	r := m.Get(runID)
	if r == nil {
		return drafts.Draft{}, "", ErrNoRun
	}
	d, ok := r.store.Get(id)
	if !ok {
		return drafts.Draft{}, "", ErrNoRun
	}
	return d, r.Head, nil
}

// Decide records the person's decision on one of a run's drafts.
func (m *Manager) Decide(runID, id, decision, noteID string, edited bool) error {
	r := m.Get(runID)
	if r == nil {
		return ErrNoRun
	}
	if _, err := r.store.Decide(id, decision, noteID, edited); err != nil {
		return err
	}
	r.save()
	return nil
}

// Keep records the person's Keep of one of a run's drafts around write,
// which puts it on the forge. A second Keep of the same draft while the
// first is writing is refused with drafts.ErrKeeping.
func (m *Manager) Keep(runID, id string, write func(drafts.Draft) (string, bool, error)) error {
	r := m.Get(runID)
	if r == nil {
		return ErrNoRun
	}
	if _, err := r.store.Keep(id, write); err != nil {
		return err
	}
	r.save()
	return nil
}

// AskSuggestion starts a fresh session to write the suggested change for one
// finding: the lines that replace its range at the head (D37). It does not
// resume the session that drafted the finding, whose context holds the whole
// change. The new session reads only what it needs, and its token may set
// that one draft's suggestion and nothing else. The draft must still be the
// person's to decide, on head-side lines.
func (m *Manager) AskSuggestion(runID, id string) error {
	r := m.Get(runID)
	if r == nil {
		return ErrNoRun
	}
	d, ok := r.store.Get(id)
	switch {
	case !ok:
		return ErrNoRun
	case d.Status != drafts.Pending || d.Decision != "":
		return fmt.Errorf("review: %s is no longer pending", id)
	case d.Anchor == nil || d.Anchor.Side != drafts.Head:
		return fmt.Errorf("review: %s is not on lines of the change's head, so there is nothing for a suggestion to replace", id)
	}
	r.mu.Lock()
	if r.Status == Closed || r.Status == Preparing || r.Status == Failed {
		r.mu.Unlock()
		return fmt.Errorf("review: the run is %s", r.Status)
	}
	var drafter *Session
	for _, s := range r.sessions {
		if s.Role == drafts.Suggester && s.Target == id && (s.Status == Working || s.Status == Preparing) {
			r.mu.Unlock()
			return fmt.Errorf("review: a suggestion for %s is already being written", id)
		}
		if s.ID == d.Session {
			drafter = s
		}
	}
	if drafter == nil {
		r.mu.Unlock()
		return ErrNoRun
	}
	s := &Session{
		ID: fmt.Sprintf("s%d", len(r.sessions)+1), Profile: "suggestion", Tool: drafter.Tool, Model: drafter.Model,
		Role: drafts.Suggester, Target: id, Status: Preparing,
		spec: agent.Spec{SystemPrompt: suggestionPrompt(), Model: drafter.Model},
	}
	r.sessions = append(r.sessions, s)
	r.settle()
	r.mu.Unlock()
	r.save()
	go m.begin(r, s, suggestionMessage(d))
	return nil
}

// Override records the person's own confidence mark on one of a run's
// drafts, or with an empty level takes it back.
func (m *Manager) Override(runID, id string, level drafts.Level, reason string) error {
	r := m.Get(runID)
	if r == nil {
		return ErrNoRun
	}
	if _, err := r.store.Override(id, level, reason); err != nil {
		return err
	}
	r.save()
	return nil
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
	if err := r.endPrepare(ctx); err != nil {
		return err
	}
	if r.Mode == "summary" {
		if err := m.stopSummary(r); err != nil {
			return err
		}
		if r.summaryDone != nil {
			select {
			case <-r.summaryDone:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
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
	r.mu.Lock()
	ed := r.editor.Active()
	if ed {
		r.editor.Discarded = true
	}
	r.mu.Unlock()
	if ed { // nothing of the editor's outlives its review
		_ = os.RemoveAll(m.editorDir(r.ID))
		_ = m.Repos.DropEditor(ctx, r.repoKey, r.ID)
	}
	err := m.Repos.Remove(ctx, r.repoKey, r.dir)
	if _, statErr := os.Stat(r.dir); errors.Is(statErr, os.ErrNotExist) {
		err = nil
	}
	r.save()
	return err
}

// endPrepare stops the run's checkout if it is still running, marks the run
// closed so the checkout starts no session, and waits for it to return.
func (r *Run) endPrepare(ctx context.Context) error {
	if r.prepared == nil {
		return nil
	}
	r.mu.Lock()
	r.Status = Closed
	r.mu.Unlock()
	r.stopPrepare()
	select {
	case <-r.prepared:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Shutdown ends every live session, as vuoro exits, so no agent outlives it.
// Runs stay open, and their sessions are resumable from their records.
func (m *Manager) Shutdown() {
	m.down.Store(true)
	for _, r := range m.Runs() {
		if r.prepared != nil {
			r.stopPrepare()
			<-r.prepared
		}
		if r.Mode == "summary" {
			_ = m.stopSummary(r)
			if r.summaryDone != nil {
				<-r.summaryDone
			}
			continue
		}
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
