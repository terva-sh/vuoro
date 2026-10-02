package app

import (
	"strings"
	"testing"

	"github.com/terva-sh/vuoro/review/drafts"
)

func TestAKeptFindingPostsASuggestionBlockCoveringItsRange(t *testing.T) {
	fix := "\treturn a + b\n\t// sums"
	d := drafts.Draft{Title: "Add subtracts", Description: "It returns a - b.", Evidence: "return a - b",
		Anchor: &drafts.Anchor{Path: "calc.go", Side: drafts.Head, StartLine: 3, EndLine: 4}, Suggestion: &fix}
	got := keepBody(d)
	want := "**Add subtracts**\n\nIt returns a - b.\n\n```\nreturn a - b\n```\n\n```suggestion:-1+0\n\treturn a + b\n\t// sums\n```"
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
	d.Suggestion = nil
	if strings.Contains(keepBody(d), "suggestion") {
		t.Fatal("a draft with no suggestion posted a block")
	}
}
