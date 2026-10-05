package review

import (
	"fmt"
	"strings"
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
	fmt.Fprintf(&b, "Run `%s`, cost %s.\n\n", v.ID, usd(v.CostUSD))
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
		writeFinding(&b, i+1, d)
	}
	return b.String()
}

func usd(v float64) string {
	if v == 0 {
		return "unreported"
	}
	return fmt.Sprintf("$%.2f", v)
}
