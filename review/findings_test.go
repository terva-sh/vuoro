package review

import (
	"strings"
	"testing"

	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/review/drafts"
)

func TestFindingsShowWhatTheWindowShows(t *testing.T) {
	v := RunView{
		ID: "r1", Kind: Local, Ref: forge.Ref{Host: LocalHost, Project: "vuoro-1a2b3c4d"},
		Title: "Add a command", Head: "head", Base: "base", CostUSD: 0.42,
		Sessions: []SessionView{
			{ID: "s1", Profile: "code", Tool: "claude", Model: "sonnet", Role: "reviewer", Status: Idle},
			{ID: "s2", Profile: "security", Tool: "claude", Role: "reviewer", Status: Failed, Error: "the tool\nexited"},
			{ID: "s3", Profile: "synthesis", Tool: "claude", Role: "synthesizer", Status: Idle},
		},
		Drafts: []drafts.Draft{
			{ID: "d1", Session: "s1", Status: drafts.Pending, Severity: "high", Title: "Raw reviewer finding", Description: "merged below"},
			{ID: "d2", Session: "s3", Status: drafts.Pending, Severity: "low", Title: "A nit", Description: "small"},
			{ID: "d3", Session: "s3", Status: drafts.Pending, Severity: "critical", Title: "A real bug", Description: "big",
				Anchor: &drafts.Anchor{Path: "main.go", StartLine: 4, EndLine: 6}, Confidence: &drafts.Confidence{Level: drafts.Confirmed, Reason: "line 5 shows it"}},
			{ID: "d4", Session: "s3", Status: drafts.Withdrawn, Severity: "high", Title: "Withdrawn one", Description: "gone"},
		},
	}
	out := Findings(v)
	for _, want := range []string{
		"# Review of the local commits of vuoro-1a2b3c4d",
		"Run `r1`, cost $0.42.",
		"- reviewer security on claude, default model: failed (the tool exited)",
		"2 findings, drafted by agents.",
		"## 1. A real bug",
		"- Where: `main.go` lines 4–6",
		"- Confidence: confirmed, by the verifier: line 5 shows it",
		"## 2. A nit",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	for _, not := range []string{"Raw reviewer finding", "Withdrawn one", "Change: "} {
		if strings.Contains(out, not) {
			t.Errorf("%q should not appear in:\n%s", not, out)
		}
	}

	// With no synthesizer drafts, every pending draft is shown.
	v.Drafts = v.Drafts[:1]
	if got := Shown(v); len(got) != 1 || got[0].ID != "d1" {
		t.Fatalf("shown %+v", got)
	}
	v.Drafts = nil
	if out := Findings(v); !strings.Contains(out, "\nNo findings.\n") {
		t.Fatalf("no findings:\n%s", out)
	}
}
