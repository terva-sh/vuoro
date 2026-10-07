package drafts

import "testing"

func TestADraftRestatingAnEarlierFindingKeepsThePersonsDecision(t *testing.T) {
	earlier := EarlierOf([]Draft{
		{ID: "d1", Severity: "high", Title: "Dismissed", Status: Pending, Decision: Dropped},
		{ID: "d2", Severity: "high", Title: "Kept", Status: Pending, Decision: Kept, NoteID: "n9"},
		{ID: "d3", Severity: "low", Title: "Fixed", Status: Pending, FixedIn: "abc1234", FixedSubject: "Fix it"},
		{ID: "d4", Severity: "low", Title: "Open", Status: Pending},
		{ID: "d5", Title: "A reply", Status: Pending, ThreadID: "t1"},
	})
	if len(earlier) != 4 {
		t.Fatalf("triage replies are not findings: %+v", earlier)
	}
	for i, want := range []string{StandingDismissed, StandingKept, StandingFixed, StandingOpen} {
		if earlier[i].Standing != want {
			t.Fatalf("%s stands %s, want %s", earlier[i].ID, earlier[i].Standing, want)
		}
	}
	st := NewStore()
	st.SetEarlier(earlier)

	p := good()
	p.Earlier = "d1"
	d, err := st.Propose(alice, p)
	if err != nil || d.Earlier != "d1" || d.Decision != Dropped || !d.Carried {
		t.Fatalf("a dismissed finding restated arrives dismissed: %+v %v", d, err)
	}
	// Carried, it is decided, so the agent can no longer change it.
	if _, err := st.Revise(alice, d.ID, good()); rule(t, err) != "draft.decided" {
		t.Fatalf("revising a carried draft: %v", err)
	}

	p.Earlier = "d2"
	if d, _ := st.Propose(alice, p); d.Decision != Kept || d.NoteID != "n9" || !d.Carried {
		t.Fatalf("a kept finding restated stays the posted comment: %+v", d)
	}
	p.Earlier = "d4"
	open, _ := st.Propose(bob, p)
	if open.Earlier != "d4" || open.Decision != "" || open.Carried {
		t.Fatalf("an open finding restated is still the person's to decide: %+v", open)
	}

	p.Earlier = "d9"
	if _, err := st.Propose(alice, p); rule(t, err) != "earlier.unknown" {
		t.Fatalf("an unknown earlier finding: %v", err)
	}
	if n := len(st.All()); n != 3 {
		t.Fatalf("a refused proposal adds nothing: %d drafts", n)
	}
	// A revision that names a dismissed finding takes its decision.
	p.Earlier = "d1"
	if d, err := st.Revise(bob, open.ID, p); err != nil || d.Decision != Dropped || !d.Carried {
		t.Fatalf("revised to restate a dismissed finding: %+v %v", d, err)
	}

	// A synthesizer's merge takes the earlier finding its sources name.
	q := good()
	q.Earlier = "d4"
	src, _ := st.Propose(alice, q)
	m := good()
	m.Sources = []string{src.ID}
	if d, err := st.Propose(synth, m); err != nil || d.Earlier != "d4" || d.Carried {
		t.Fatalf("a merge of a restated finding: %+v %v", d, err)
	}
}

func TestATriageReplyNamesNoEarlierFinding(t *testing.T) {
	p := Proposal{ThreadID: "t1", Category: "nit", Reason: "style", Earlier: "d1"}
	if err := Validate(p, Triage); rule(t, err) != "triage.shape" {
		t.Fatalf("a triage reply with an earlier finding: %v", err)
	}
}
