package review

import (
	"fmt"
	"strings"

	"github.com/terva-sh/vuoro/review/drafts"
)

// Findings is a run's result for a terminal (docs/plan.md section 6.7): every
// finding the person would work from in the window, with its verifier mark,
// in Markdown. There is no keep or drop step here, so nothing is left out for
// being undecided.
func Findings(v RunView) string {
	shown := Shown(v)
	bySeverity(shown)

	var b strings.Builder
	what := v.Ref.Display(v.Kind)
	if v.Kind == Local {
		what = "the local commits of " + v.Ref.Project
	}
	fmt.Fprintf(&b, "# Review of %s\n\n", what)
	if v.WebURL != "" {
		fmt.Fprintf(&b, "Change: %s\n", v.WebURL)
	}
	fmt.Fprintf(&b, "Title: %s\n", oneLine(v.Title))
	fmt.Fprintf(&b, "Reviewed at head `%s`, against merge base `%s`.\n", v.Head, v.Base)
	fmt.Fprintf(&b, "Run `%s`, cost %s.\n", v.ID, usd(v.CostUSD))
	if v.Follows != "" {
		// Two drafts can restate one earlier finding, so this counts the
		// earlier findings, not the drafts.
		again := map[string]bool{}
		for _, d := range shown {
			if d.Earlier != "" {
				again[d.Earlier] = true
			}
		}
		fmt.Fprintf(&b, "It follows run `%s`, which had %d finding%s; %d %s raised again here.\n", v.Follows, len(v.Earlier), plural(len(v.Earlier)), len(again), isAre(len(again)))
	}
	b.WriteString("\n")
	for _, s := range v.Sessions {
		model := s.Model
		if model == "" {
			model = "default model"
		}
		fmt.Fprintf(&b, "- %s %s on %s, %s: %s", s.Role, s.Profile, s.Tool, model, s.Status)
		if s.Error != "" {
			fmt.Fprintf(&b, " (%s)", oneLine(s.Error))
		}
		b.WriteString("\n")
	}
	if v.Error != "" {
		fmt.Fprintf(&b, "\nThe run failed: %s\n", oneLine(v.Error))
	}
	if len(shown) == 0 {
		b.WriteString("\nNo findings.\n")
		return b.String()
	}
	fmt.Fprintf(&b, "\n%d finding%s, drafted by agents. Read the code before acting on one: a finding can be wrong, and a doubtful mark says the verifier argued against it.\n", len(shown), plural(len(shown)))
	for i, d := range shown {
		var extra []string
		if d.Earlier != "" {
			extra = append(extra, fmt.Sprintf("- Raised before: finding `%s` of run `%s`, %s", d.Earlier, v.Follows, standing(v.Earlier, d.Earlier)))
		}
		writeFinding(&b, i+1, d, extra...)
	}
	return b.String()
}

// standing says where the earlier finding id stood when this run began.
func standing(earlier []drafts.Earlier, id string) string {
	for _, e := range earlier {
		if e.ID != id {
			continue
		}
		switch e.Standing {
		case drafts.StandingFixed:
			return fmt.Sprintf("which was fixed in `%s`, so the fix may not hold", shortCommit(e.FixedIn))
		case drafts.StandingDismissed:
			return "which the person dismissed"
		case drafts.StandingKept:
			return "which the person posted and is not fixed yet"
		}
	}
	return "which nobody had decided"
}

func shortCommit(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

func isAre(n int) string {
	if n == 1 {
		return "is"
	}
	return "are"
}

func usd(v float64) string {
	if v == 0 {
		return "unreported"
	}
	return fmt.Sprintf("$%.2f", v)
}
