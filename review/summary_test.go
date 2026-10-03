package review

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/terva-sh/vuoro/agent"
	"github.com/terva-sh/vuoro/review/drafts"
)

type summaryDriver struct {
	mu               sync.Mutex
	specs            []agent.Spec
	started          chan struct{}
	gate             chan struct{}
	hold             bool
	session          *summarySession
	keepOpenOnCancel bool
}

func (d *summaryDriver) Name() string { return "summary fixture" }
func (d *summaryDriver) Probe(context.Context) (agent.Probe, error) {
	return agent.Probe{Version: "fixture"}, nil
}
func (d *summaryDriver) Start(_ context.Context, spec agent.Spec) (agent.Session, error) {
	d.mu.Lock()
	d.specs = append(d.specs, spec)
	s := &summarySession{events: make(chan agent.Event, 32), hold: d.hold, keepOpenOnCancel: d.keepOpenOnCancel}
	d.session = s
	d.mu.Unlock()
	if d.started != nil {
		close(d.started)
	}
	if d.gate != nil {
		<-d.gate
	}
	return s, nil
}

type summarySession struct {
	mu               sync.Mutex
	events           chan agent.Event
	closed           bool
	keepOpenOnCancel bool
	text             string
	sends            int
	cancels          int
	hold             bool
}

func (s *summarySession) Ref() string                { return "summary-fixture" }
func (s *summarySession) Events() <-chan agent.Event { return s.events }
func (s *summarySession) Send(_ context.Context, text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sends++
	s.text = text
	s.events <- agent.Event{Kind: agent.Text, Text: "Partial summary"}
	if !s.hold {
		s.events <- agent.Event{Kind: agent.Text, Text: " completed"}
		s.events <- agent.Event{Kind: agent.Usage, Usage: &agent.TurnUsage{CostUSD: 0.01}}
		s.events <- agent.Event{Kind: agent.TurnDone}
	}
	return nil
}
func (s *summarySession) Cancel() error {
	s.mu.Lock()
	s.cancels++
	if s.keepOpenOnCancel {
		if !s.closed {
			s.events <- agent.Event{Kind: agent.TurnDone, Cancelled: true}
		}
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()
	return s.Close()
}
func (s *summarySession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		close(s.events)
	}
	return nil
}
func summaryText(v RunView) string {
	var b strings.Builder
	for _, e := range v.Sessions[0].Log {
		if e.Kind == "assistant" {
			b.WriteString(e.Text)
		}
	}
	return b.String()
}
func summaryTerminal(v RunView) bool {
	return v.Status == Ended || v.Status == Failed || v.Status == Cancelled
}
func awaitSummary(t *testing.T, r *Run) {
	t.Helper()
	select {
	case <-r.summaryDone:
	case <-time.After(5 * time.Second):
		t.Fatal("summary lifecycle did not end")
	}
}

func TestSummaryStreamsWithoutDraftCapabilitiesAndPersistsExactRange(t *testing.T) {
	m, _, ch, src := setup(t, nil)
	d := &summaryDriver{hold: true}
	m.Drivers = map[string]agent.Driver{"summary": d}
	r, err := m.StartSummary(ch, src, Reviewer{}, "")
	if err != nil {
		t.Fatal(err)
	}
	v := wait(t, r, func(v RunView) bool { return summaryText(v) == "Partial summary" })
	if v.Status != Working || len(v.Drafts) != 0 {
		t.Fatalf("stream %+v", v)
	}
	d.mu.Lock()
	spec, s := d.specs[0], d.session
	d.mu.Unlock()
	if spec.Drafts != nil || spec.Resume != "" || spec.Dir != r.dir {
		t.Fatal("summary got write tools or wrong checkout")
	}
	s.mu.Lock()
	input := s.text
	s.events <- agent.Event{Kind: agent.Text, Text: " completed"}
	s.events <- agent.Event{Kind: agent.TurnDone}
	s.mu.Unlock()
	wait(t, r, summaryTerminal)
	awaitSummary(t, r)
	if !strings.Contains(input, "-func Add") || !strings.Contains(input, ch.Diff.Base+".."+ch.Diff.Head) {
		t.Fatalf("input lacks exact diff/range: %s", input)
	}
	if historyGit(t, r.dir, "rev-parse", "HEAD") != src.Head {
		t.Fatal("wrong checkout")
	}
	loaded := &Manager{StateDir: m.StateDir}
	if err := loaded.Load(); err != nil {
		t.Fatal(err)
	}
	back := loaded.Get(r.ID).View()
	if back.Mode != "summary" || back.Summary.From != src.Base || back.Summary.To != src.Head || summaryText(back) != "Partial summary completed" || len(back.Drafts) != 0 {
		t.Fatalf("persisted %+v", back)
	}
	if err := loaded.Say(r.ID, "s1", "post this"); err == nil {
		t.Fatal("summary resumed with reviewer capabilities")
	}
	if err := m.Close(context.Background(), r.ID); err != nil {
		t.Fatal(err)
	}
}

