package review

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/review/drafts"
)

func TestTheReportListsEveryKeptFindingAndNothingElse(t *testing.T) {
	v := RunView{
		Ref: forge.Ref{Host: "gitlab.com", Project: "g/p", Number: 7}, Title: "Add\nthings", WebURL: "https://gitlab.com/g/p/-/merge_requests/7",
		Head: "h1", Base: "b1",
		Drafts: []drafts.Draft{
			{ID: "d1", Severity: "low", Title: "Kept low", Description: "low text", Status: drafts.Pending, Decision: drafts.Kept,
				Anchor: &drafts.Anchor{Path: "a.go", Side: drafts.Head, StartLine: 3, EndLine: 5}},
			{ID: "d2", Severity: "high", Title: "Dropped", Description: "no", Status: drafts.Pending, Decision: drafts.Dropped},
			{ID: "d3", Severity: "critical", Title: "Undecided", Description: "no", Status: drafts.Pending},
			{ID: "d4", Severity: "critical", Title: "Withdrawn", Description: "no", Status: drafts.Withdrawn},
			{ID: "d5", Severity: "high", Title: "Kept high", Description: "high text", Evidence: "x := \"```\"", Status: drafts.Pending, Decision: drafts.Kept, Edited: true,
				Confidence: &drafts.Confidence{Level: drafts.Doubtful, Reason: "guarded", By: "s4"},
				Override:   &drafts.Confidence{Level: drafts.Confirmed, Reason: "seen in staging", By: "person"}},
		},
	}
	got := Report(v)
	for _, absent := range []string{"Dropped", "Undecided", "Withdrawn"} {
		if strings.Contains(got, absent) {
			t.Errorf("the report carries %q:\n%s", absent, got)
		}
	}
	for _, want := range []string{
		"# Fix the review findings on gitlab.com/g/p!7", "Title: Add things", "`h1`",
		"## 1. Kept high", "## 2. Kept low", "`a.go` lines 3–5", "Confidence: confirmed, by the reviewer: seen in staging",
		"edited this before posting", "high text", "low text", "````\nx := \"```\"\n````",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the report lacks %q:\n%s", want, got)
		}
	}
	if none := Report(RunView{Ref: v.Ref}); !strings.Contains(none, "nothing to fix") {
		t.Fatalf("an empty report: %s", none)
	}
}

func TestTheReportIsWrittenInTheRun(t *testing.T) {
	m, _, ch, src := setup(t, func(call toolCaller, text string) string {
		call("propose_draft", map[string]any{"severity": "high", "title": "Add subtracts", "description": "d"})
		return "done"
	})
	r, err := m.Start(ch, src, []Reviewer{{Profile: "code"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	wait(t, r, idle)
	if err := m.Decide(r.ID, "d1", drafts.Kept, "41", false); err != nil {
		t.Fatal(err)
	}
	text, path, err := m.WriteReport(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil || string(b) != text || path != filepath.Join(m.StateDir, "runs", r.ID, "report.md") || !strings.Contains(text, "Add subtracts") {
		t.Fatalf("path %s, err %v, file %q", path, err, b)
	}
	m.Close(t.Context(), r.ID)
}
