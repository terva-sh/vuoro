package app

import (
	"strings"
	"testing"

	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/review/drafts"
)

func TestAKeptFindingPostsASuggestionBlockCoveringItsRange(t *testing.T) {
	fix := "\treturn a + b\n\t// sums"
	d := drafts.Draft{Title: "Add subtracts", Description: "It returns a - b.", Evidence: "return a - b",
		Anchor: &drafts.Anchor{Path: "calc.go", Side: drafts.Head, StartLine: 3, EndLine: 4}, Suggestion: &fix}
	applies := forge.Capabilities{Suggestions: true}
	got := keepBody(d, applies)
	want := "**Add subtracts**\n\nIt returns a - b.\n\n```\nreturn a - b\n```\n\n```suggestion:-1+0\n\treturn a + b\n\t// sums\n```"
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
	// A forge that cannot apply a suggestion gets the replacement as plain
	// code, saying which lines it replaces (D43).
	got = keepBody(d, forge.Capabilities{})
	want = "**Add subtracts**\n\nIt returns a - b.\n\n```\nreturn a - b\n```\n\nSuggested replacement for lines 3–4:\n\n```\n\treturn a + b\n\t// sums\n```"
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
	d.Suggestion = nil
	if strings.Contains(keepBody(d, applies), "suggestion") || strings.Contains(keepBody(d, forge.Capabilities{}), "replacement") {
		t.Fatal("a draft with no suggestion posted a block")
	}
}

// GitHub's suggestion replaces the lines its comment spans (D59), so its
// block names no offsets, and the comment spans the range, inside one hunk.
func TestOnGitHubAKeptSuggestionSpansItsRange(t *testing.T) {
	fix := "\treturn a + b\n\t// sums"
	d := drafts.Draft{Title: "Add subtracts", Description: "It returns a - b.",
		Anchor: &drafts.Anchor{Path: "calc.go", Side: drafts.Head, StartLine: 3, EndLine: 4}, Suggestion: &fix}
	got := keepBody(d, forge.Capabilities{Suggestions: true, SpanSuggestions: true})
	if want := "**Add subtracts**\n\nIt returns a - b.\n\n```suggestion\n\treturn a + b\n\t// sums\n```"; got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
	ch := &forge.Change{Files: []forge.FileDiff{{NewPath: "calc.go", Patch: "@@ -1,4 +1,5 @@\n a\n b\n-c\n+c2\n+d\n e\n@@ -20,1 +21,1 @@\n-x\n+y"}}}
	a := &forge.Anchor{Path: "calc.go", NewLine: 4}
	if err := spanLines(ch, a, 3); err != nil || a.StartLine != 3 {
		t.Fatalf("span %+v %v", a, err)
	}
	far := &forge.Anchor{Path: "calc.go", NewLine: 21}
	if err := spanLines(ch, far, 4); err == nil || far.StartLine != 0 {
		t.Fatalf("a span across hunks was allowed: %+v", far)
	}
}
