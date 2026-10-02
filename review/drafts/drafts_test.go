package drafts

import (
	"errors"
	"strings"
	"testing"
)

var (
	alice = Session{ID: "alice", Role: Reviewer, Drafted: "claude haiku"}
	bob   = Session{ID: "bob", Role: Reviewer, Drafted: "claude sonnet"}
	synth = Session{ID: "synth", Role: Synthesizer, Drafted: "claude opus"}
)

func good() Proposal {
	return Proposal{
		Severity: "high", Title: "Mean skips the first element",
		Description: "The loop starts at 1.",
		Anchor:      &Anchor{Path: "calc.go", StartLine: 6},
	}
}

func rule(t *testing.T, err error) string {
	t.Helper()
	var re *RuleError
	if !errors.As(err, &re) {
		t.Fatalf("error %v is not a RuleError", err)
	}
	return re.Rule
}

func TestAnInvalidDraftReturnsTheRuleItBroke(t *testing.T) {
	for want, edit := range map[string]func(*Proposal){
		"severity.value":     func(p *Proposal) { p.Severity = "urgent" },
		"title.empty":        func(p *Proposal) { p.Title = "  " },
		"title.length":       func(p *Proposal) { p.Title = strings.Repeat("x", 257) },
		"title.oneLine":      func(p *Proposal) { p.Title = "two\nlines" },
		"description.empty":  func(p *Proposal) { p.Description = "" },
		"description.length": func(p *Proposal) { p.Description = strings.Repeat("x", maxText+1) },
		"evidence.length":    func(p *Proposal) { p.Evidence = strings.Repeat("x", maxText+1) },
		"anchor.path":        func(p *Proposal) { p.Anchor.Path = "../outside.go" },
		"anchor.side":        func(p *Proposal) { p.Anchor.Side = "left" },
		"anchor.line":        func(p *Proposal) { p.Anchor.StartLine = 0 },
		"anchor.range":       func(p *Proposal) { p.Anchor.StartLine, p.Anchor.EndLine = 9, 3 },
		"sources.reviewer":   func(p *Proposal) { p.Sources = []string{"d1"} },
	} {
		p := good()
		edit(&p)
		_, err := NewStore().Propose(alice, p)
		if got := rule(t, err); got != want {
			t.Errorf("rule %q, want %q", got, want)
		}
	}
	for _, bad := range []string{"", "/etc/passwd", "a/../../b", "./calc.go", "a//b", ".."} {
		p := good()
		p.Anchor.Path = bad
		if got := rule(t, Validate(p, Reviewer)); got != "anchor.path" {
			t.Errorf("path %q: rule %q", bad, got)
		}
	}
}

func TestTheRuleErrorDoesNotQuoteTheProposal(t *testing.T) {
	p := good()
	p.Severity = "SECRET-LOOKING-VALUE"
	err := Validate(p, Reviewer)
	if strings.Contains(err.Error(), "SECRET-LOOKING-VALUE") {
		t.Fatalf("error quotes the proposal: %v", err)
	}
}

func TestAReviewerProposesRevisesAndWithdrawsItsOwnDraft(t *testing.T) {
	st := NewStore()
	d, err := st.Propose(alice, good())
	if err != nil {
		t.Fatal(err)
	}
	if d.ID != "d1" || d.Status != Pending || d.Revision != 1 || d.Session != "alice" || d.Drafted != "claude haiku" {
		t.Fatalf("draft %+v", d)
	}
	if d.Anchor.Side != Head || d.Anchor.EndLine != 6 {
		t.Fatalf("anchor defaults %+v", *d.Anchor)
	}

	p := good()
	p.Severity = "critical"
	d, err = st.Revise(alice, "d1", p)
	if err != nil || d.Severity != "critical" || d.Revision != 2 {
		t.Fatalf("revise: %+v %v", d, err)
	}
	d, err = st.Withdraw(alice, "d1")
	if err != nil || d.Status != Withdrawn {
		t.Fatalf("withdraw: %+v %v", d, err)
	}
	if got := st.List(alice, false); len(got) != 0 {
		t.Fatalf("withdrawn draft listed: %+v", got)
	}
	if got := st.List(alice, true); len(got) != 1 {
		t.Fatalf("withdrawn draft gone: %+v", got)
	}
	if _, err := st.Revise(alice, "d1", good()); rule(t, err) != "draft.withdrawn" {
		t.Fatalf("revised a withdrawn draft: %v", err)
	}
}

