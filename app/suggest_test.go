package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/review"
	"github.com/terva-sh/vuoro/review/drafts"
)

// suggestForge records the pending comments a fix writes, in order.
type suggestForge struct {
	capsForge
	created []forge.NewDraft
	updated map[string]string
}

func (f *suggestForge) CreateDraft(_ context.Context, _ forge.Ref, d forge.NewDraft) (forge.Draft, error) {
	f.created = append(f.created, d)
	return forge.Draft{ID: "n" + string(rune('0'+len(f.created))), Body: d.Body}, nil
}

func (f *suggestForge) UpdateDraft(_ context.Context, _ forge.Ref, id, body string) error {
	f.updated[id] = body
	return nil
}

// A fix's new comments go first, each on its lines, spanning them on
// GitHub, then the finding's own comment gets the suggestion that rides on
// it; a comment gone from the forge is written again in its place (D59).
func TestAFixWritesItsNewCommentsThenItsOwn(t *testing.T) {
	ch := &forge.Change{Ref: forge.Ref{Project: "g/p", Number: 1}, Diff: forge.DiffRefs{Head: "h"},
		Files: []forge.FileDiff{{OldPath: "a.go", NewPath: "a.go", Patch: "@@ -1,4 +1,4 @@\n a\n-b\n+B\n c\n d\n"}}}
	f := review.PlannedFix{Finding: "d1", Title: "Fix b", NoteID: "41",
		OnComment: &review.Placed{Path: "a.go", Start: 2, End: 2, Line: 2, Text: "bb", Block: "```suggestion:-0+0\nbb\n```"},
		Comments:  []review.Placed{{Path: "a.go", Start: 3, End: 4, Line: 4, Text: "C\nD", Block: "```suggestion\nC\nD\n```"}},
		Commits:   []review.EditorCommit{{SHA: strings.Repeat("a", 40), Subject: "Fix b"}}}
	for _, span := range []bool{false, true} {
		fake := &suggestForge{updated: map[string]string{}}
		s := &suggester{ch: ch, wr: fake, opt: review.SuggestOptions{Shows: shows(ch), Span: span},
			drafts: map[string]drafts.Draft{"d1": {ID: "d1", Title: "Fix b", Description: "b is wrong."}},
			pend:   map[string]string{"41": "**Fix b**\n\nb is wrong."}}
		a := &App{Runs: &review.Manager{}}
		n, err := a.writeFix(context.Background(), "no-run", s, f)
		// The run is not there to record it, which comes after every write.
		if !errors.Is(err, review.ErrNoRun) || n != 2 {
			t.Fatalf("span %v: wrote %d, %v", span, n, err)
		}
		if len(fake.created) != 1 || fake.created[0].Anchor.NewLine != 4 || !strings.HasPrefix(fake.created[0].Body, "Part of the fix for **Fix b**") {
			t.Fatalf("span %v: created %+v", span, fake.created)
		}
		if got, want := fake.created[0].Anchor.StartLine, map[bool]int{false: 0, true: 3}[span]; got != want {
			t.Fatalf("span %v: the new comment starts at %d", span, got)
		}
		if got := fake.updated["41"]; got != "**Fix b**\n\nb is wrong.\n\nSuggested fix:\n\n```suggestion:-0+0\nbb\n```" {
			t.Fatalf("span %v: updated %q", span, got)
		}
	}
	// The finding's comment is gone from the forge, so it is written again.
	fake := &suggestForge{updated: map[string]string{}}
	s := &suggester{ch: ch, wr: fake, opt: review.SuggestOptions{Shows: shows(ch)},
		drafts: map[string]drafts.Draft{"d1": {ID: "d1", Title: "Fix b", Description: "b is wrong.", Anchor: &drafts.Anchor{Path: "a.go", Side: drafts.Head, StartLine: 2, EndLine: 2}}},
		pend:   map[string]string{}}
	f.Comments = nil
	(&App{Runs: &review.Manager{}}).writeFix(context.Background(), "no-run", s, f)
	if len(fake.updated) != 0 || len(fake.created) != 1 || fake.created[0].Anchor.NewLine != 2 || !strings.HasPrefix(fake.created[0].Body, "**Fix b**\n\nb is wrong.\n\nSuggested fix:") {
		t.Fatalf("a missing comment: updated %v, created %+v", fake.updated, fake.created)
	}
}

// The planner reads the change's diff through shows: on GitLab a comment
// needs its last line shown, and on GitHub all its lines in one hunk.
func TestShowsReadsTheChangesDiffForTheForge(t *testing.T) {
	ch := &forge.Change{Files: []forge.FileDiff{
		{NewPath: "a.go", Patch: "@@ -1,3 +1,4 @@\n a\n+b\n c\n d\n@@ -20,2 +21,2 @@\n-x\n+y\n z\n"},
		{NewPath: "gone.go", Deleted: true, Patch: "@@ -1 +0,0 @@\n-x\n"},
	}}
	gl := shows(ch)
	if c, ok := gl("a.go", 3, 21); !c || !ok {
		t.Fatal("GitLab: a comment on a shown line was refused")
	}
	if c, ok := gl("a.go", 5, 10); !c || ok {
		t.Fatal("GitLab: a comment on a line the diff does not show was allowed")
	}
	if c, _ := gl("gone.go", 1, 1); c {
		t.Fatal("a deleted file counted as changed")
	}
	ch.Capabilities.SpanSuggestions = true
	if _, ok := shows(ch)("a.go", 3, 21); ok {
		t.Fatal("GitHub: a span across hunks was allowed")
	}
	if _, ok := shows(ch)("a.go", 1, 4); !ok {
		t.Fatal("GitHub: a span inside one hunk was refused")
	}
}
