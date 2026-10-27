package drafts

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestProjectTriageProposesOnlyWithinItsRunAndThePersonAccepts(t *testing.T) {
	st := NewStore()
	st.CheckProposal = func(p Proposal) error {
		if p.Change != 4 && p.Change != 5 {
			return &RuleError{"change.unknown", "change", "a supplied change"}
		}
		for _, u := range p.Reviewers {
			if u != "bob" && u != "carol" {
				return &RuleError{"reviewer.unknown", "reviewers", "a supplied candidate"}
			}
		}
		return nil
	}
	e := entry{store: st, sess: Session{ID: "s1", Role: ProjectTriage, Drafted: "fake"}}
	call := func(name, raw string) bool { return e.call(name, json.RawMessage(raw))["isError"].(bool) }
	if !call("submit", `{}`) || !call("resolve", `{}`) || !call("rate_draft", `{}`) {
		t.Fatal("project triage can reach a nonlocal tool")
	}
	for _, raw := range []string{
		`{"change":9,"reviewers":["bob"],"reason":"Owns it"}`,             // not in the run
		`{"change":4,"reviewers":["mallory"],"reason":"Owns it"}`,         // not a candidate
		`{"change":4,"reason":"Nothing proposed"}`,                        // empty
		`{"change":4,"reviewers":["bob"],"reason":""}`,                    // no reason
		`{"change":4,"reviewers":["bob","bob"],"reason":"Twice"}`,         // duplicate
		`{"change":4,"unblock":"one\ntwo","reason":"Two lines"}`,          // multiline
		`{"change":4,"reviewers":["bob"],"reason":"x","severity":"high"}`, // a finding's field
	} {
		if !call("propose_draft", raw) {
			t.Fatalf("invalid proposal accepted: %s", raw)
		}
	}
	if call("propose_draft", `{"change":4,"reviewers":["bob"],"labels":["backend"],"unblock":"Fix the pipeline","reason":"Bob owns api/"}`) {
		t.Fatal("valid proposal refused")
	}
	if !call("propose_draft", `{"change":4,"reviewers":["carol"],"reason":"Again"}`) {
		t.Fatal("a second pending proposal for one change was accepted")
	}
	if !call("revise_draft", `{"id":"d1","change":5,"reviewers":["carol"],"reason":"Moved"}`) {
		t.Fatal("a proposal was moved to another change")
	}
	if call("revise_draft", `{"id":"d1","change":4,"reviewers":["carol"],"reason":"Carol instead"}`) {
		t.Fatal("revision refused")
	}
	d := st.All()[0]
	if d.Change != 4 || d.Reviewers[0] != "carol" || d.Decision != "" {
		t.Fatalf("proposal %+v", d)
	}
	ordinary := entry{store: st, sess: Session{ID: "r", Role: Reviewer}}
	if !ordinary.call("propose_draft", json.RawMessage(`{"severity":"high","title":"T","change":4,"reviewers":["bob"]}`))["isError"].(bool) {
		t.Fatal("a reviewer can propose project triage fields")
	}

	// Accepting runs the person's apply, and marks the proposal kept only if
	// it succeeds; a failed apply leaves it pending.
	failed := errors.New("forge said no")
	if err := st.AcceptProposal(d.ID, func(Draft) error { return failed }); !errors.Is(err, failed) || st.All()[0].Decision != "" {
		t.Fatal("a failed apply marked the proposal")
	}
	var applied Draft
	if err := st.AcceptProposal(d.ID, func(cur Draft) error { applied = cur; return nil }); err != nil || st.All()[0].Decision != Kept || applied.Reviewers[0] != "carol" {
		t.Fatalf("accept: %v %+v", err, st.All()[0])
	}
	if st.AcceptProposal(d.ID, func(Draft) error { t.Fatal("accepted twice"); return nil }) == nil {
		t.Fatal("an accepted proposal was accepted again")
	}
	if !call("revise_draft", `{"id":"d1","change":4,"reviewers":["bob"],"reason":"After"}`) {
		t.Fatal("the agent revised an accepted proposal")
	}
}

func TestProjectTriageWithoutABindingProposesNothing(t *testing.T) {
	st := NewStore()
	if _, err := st.Propose(Session{ID: "s", Role: ProjectTriage}, Proposal{Change: 1, Reviewers: []string{"bob"}, Reason: "x"}); err == nil {
		t.Fatal("an unbound proposal was accepted")
	}
}
