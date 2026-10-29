package review

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/terva-sh/vuoro/agent"
	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/state"
)

func answers(v RunView) []string {
	var out []string
	for _, e := range v.Sessions[0].Log {
		if e.Kind == "assistant" {
			out = append(out, e.Text)
		}
	}
	return out
}

// A conversation reads the change and nothing else, takes follow-ups on its
// one session, and carries on after a restart by resuming that session.
func TestAnAskIsReadOnlyAndCarriesOnAcrossARestart(t *testing.T) {
	var mu sync.Mutex
	var asked []string
	m, d, ch, src := setup(t, func(call toolCaller, text string) string {
		mu.Lock()
		asked = append(asked, text)
		mu.Unlock()
		if out, isErr := call("list_drafts", nil); !isErr {
			return "reached a drafts server: " + out
		}
		return "answer " + text[:3]
	})
	r, err := m.StartAsk(ch, src, Reviewer{Tool: "fake"}, AskInput{Question: "Why does Add subtract now?"})
	if err != nil {
		t.Fatal(err)
	}
	v := wait(t, r, func(v RunView) bool { return len(answers(v)) == 1 && v.Status == Idle })
	if v.Mode != AskMode || v.Ask == nil || v.Ask.Title != "Why does Add subtract now?" || v.Ask.Prompt != "" || v.Summary.Scope != "change" {
		t.Fatalf("conversation %+v %+v", v.Ask, v.Summary)
	}
	d.mu.Lock()
	spec := d.specs[0]
	d.mu.Unlock()
	if spec.Drafts != nil || spec.Editor != nil || spec.Resume != "" || spec.Dir != r.dir || spec.SystemPrompt != askPrompt {
		t.Fatalf("an ask session got more than reading: %+v", spec)
	}
	if got := answers(v)[0]; got != "answer Why" {
		t.Fatalf("answer %q", got)
	}
	mu.Lock()
	first := asked[0]
	mu.Unlock()
	if !strings.HasPrefix(first, "Why does Add subtract now?") || !strings.Contains(first, src.Base+".."+src.Head) || !strings.Contains(first, "-func Add") {
		t.Fatalf("first message lacks the question, range, or diff:\n%s", first)
	}
	if len(v.Drafts) != 0 {
		t.Fatal("a conversation holds drafts")
	}

	if err := m.Say(r.ID, "s1", "And the tests?"); err != nil {
		t.Fatal(err)
	}
	wait(t, r, func(v RunView) bool { return len(answers(v)) == 2 && v.Status == Idle })
	d.mu.Lock()
	if len(d.specs) != 1 {
		t.Fatalf("a follow-up started %d sessions", len(d.specs))
	}
	d.mu.Unlock()

	m.Shutdown()
	loaded := &Manager{StateDir: m.StateDir, Repos: m.Repos, Drivers: map[string]agent.Driver{"fake": d}}
	t.Cleanup(loaded.Shutdown)
	if err := loaded.Load(); err != nil {
		t.Fatal(err)
	}
	back := loaded.Get(r.ID)
	if bv := back.View(); bv.Ask == nil || bv.Ask.Question != "Why does Add subtract now?" || bv.Summary == nil || len(answers(bv)) != 2 {
		t.Fatalf("reloaded %+v", bv)
	}
	if err := loaded.Say(r.ID, "s1", "One more"); err != nil {
		t.Fatal(err)
	}
	wait(t, back, func(v RunView) bool { return len(answers(v)) == 3 && v.Status == Idle })
	d.mu.Lock()
	resumed := d.specs[len(d.specs)-1]
	d.mu.Unlock()
	if resumed.Resume != "ref-1" || resumed.Drafts != nil || resumed.Editor != nil || resumed.SystemPrompt != askPrompt {
		t.Fatalf("resumed as %+v", resumed)
	}
}

func TestAnAskNeedsAQuestionAndACompletedBaseline(t *testing.T) {
	m, _, ch, src := setup(t, func(toolCaller, string) string { return "ok" })
	if _, err := m.StartAsk(ch, src, Reviewer{Tool: "fake"}, AskInput{Question: "  "}); err == nil {
		t.Fatal("an empty question started a conversation")
	}
	if _, err := m.StartAsk(ch, src, Reviewer{Tool: "fake"}, AskInput{Question: "Why?", Baseline: "nope"}); err != ErrNoRun {
		t.Fatalf("an unknown baseline: %v", err)
	}
	moved := *ch
	moved.Diff.Head = strings.Repeat("a", 40)
	if _, err := m.StartAsk(&moved, src, Reviewer{Tool: "fake"}, AskInput{Question: "Why?"}); err == nil {
		t.Fatal("a conversation started on a head the clone was not given")
	}
}

