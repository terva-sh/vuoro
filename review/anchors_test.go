package review

import (
	"errors"
	"testing"

	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/review/drafts"
)

func TestAnAnchorMustBeOnALineTheDiffShows(t *testing.T) {
	idx := indexLines([]forge.FileDiff{
		{OldPath: "a.go", NewPath: "a.go", Patch: "@@ -10,4 +10,5 @@ func f() {\n x\n-y\n+y2\n+y3\n z\n w\n@@ -40 +41 @@\n-q\n+r\n"},
		{OldPath: "old.go", NewPath: "new.go", Renamed: true, Patch: "@@ -1,2 +1,2 @@\n-a\n+b\n c\n"},
		{OldPath: "logo.png", NewPath: "logo.png", Omitted: true},
		{OldPath: "", NewPath: "fresh.go", New: true, Patch: "@@ -0,0 +1,2 @@\n+a\n+b\n"},
	})
	for _, c := range []struct {
		a    drafts.Anchor
		rule string
	}{
		{drafts.Anchor{Path: "a.go", Side: drafts.Head, StartLine: 10, EndLine: 14}, ""},
		{drafts.Anchor{Path: "a.go", Side: drafts.Head, StartLine: 41, EndLine: 41}, ""},
		{drafts.Anchor{Path: "a.go", Side: drafts.Base, StartLine: 40, EndLine: 40}, ""},
		{drafts.Anchor{Path: "a.go", Side: drafts.Head, StartLine: 14, EndLine: 41}, "anchor.notInDiff"}, // spans two hunks
		{drafts.Anchor{Path: "a.go", Side: drafts.Head, StartLine: 20, EndLine: 20}, "anchor.notInDiff"},
		{drafts.Anchor{Path: "new.go", Side: drafts.Head, StartLine: 2, EndLine: 2}, ""},
		{drafts.Anchor{Path: "old.go", Side: drafts.Base, StartLine: 1, EndLine: 1}, ""},
		{drafts.Anchor{Path: "old.go", Side: drafts.Head, StartLine: 1, EndLine: 1}, "anchor.notInDiff"},
		{drafts.Anchor{Path: "fresh.go", Side: drafts.Head, StartLine: 2, EndLine: 2}, ""},
		{drafts.Anchor{Path: "fresh.go", Side: drafts.Base, StartLine: 1, EndLine: 1}, "anchor.notInDiff"},
		{drafts.Anchor{Path: "logo.png", Side: drafts.Head, StartLine: 1, EndLine: 1}, "anchor.notShown"},
		{drafts.Anchor{Path: "elsewhere.go", Side: drafts.Head, StartLine: 1, EndLine: 1}, "anchor.notInChange"},
	} {
		err := idx.check(c.a)
		var re *drafts.RuleError
		switch {
		case c.rule == "" && err != nil:
			t.Errorf("%+v: refused: %v", c.a, err)
		case c.rule != "" && (!errors.As(err, &re) || re.Rule != c.rule):
			t.Errorf("%+v: %v, want rule %s", c.a, err, c.rule)
		}
	}
}
