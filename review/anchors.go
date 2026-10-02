package review

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/review/drafts"
)

// lineIndex holds, for each file in a change, the lines its diff shows on
// each side. A forge places an inline comment only on such a line, so a
// draft anchored anywhere else could never be posted where it points.
type lineIndex map[string]*fileLines

type fileLines struct {
	omitted    bool
	head, base []span // sorted, from the hunks
}

type span struct{ from, to int }

func (f *fileLines) side(s drafts.Side) []span {
	if s == drafts.Base {
		return f.base
	}
	return f.head
}

func indexLines(files []forge.FileDiff) lineIndex {
	idx := lineIndex{}
	for _, fd := range files {
		fl := &fileLines{omitted: fd.Omitted}
		for _, h := range hunks(fd.Patch) {
			if h.oldLen > 0 {
				fl.base = append(fl.base, span{h.oldStart, h.oldStart + h.oldLen - 1})
			}
			if h.newLen > 0 {
				fl.head = append(fl.head, span{h.newStart, h.newStart + h.newLen - 1})
			}
		}
		idx[fd.NewPath] = fl
		if fd.OldPath != fd.NewPath && fd.OldPath != "" {
			// A base-side anchor on a renamed file names its old path.
			old := &fileLines{omitted: fd.Omitted, base: fl.base}
			if _, ok := idx[fd.OldPath]; !ok {
				idx[fd.OldPath] = old
			}
		}
	}
	return idx
}

type hunk struct{ oldStart, oldLen, newStart, newLen int }

// hunks reads the @@ headers of a unified diff.
func hunks(patch string) []hunk {
	var out []hunk
	for _, line := range strings.Split(patch, "\n") {
		rest, ok := strings.CutPrefix(line, "@@ -")
		if !ok {
			continue
		}
		ranges, _, ok := strings.Cut(rest, " @@")
		if !ok {
			continue
		}
		o, n, ok := strings.Cut(ranges, " +")
		if !ok {
			continue
		}
		var h hunk
		h.oldStart, h.oldLen = parseRange(o)
		h.newStart, h.newLen = parseRange(n)
		out = append(out, h)
	}
	return out
}

func parseRange(s string) (start, n int) {
	a, b, ok := strings.Cut(s, ",")
	start, _ = strconv.Atoi(a)
	n = 1
	if ok {
		n, _ = strconv.Atoi(b)
	}
	return start, n
}

// check is the drafts store's CheckAnchor for this change. Its errors tell
// the agent where it may anchor instead, and never quote the proposal.
func (idx lineIndex) check(a drafts.Anchor) error {
	fl, ok := idx[a.Path]
	switch {
	case !ok:
		return &drafts.RuleError{Rule: "anchor.notInChange", Field: "anchor/path",
			Detail: "a file this change touches; for any other place, drop the anchor and name it in the description"}
	case fl.omitted:
		return &drafts.RuleError{Rule: "anchor.notShown", Field: "anchor/path",
			Detail: "a file whose diff is shown; this one is binary or too large, so drop the anchor and name the place in the description"}
	}
	spans := fl.side(a.Side)
	for _, sp := range spans {
		if a.StartLine >= sp.from && a.EndLine <= sp.to {
			return nil
		}
	}
	return &drafts.RuleError{Rule: "anchor.notInDiff", Field: "anchor/start_line",
		Detail: fmt.Sprintf("lines inside one hunk of the diff on the %s side, which here are %s; or drop the anchor", a.Side, describe(spans))}
}

func describe(spans []span) string {
	if len(spans) == 0 {
		return "none"
	}
	var parts []string
	for i, sp := range spans {
		if i == 20 {
			parts = append(parts, fmt.Sprintf("and %d more", len(spans)-i))
			break
		}
		parts = append(parts, fmt.Sprintf("%d-%d", sp.from, sp.to))
	}
	return strings.Join(parts, ", ")
}
