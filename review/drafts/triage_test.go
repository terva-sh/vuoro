package drafts

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTriageToolsBindRepliesAndProtectPersonEdits(t *testing.T) {
	st := NewStore()
	st.CheckThread = func(id string) (string, error) {
		if id == "t1" || id == "t2" {
			return "snapshot-key", nil
		}
		return "", &RuleError{"thread.unknown", "thread_id", "a supplied thread"}
	}
	sess := Session{ID: "author", Role: Triage, Drafted: "fake"}
	e := entry{store: st, sess: sess}
	call := func(name, raw string) bool { return e.call(name, json.RawMessage(raw))["isError"].(bool) }
	if !call("resolve", `{}`) || !call("submit", `{}`) || !call("rate_draft", `{}`) {
		t.Fatal("triage can reach a nonlocal tool")
	}
	for _, raw := range []string{
		`{"thread_id":"unknown","category":"must_fix","reason":"Bug"}`,
		`{"thread_id":"t1","category":"high","reason":"Bug"}`,
		`{"thread_id":"t1","category":"must_fix","reason":""}`,
		`{"thread_id":"t1","category":"must_fix","reason":"Bug","anchor":{"path":"a.go","start_line":1}}`,
	} {
		if !call("propose_draft", raw) {
			t.Fatal("invalid triage accepted")
		}
	}
	raw := `{"thread_id":"t1","category":"discuss","reason":"Need more evidence","description":"Could you share an example?"}`
	if call("propose_draft", raw) || !call("propose_draft", raw) {
		t.Fatal("proposal/duplicate rules failed")
	}
	d := st.All()[0]
	if d.ThreadKey != "snapshot-key" {
		t.Fatal("agent chose the snapshot key")
	}
	if !call("revise_draft", `{"id":"d1","thread_id":"t2","category":"discuss","reason":"Moved"}`) {
		t.Fatal("reply retargeted")
	}
	if _, err := st.EditReply(d.ID, "My own wording", MustFix); err != nil {
		t.Fatal(err)
	}
	if !call("revise_draft", `{"id":"d1","thread_id":"t1","category":"nit","reason":"New"}`) || !call("withdraw_draft", `{"id":"d1"}`) {
		t.Fatal("agent overwrote the person's edit")
	}
	if st.All()[0].Description != "My own wording" {
		t.Fatal("local reply lost")
	}
	ordinary := entry{store: st, sess: Session{ID: "reviewer", Role: Reviewer}}
	if !ordinary.call("propose_draft", json.RawMessage(raw))["isError"].(bool) {
		t.Fatal("reviewer can create triage replies")
	}
	if len(st.List(Session{ID: "other", Role: Triage}, false)) != 0 {
		t.Fatal("triage sees another session's drafts")
	}
}

func TestReplyAcceptanceSerializesDecisionAndRevision(t *testing.T) {
	st := NewStore()
	st.CheckThread = func(string) (string, error) { return "key", nil }
	sess := Session{ID: "s", Role: Triage}
	p := Proposal{ThreadID: "t", Category: Discuss, Reason: "Question", Description: "Original reply"}
	d, err := st.Propose(sess, p)
	if err != nil {
		t.Fatal(err)
	}
	entered, finish := make(chan struct{}), make(chan struct{})
	accepted := make(chan error, 1)
	go func() {
		accepted <- st.AcceptReply(d.ID, func(current Draft) (string, bool, error) {
			close(entered)
			<-finish
			if current.Description != "Original reply" {
				t.Error("inconsistent accepted text")
			}
			return "note1", false, nil
		})
	}()
	<-entered
	revised := make(chan error, 1)
	go func() { p.Description = "Racing reply"; _, err := st.Revise(sess, d.ID, p); revised <- err }()
	close(finish)
	if err := <-accepted; err != nil {
		t.Fatal(err)
	}
	if err := <-revised; err == nil || !strings.Contains(err.Error(), "decided") {
		t.Fatal("agent revised an accepted reply")
	}
	if err := st.AcceptReply(d.ID, func(Draft) (string, bool, error) {
		t.Error("duplicate acceptance reached forge callback")
		return "", false, nil
	}); err == nil {
		t.Fatal("duplicate accepted")
	}
}
