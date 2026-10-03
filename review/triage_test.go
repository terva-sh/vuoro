package review

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/review/drafts"
)

func TestAuthorTriageUsesCompleteThreadContextAndPersistsLocalReplies(t *testing.T) {
	m, driver, ch, src := setup(t, func(call toolCaller, text string) string {
		if !strings.Contains(text, "Second undated reply") {
			t.Error("thread context was truncated")
		}
		if _, bad := call("submit", map[string]any{}); !bad {
			t.Error("agent can submit")
		}
		_, bad := call("propose_draft", map[string]any{"thread_id": "human", "category": "must_fix", "reason": "Add subtracts", "description": "I will check this."})
		if bad {
			t.Error("valid thread proposal failed")
		}
		return "Triaged one thread."
	})
	ch.Kind = forge.GitLab
	ch.State = forge.Open
	ch.Author.Username = "me"
	ch.Threads = []forge.Thread{
		{ID: "human", Resolvable: true, Notes: []forge.Note{{ID: "n1", Author: forge.User{Username: "reviewer"}, Body: "Add subtracts"}, {ID: "n2", Author: forge.User{Username: "reviewer"}, Body: "Second undated reply"}}},
		{ID: "bot", Notes: []forge.Note{{Author: forge.User{Username: "bot", Bot: true}, Body: "Bot"}}},
		{ID: "system", Notes: []forge.Note{{System: true, Author: forge.User{Username: "reviewer"}}}},
		{ID: "closed", Resolved: true, Notes: []forge.Note{{Author: forge.User{Username: "reviewer"}}}},
		{ID: "mine", Notes: []forge.Note{{Author: forge.User{Username: "me"}}}},
	}
	r, err := m.StartTriage(ch, src, Reviewer{Tool: "fake"})
	if err != nil {
		t.Fatal(err)
	}
	v := wait(t, r, func(v RunView) bool { return v.Status == Idle && len(v.Drafts) == 1 })
	if v.Mode != "triage" || len(v.Threads) != 1 || len(v.Sessions) != 1 || v.Sessions[0].Role != "triage" {
		t.Fatal("triage run shape")
	}
	driver.mu.Lock()
	spec := driver.specs[0]
	driver.mu.Unlock()
	if strings.Contains(strings.Join(spec.Drafts.Tools, ","), "submit") || !strings.Contains(spec.SystemPrompt, "untrusted") {
		t.Fatal("triage lockdown contract")
	}
	d := v.Drafts[0]
	v.Threads[0].Notes[0].Body = "A caller changed its copy"
	if r.View().Threads[0].Notes[0].Body != "Add subtracts" {
		t.Fatal("run view exposed mutable thread snapshot")
	}
	if err := m.CheckReply(r.ID, d, ch); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*forge.Change){
		func(c *forge.Change) { c.Ref.Number++ }, func(c *forge.Change) { c.Diff.Head = "moved" }, func(c *forge.Change) { c.State = forge.Closed },
		func(c *forge.Change) { c.Threads[0].Resolved = true }, func(c *forge.Change) { c.Threads[0].Notes[0].Body = "Edited without a timestamp" },
	} {
		b, _ := json.Marshal(ch)
		var changed forge.Change
		json.Unmarshal(b, &changed)
		mutate(&changed)
		if m.CheckReply(r.ID, d, &changed) == nil {
			t.Fatal("changed snapshot accepted")
		}
	}
	if err := m.EditReply(r.ID, d.ID, "The person's reply", drafts.Discuss); err != nil {
		t.Fatal(err)
	}
	loaded := &Manager{StateDir: m.StateDir}
	if err := loaded.Load(); err != nil {
		t.Fatal(err)
	}
	restored := loaded.Get(r.ID)
	got := restored.View()
	if got.Mode != "triage" || got.Drafts[0].Description != "The person's reply" || !got.Drafts[0].Edited || got.Sessions[0].Role != "triage" || !strings.Contains(restored.first, "Second undated reply") {
		t.Fatal("triage record lost")
	}
	if err := loaded.CheckReply(r.ID, got.Drafts[0], ch); err != nil {
		t.Fatal("restored thread binding:", err)
	}
}