func TestAReviewerSeesAndChangesOnlyItsOwnDrafts(t *testing.T) {
	st := NewStore()
	if _, err := st.Propose(alice, good()); err != nil {
		t.Fatal(err)
	}
	if got := st.List(bob, true); len(got) != 0 {
		t.Fatalf("bob sees alice's drafts: %+v", got)
	}
	if _, err := st.Revise(bob, "d1", good()); rule(t, err) != "draft.owner" {
		t.Fatalf("bob revised alice's draft: %v", err)
	}
	if _, err := st.Withdraw(bob, "d1"); rule(t, err) != "draft.owner" {
		t.Fatalf("bob withdrew alice's draft: %v", err)
	}
	if _, err := st.Withdraw(bob, "d9"); rule(t, err) != "draft.unknown" {
		t.Fatalf("unknown draft: %v", err)
	}
}

func TestASynthesizerMergesDraftsAndNamesItsSources(t *testing.T) {
	st := NewStore()
	a, _ := st.Propose(alice, good())
	b, _ := st.Propose(bob, good())
	if got := st.List(synth, false); len(got) != 2 {
		t.Fatalf("synthesizer sees %d drafts, want 2", len(got))
	}

	p := good()
	if _, err := st.Propose(synth, p); rule(t, err) != "sources.required" {
		t.Fatalf("a merge without sources: %v", err)
	}
	p.Sources = []string{a.ID, "d9"}
	if _, err := st.Propose(synth, p); rule(t, err) != "sources.unknown" {
		t.Fatalf("an unknown source: %v", err)
	}
	p.Sources = []string{a.ID, a.ID}
	if _, err := st.Propose(synth, p); rule(t, err) != "sources.duplicate" {
		t.Fatalf("a repeated source: %v", err)
	}
	p.Sources = []string{a.ID, b.ID}
	m, err := st.Propose(synth, p)
	if err != nil {
		t.Fatal(err)
	}
	if m.Session != "synth" || len(m.Sources) != 2 || m.Sources[0] != "d1" || m.Sources[1] != "d2" {
		t.Fatalf("merged %+v", m)
	}
	// The sources stay, so the person can see what the merge came from.
	if got := st.All(); len(got) != 3 || got[0].Status != Pending {
		t.Fatalf("all %+v", got)
	}
}

func TestAPersonsMarkWinsAndTheVerifiersStaysBesideIt(t *testing.T) {
	st := NewStore()
	v := Session{ID: "v", Role: Verifier}
	d, _ := st.Propose(alice, Proposal{Severity: "low", Title: "t", Description: "d"})
	if _, err := st.Rate(alice, d.ID, Confirmed, "r"); err == nil {
		t.Fatal("a reviewer rated")
	}
	for _, bad := range []string{"", "  ", "two\nlines"} {
		if _, err := st.Rate(v, d.ID, Doubtful, bad); err == nil {
			t.Fatalf("reason %q was taken", bad)
		}
	}
	if _, err := st.Rate(v, d.ID, Doubtful, "the guard on line 4 already handles it"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Override(d.ID, Confirmed, "I hit it in staging"); err != nil {
		t.Fatal(err)
	}
	// The verifier rating again changes its own mark, never the person's.
	st.Rate(v, d.ID, Plausible, "on a second look")
	got, _ := st.Get(d.ID)
	if got.Override == nil || got.Override.Level != Confirmed || got.Override.By != "person" || got.Confidence.Level != Plausible {
		t.Fatalf("override %+v, mark %+v", got.Override, got.Confidence)
	}
	// It outlives a restart.
	back, _ := Restore(st.All()).Get(d.ID)
	if back.Override == nil || back.Override.Level != Confirmed {
		t.Fatalf("restored %+v", back.Override)
	}
	if _, err := st.Override(d.ID, "", ""); err != nil {
		t.Fatal(err)
	}
	if got, _ := st.Get(d.ID); got.Override != nil {
		t.Fatal("the override was not taken back")
	}
	if _, err := st.Override(d.ID, "certain", ""); err == nil {
		t.Fatal("a bad level was taken")
	}
	st.Withdraw(alice, d.ID)
	if _, err := st.Rate(v, d.ID, Confirmed, "r"); err == nil {
		t.Fatal("a withdrawn draft was rated")
	}
}
