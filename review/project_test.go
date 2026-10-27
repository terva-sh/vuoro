package review

import (
	"os"
	"strings"
	"testing"

	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/review/drafts"
)

func TestProjectTriageRunsLockedDownAndProposesOnlyWithinItsRun(t *testing.T) {
	refused := 0
	m, d, _, _ := setup(t, func(call toolCaller, text string) string {
		if !strings.Contains(text, "<facts>") || !strings.Contains(text, `"number":4`) {
			return "no facts"
		}
		for _, args := range []map[string]any{
			{"change": 9, "reviewers": []string{"bob"}, "reason": "Not in the run"},
			{"change": 4, "reviewers": []string{"mallory"}, "reason": "Not a candidate"},
			{"change": 4, "labels": []string{"made-up"}, "reason": "Not a label"},
		} {
			if _, bad := call("propose_draft", args); bad {
				refused++
			}
		}
		if _, bad := call("submit", map[string]any{}); !bad {
			return "submitted"
		}
		if out, bad := call("propose_draft", map[string]any{"change": 4, "reviewers": []string{"BOB"}, "labels": []string{"backend"}, "unblock": "Review it", "reason": "Bob owns api/"}); bad {
			return out
		}
		return "Proposed one."
	})
	in := ProjectTriageInput{Kind: forge.GitLab, Host: "gitlab.example", Name: "G / Calc", WebURL: "https://gitlab.example/g/calc",
		Triage: ProjectTriage{Path: "g/calc", Changes: []ProjectChange{{Number: 4, Stamp: "s4"}}, Candidates: []string{"bob"}, Labels: []string{"backend"}},
		Facts:  `{"changes":[{"number":4}]}`}
	r, err := m.StartProjectTriage(in, Reviewer{Tool: "fake"})
	if err != nil {
		t.Fatal(err)
	}
	v := wait(t, r, func(v RunView) bool { return idle(v) && len(v.Drafts) == 1 })
	if refused != 3 || v.Mode != ProjectMode || v.Sessions[0].Role != "project-triage" || v.Drafts[0].Change != 4 || v.Drafts[0].Unblock != "Review it" {
		t.Fatalf("refused %d, run %+v", refused, v)
	}
	d.mu.Lock()
	spec := d.specs[0]
	d.mu.Unlock()
	if spec.Editor != nil || spec.Drafts == nil || strings.Contains(strings.Join(spec.Drafts.Tools, ","), "submit") || !strings.Contains(spec.SystemPrompt, "untrusted") {
		t.Fatal("project triage is not locked down as a reviewer")
	}
	if entries, err := os.ReadDir(spec.Dir); err != nil || len(entries) != 0 {
		t.Fatalf("the session's directory is not empty: %v %v", entries, err)
	}
	if r.Stamp(4) != "s4" || r.Stamp(5) != "" {
		t.Fatal("stamps")
	}
	if runs := m.ProjectRuns("GITLAB.example", "G/calc"); len(runs) != 1 || runs[0] != r {
		t.Fatal("project runs")
	}

	// A reloaded run binds its proposals the same way.
	loaded := &Manager{StateDir: m.StateDir, Drafts: m.Drafts}
	if err := loaded.Load(); err != nil {
		t.Fatal(err)
	}
	got := loaded.Get(r.ID)
	if got == nil || got.Project == nil || got.Stamp(4) != "s4" {
		t.Fatal("the project triage record was lost")
	}
	if _, err := got.store.Propose(drafts.Session{ID: "s1", Role: drafts.ProjectTriage}, drafts.Proposal{Change: 9, Reviewers: []string{"bob"}, Reason: "x"}); err == nil {
		t.Fatal("a reloaded run lost its binding")
	}
	if err := loaded.AcceptProposal(r.ID, v.Drafts[0].ID, func(drafts.Draft) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if loaded.Get(r.ID).View().Drafts[0].Decision != drafts.Kept {
		t.Fatal("accepting did not keep the proposal")
	}
	if _, err := m.StartProjectTriage(ProjectTriageInput{Triage: ProjectTriage{Path: "g/calc"}}, Reviewer{Tool: "fake"}); err == nil {
		t.Fatal("a run with no changes started")
	}
}