// A summary from before Ask is listed as an explain conversation, and its
// record is not rewritten.
func TestSummariesAreListedAsExplainAndLeftAsTheyAre(t *testing.T) {
	dir := t.TempDir()
	ref := forge.Ref{Host: "lab.test", Project: "g/calc", Number: 7}
	write := func(id, mode string, ref forge.Ref, at time.Time) string {
		rec := map[string]any{"id": id, "kind": forge.GitLab, "mode": mode, "ref": ref, "title": "Fix Add", "status": "ended", "created": at,
			"head": "bbbb", "sessions": []any{}, "summary": SummaryRange{Scope: "change", From: "aaaa", To: "bbbb"}}
		if mode == AskMode {
			rec["ask"] = Ask{Prompt: "risks", Title: "What is risky here", Question: "What could go wrong?"}
		}
		path := filepath.Join(dir, "runs", id, "run.json")
		if err := state.WriteJSON(path, rec); err != nil {
			t.Fatal(err)
		}
		return path
	}
	now := time.Now().UTC()
	old := write("old-summary", "summary", ref, now.Add(-time.Hour))
	write("new-ask", AskMode, ref, now)
	write("a-review", "", ref, now)
	write("other-summary", "summary", forge.Ref{Host: "lab.test", Project: "g/calc", Number: 8}, now)
	before, _ := os.ReadFile(old)
	m := &Manager{StateDir: dir}
	if err := m.Load(); err != nil {
		t.Fatal(err)
	}
	list := m.AsksOf(forge.GitLab, ref)
	if len(list) != 2 || list[0].ID != "new-ask" || list[1].ID != "old-summary" {
		t.Fatalf("asks %+v", list)
	}
	if a := list[1].Ask; a == nil || a.Prompt != Explain || a.Title != "Explain this change" {
		t.Fatalf("summary listed as %+v", a)
	}
	if a := list[0].Ask; a == nil || a.Prompt != "risks" {
		t.Fatalf("ask listed as %+v", a)
	}
	if after, _ := os.ReadFile(old); string(after) != string(before) {
		t.Fatal("listing rewrote the summary's record")
	}
	if m.Get("old-summary").View().Ask != nil {
		t.Fatal("the summary run itself gained an ask")
	}
	m.Get("old-summary").save()
	if after, _ := os.ReadFile(old); string(after) == "" || strings.Contains(string(after), `"ask"`) {
		t.Fatalf("the summary record was rewritten with an ask: %s", after)
	}
}

// A conversation about what changed since a completed review is given only
// that range.
func TestAnAskSinceAReviewIsGivenOnlyWhatChanged(t *testing.T) {
	var mu sync.Mutex
	var asked []string
	m, _, ch, src := setup(t, func(_ toolCaller, text string) string {
		mu.Lock()
		asked = append(asked, text)
		mu.Unlock()
		return "ok"
	})
	old, err := m.Start(ch, src, []Reviewer{{Profile: "code", Tool: "fake"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	reviewed := wait(t, old, func(v RunView) bool { return v.ReviewComplete }).Head
	if _, err := m.StartAsk(ch, src, Reviewer{Tool: "fake"}, AskInput{Question: "What changed?", Baseline: old.ID}); err == nil {
		t.Fatal("a conversation started on an empty range")
	}
	newHead := historyCommit(t, src.Remote, "later-file")
	ch.Diff.Head, src.Head = newHead, newHead
	r, err := m.StartAsk(ch, src, Reviewer{Tool: "fake"}, AskInput{Question: "What changed?", Baseline: old.ID})
	if err != nil {
		t.Fatal(err)
	}
	v := wait(t, r, func(v RunView) bool { return len(answers(v)) == 1 && v.Status == Idle })
	if v.Summary.Scope != "since_review" || v.Summary.ReviewID != old.ID || v.Summary.From != reviewed || v.Summary.To != newHead {
		t.Fatalf("range %+v", v.Summary)
	}
	mu.Lock()
	first := asked[len(asked)-1]
	mu.Unlock()
	if !strings.Contains(first, "later-file") || strings.Contains(first, "-func Add") || !strings.Contains(first, reviewed+".."+newHead) {
		t.Fatalf("the since-review message leaked earlier work:\n%s", first)
	}
}

// A conversation closed while its tool is starting gets no live session: the
// tool started late is closed, and the run stays closed.
func TestAnAskClosedWhileItsToolStartsDoesNotGoOn(t *testing.T) {
	m, _, ch, src := setup(t, nil)
	d := &summaryDriver{started: make(chan struct{}), gate: make(chan struct{})}
	m.Drivers = map[string]agent.Driver{"gated": d}
	r, err := m.StartAsk(ch, src, Reviewer{Tool: "gated"}, AskInput{Question: "Why?"})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-d.started:
	case <-time.After(10 * time.Second):
		t.Fatal("the tool never started")
	}
	if err := m.Close(context.Background(), r.ID); err != nil {
		t.Fatal(err)
	}
	close(d.gate)
	deadline := time.Now().Add(5 * time.Second)
	for {
		d.mu.Lock()
		s := d.session
		d.mu.Unlock()
		s.mu.Lock()
		closed, sends := s.closed, s.sends
		s.mu.Unlock()
		if closed {
			if sends != 0 {
				t.Fatal("the closed conversation was sent its question")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the tool started for a closed conversation was left running")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if v := r.View(); v.Status != Closed {
		t.Fatalf("the run is %s, not closed", v.Status)
	}
}