func TestSinceSummaryContainsOnlyChangesAfterCompletedReview(t *testing.T) {
	m, _, ch, src := setup(t, func(toolCaller, string) string { return "Reviewed the initial change" })
	old, err := m.Start(ch, src, []Reviewer{{Profile: "code"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	v := wait(t, old, func(v RunView) bool { return v.ReviewComplete })
	old.save()
	loaded := &Manager{StateDir: m.StateDir}
	if err := loaded.Load(); err != nil {
		t.Fatal(err)
	}
	if !loaded.Get(old.ID).View().ReviewComplete {
		t.Fatal("review completion lost on reload")
	}
	d := &summaryDriver{}
	m.Drivers = map[string]agent.Driver{"summary": d}
	if _, err = m.StartSummary(ch, src, Reviewer{}, old.ID); err == nil {
		t.Fatal("empty incremental range accepted")
	}
	newHead := historyCommit(t, src.Remote, "later-file")
	ch.Diff.Head, src.Head = newHead, newHead
	r, err := m.StartSummary(ch, src, Reviewer{}, old.ID)
	if err != nil {
		t.Fatal(err)
	}
	wait(t, r, summaryTerminal)
	awaitSummary(t, r)
	d.mu.Lock()
	s := d.session
	d.mu.Unlock()
	s.mu.Lock()
	input := s.text
	s.mu.Unlock()
	if !strings.Contains(input, "later-file") || strings.Contains(input, "-func Add") || !strings.Contains(input, v.Head+".."+newHead) {
		t.Fatalf("incremental input leaked earlier work: %s", input)
	}
	if sr := r.View().Summary; sr.Scope != "since_review" || sr.ReviewID != old.ID || sr.From != v.Head {
		t.Fatal("wrong baseline")
	}
	ch.Ref.Number++
	if _, err = m.StartSummary(ch, src, Reviewer{}, old.ID); err == nil {
		t.Fatal("another change supplied baseline")
	}
	ch.Ref.Number--
	old.mu.Lock()
	old.sessions[0].FirstComplete = false
	old.mu.Unlock()
	if _, err = m.StartSummary(ch, src, Reviewer{}, old.ID); err == nil {
		t.Fatal("unproven legacy/cancelled review supplied baseline")
	}
	_ = m.Close(context.Background(), old.ID)
	_ = m.Close(context.Background(), r.ID)
}

func TestSummaryCancellationCoversPreparingLateStartAndProvider(t *testing.T) {
	for _, phase := range []string{"repository", "startup", "generation"} {
		t.Run(phase, func(t *testing.T) {
			m, _, ch, src := setup(t, nil)
			d := &summaryDriver{started: make(chan struct{}), hold: true}
			m.Drivers = map[string]agent.Driver{"summary": d}
			var unlock func()
			if phase == "repository" {
				var err error
				unlock, err = m.Repos.lock(context.Background(), filepath.Join(m.Repos.Dir, filepath.FromSlash(src.Key)+".git"))
				if err != nil {
					t.Fatal(err)
				}
				defer unlock()
			}
			if phase == "startup" {
				d.gate = make(chan struct{})
			}
			r, err := m.StartSummary(ch, src, Reviewer{}, "")
			if err != nil {
				t.Fatal(err)
			}
			if phase == "startup" {
				select {
				case <-d.started:
				case <-time.After(5 * time.Second):
					t.Fatal("start did not arrive")
				}
			}
			if phase == "generation" {
				wait(t, r, func(v RunView) bool { return summaryText(v) == "Partial summary" })
			}
			if err := m.Stop(r.ID, "s1"); err != nil {
				t.Fatal(err)
			}
			if phase == "startup" {
				close(d.gate)
			}
			awaitSummary(t, r)
			v := r.View()
			if v.Status != Cancelled {
				t.Fatalf("cancel became %s: %s", v.Status, v.Error)
			}
			d.mu.Lock()
			s, n := d.session, len(d.specs)
			d.mu.Unlock()
			if phase == "repository" {
				if n != 0 {
					t.Fatal("cancelled preparation launched agent")
				}
			} else {
				s.mu.Lock()
				defer s.mu.Unlock()
				if s.cancels == 0 {
					t.Fatal("provider was not stopped")
				}
				if phase == "startup" && s.sends != 0 {
					t.Fatal("late start sent request after cancellation")
				}
			}
			loaded := &Manager{StateDir: m.StateDir}
			_ = loaded.Load()
			if loaded.Get(r.ID).View().Status != Cancelled {
				t.Fatal("cancel state lost")
			}
		})
	}
}

func TestSummaryRangeRefusesRewritesMissingObjectsAndOversizeBeforeAgent(t *testing.T) {
	for _, mode := range []string{"rewrite", "missing", "oversize"} {
		t.Run(mode, func(t *testing.T) {
			m, _, ch, src := setup(t, nil)
			d := &summaryDriver{}
			m.Drivers = map[string]agent.Driver{"summary": d}
			switch mode {
			case "rewrite":
				historyGit(t, src.Remote, "switch", "-qc", "other", src.Base)
				ch.Diff.Base = historyCommit(t, src.Remote, "other-file")
				src.Base = ch.Diff.Base
			case "missing":
				ch.Diff.Base = strings.Repeat("0", 40)
				src.Base = ch.Diff.Base
			case "oversize":
				if err := os.WriteFile(filepath.Join(src.Remote, "huge.txt"), []byte(strings.Repeat("long patch line\n", 30000)), 0600); err != nil {
					t.Fatal(err)
				}
				historyGit(t, src.Remote, "add", "huge.txt")
				historyGit(t, src.Remote, "commit", "-qm", "huge")
				src.Head = historyGit(t, src.Remote, "rev-parse", "HEAD")
				ch.Diff.Head = src.Head
			}
			r, err := m.StartSummary(ch, src, Reviewer{}, "")
			if err != nil {
				t.Fatal(err)
			}
			v := wait(t, r, summaryTerminal)
			awaitSummary(t, r)
			if v.Status != Failed {
				t.Fatalf("unsafe range succeeded: %s", v.Status)
			}
			d.mu.Lock()
			n := len(d.specs)
			d.mu.Unlock()
			if n != 0 {
				t.Fatal("unsafe/incomplete range spent model quota")
			}
			if mode == "oversize" && !strings.Contains(v.Error, "too large") {
				t.Fatalf("oversize error: %s", v.Error)
			}
		})
	}
}

func TestSummaryRoleHasNoDraftAccess(t *testing.T) {
	st := drafts.NewStore()
	s := drafts.Session{ID: "summary", Role: drafts.Summary}
	if len(drafts.ToolNames(drafts.Summary)) != 0 {
		t.Fatal("summary tools offered")
	}
	if _, err := st.Propose(s, drafts.Proposal{Severity: "low", Title: "x", Description: "x"}); err == nil {
		t.Fatal("summary drafted")
	}
	if _, err := st.Revise(s, "d1", drafts.Proposal{}); err == nil {
		t.Fatal("summary revised")
	}
	if _, err := st.Withdraw(s, "d1"); err == nil {
		t.Fatal("summary withdrew")
	}
	if _, err := st.Rate(s, "d1", drafts.Confirmed, "x"); err == nil {
		t.Fatal("summary rated")
	}
	if len(st.List(s, true)) != 0 {
		t.Fatal("summary read drafts")
	}
}

func TestFailedOrCancelledReviewCannotBecomeSummaryBaseline(t *testing.T) {
	for _, ev := range []agent.Event{{Kind: agent.TurnDone, Err: errors.New("fixture")}, {Kind: agent.TurnDone, Cancelled: true}} {
		m, _, ch, src := setup(t, nil)
		r := &Run{ID: "failed-review", Ref: ch.Ref, Kind: ch.Kind, Head: src.Head, Status: Working, store: drafts.NewStore(), path: filepath.Join(m.StateDir, "runs", "failed-review", "run.json")}
		s := &Session{ID: "s1", Role: drafts.Reviewer, Status: Working}
		r.sessions = []*Session{s}
		live := &summarySession{events: make(chan agent.Event, 2)}
		live.events <- ev
		_ = live.Close()
		m.pump(r, s, live)
		if r.View().ReviewComplete {
			t.Fatal("failed/cancelled first turn recorded as reviewed")
		}
	}
}

func TestSummaryShutdownAndCloseStopPreparationBeforeRemovingCheckout(t *testing.T) {
	for _, action := range []string{"shutdown", "close"} {
		t.Run(action, func(t *testing.T) {
			m, _, ch, src := setup(t, nil)
			d := &summaryDriver{started: make(chan struct{}), gate: make(chan struct{}), hold: true}
			m.Drivers = map[string]agent.Driver{"summary": d}
			r, err := m.StartSummary(ch, src, Reviewer{}, "")
			if err != nil {
				t.Fatal(err)
			}
			select {
			case <-d.started:
			case <-time.After(5 * time.Second):
				t.Fatal("driver never started")
			}
			done := make(chan error, 1)
			go func() {
				if action == "shutdown" {
					m.Shutdown()
					done <- nil
				} else {
					done <- m.Close(context.Background(), r.ID)
				}
			}()
			wait(t, r, func(v RunView) bool { return v.Status == Cancelled })
			// The checkout remains while a late startup could still be reading it.
			if _, err := os.Stat(r.dir); err != nil {
				t.Fatal("checkout removed during startup")
			}
			close(d.gate)
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("shutdown/close never finished")
			}
			d.mu.Lock()
			s := d.session
			d.mu.Unlock()
			s.mu.Lock()
			defer s.mu.Unlock()
			if s.sends != 0 || s.cancels == 0 {
				t.Fatal("late startup escaped shutdown")
			}
			if action == "close" {
				if _, err := os.Stat(r.dir); !os.IsNotExist(err) {
					t.Fatal("closed checkout remains")
				}
			}
		})
	}
}

func TestInterruptedSummaryReloadKeepsPartialOutputAndDoesNotResume(t *testing.T) {
	m, _, ch, src := setup(t, nil)
	d := &summaryDriver{hold: true}
	m.Drivers = map[string]agent.Driver{"summary": d}
	r, err := m.StartSummary(ch, src, Reviewer{}, "")
	if err != nil {
		t.Fatal(err)
	}
	wait(t, r, func(v RunView) bool { return summaryText(v) == "Partial summary" })
	r.save()
	loaded := &Manager{StateDir: m.StateDir}
	if err := loaded.Load(); err != nil {
		t.Fatal(err)
	}
	v := loaded.Get(r.ID).View()
	if v.Status != Failed || summaryText(v) != "Partial summary" || !strings.Contains(v.Error, "stopped before") {
		t.Fatal("interrupted state misrepresented")
	}
	if err := loaded.Say(r.ID, "s1", "continue"); err == nil {
		t.Fatal("summary resumed")
	}
	m.Shutdown()
}

func TestSummaryCancellationClosesATurnOnlyDriver(t *testing.T) {
	m, _, ch, src := setup(t, nil)
	d := &summaryDriver{hold: true, keepOpenOnCancel: true}
	m.Drivers = map[string]agent.Driver{"summary": d}
	r, err := m.StartSummary(ch, src, Reviewer{}, "")
	if err != nil {
		t.Fatal(err)
	}
	wait(t, r, func(v RunView) bool { return summaryText(v) == "Partial summary" })
	if err := m.Stop(r.ID, "s1"); err != nil {
		t.Fatal(err)
	}
	awaitSummary(t, r)
	m.Shutdown()
	d.mu.Lock()
	s := d.session
	d.mu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed || s.cancels == 0 || r.View().Status != Cancelled {
		t.Fatal("cancelled turn kept summary session alive")
	}
}

func TestFailedInitialReviewRemainsIneligibleAfterReloadAndSuccessfulFollowUp(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		m, _, ch, src := setup(t, nil)
		r := &Run{ID: "failed-initial", Ref: ch.Ref, Kind: ch.Kind, Head: src.Head, Status: Working, store: drafts.NewStore(), path: filepath.Join(m.StateDir, "runs", "failed-initial", "run.json")}
		s := &Session{ID: "s1", Role: drafts.Reviewer, Status: Working}
		r.sessions = []*Session{s}
		first := &summarySession{events: make(chan agent.Event, 2)}
		ev := agent.Event{Kind: agent.TurnDone, Cancelled: cancelled}
		if !cancelled {
			ev.Err = errors.New("fixture initial failure")
		}
		first.events <- ev
		_ = first.Close()
		m.pump(r, s, first)
		loaded := &Manager{StateDir: m.StateDir}
		if err := loaded.Load(); err != nil {
			t.Fatal(err)
		}
		back := loaded.Get(r.ID)
		follow := &summarySession{events: make(chan agent.Event, 2)}
		follow.events <- agent.Event{Kind: agent.TurnDone}
		_ = follow.Close()
		loaded.pump(back, back.sessions[0], follow)
		if back.View().ReviewComplete {
			t.Fatal("successful follow-up upgraded a failed initial review")
		}
	}
}
