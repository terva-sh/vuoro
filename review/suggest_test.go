package review

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/terva-sh/vuoro/agent"
	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/review/drafts"
)

// A suggestion replaces lines, so a change that only adds lines takes the
// line before it, one that only removes lines takes the line before them,
// and changes a few lines apart join through the lines between.
func TestChangesBecomeEditsThatReplaceLines(t *testing.T) {
	var parent []string
	for i := 1; i <= 12; i++ {
		parent = append(parent, fmt.Sprintf("l%d", i))
	}
	hs := parseHunks("@@ -3 +3 @@\n-l3\n+b\n@@ -5,0 +6,2 @@\n+x\n+y\n@@ -9,2 +10,0 @@\n-l9\n-l10\n")
	eds := editsOf(hs, parent)
	want := []edit{{3, 3, []string{"b"}}, {5, 5, []string{"l5", "x", "y"}}, {8, 10, []string{"l8"}}}
	if fmt.Sprint(eds) != fmt.Sprint(want) {
		t.Fatalf("edits %v", eds)
	}
	joined := joinEdits(eds, parent)
	if fmt.Sprint(joined) != fmt.Sprint([]edit{{3, 10, []string{"b", "l4", "l5", "x", "y", "l6", "l7", "l8"}}}) {
		t.Fatalf("joined %v", joined)
	}
	// Lines added at the top take the first line, and lines removed from
	// the top take the line after them.
	top := editsOf(parseHunks("@@ -0,0 +1 @@\n+// top\n@@ -1,2 +1,0 @@\n-l1\n-l2\n"), parent)
	if fmt.Sprint(top) != fmt.Sprint([]edit{{1, 1, []string{"// top", "l1"}}, {1, 3, []string{"l3"}}}) {
		t.Fatalf("top %v", top)
	}
}

// A later commit's lines map back to the reviewed head through the wave's
// earlier commits, and lines an earlier commit changed, or removed between,
// cannot.
func TestALaterCommitsLinesMapBackToTheReviewedHead(t *testing.T) {
	pl := &planner{earlier: map[string][][]diffHunk{
		"a.go": {
			parseHunks("@@ -2,0 +3,2 @@\n+n1\n+n2\n"), // two lines added after line 2
			parseHunks("@@ -7,2 +6,0 @@\n-x\n-y\n"),   // then two removed after line 6
		},
	}}
	for _, c := range []struct {
		start, end, want int
		ok               bool
	}{
		{1, 2, 1, true},    // before both
		{10, 11, 10, true}, // after both: +2 removed, -2 added
		{3, 3, 0, false},   // an added line
		{2, 5, 0, false},   // spans the added lines
		{6, 7, 0, false},   // lines removed between them
		{5, 6, 3, true},    // between the two
	} {
		got, ok := pl.back("a.go", c.start, c.end)
		if ok != c.ok || ok && got != c.want {
			t.Errorf("back(%d, %d) = %d, %v", c.start, c.end, got, ok)
		}
	}
}

// What goes on the finding's comment is the comment as it stands, then the
// suggestion, then what cannot be one: inline within the cap, attached on a
// forge that takes uploads, and only counted otherwise. Never the whole
// content past the cap.
func TestAFixCommentInlinesAttachesOrCountsWhatCannotBeASuggestion(t *testing.T) {
	f := PlannedFix{Title: "Add subtracts", OnComment: &Placed{Block: "```suggestion:-0+0\nreturn a + b\n```"},
		Misfits: []Misfit{{Path: "NOTES.md", Reason: "it adds the file"}}, Diff: "--- /dev/null\n+++ b/NOTES.md\n@@ -0,0 +1 @@\n+note"}
	got := f.CommentBody("**Add subtracts**\n\nIt returns a - b.\n", "")
	want := "**Add subtracts**\n\nIt returns a - b.\n\nSuggested fix:\n\n```suggestion:-0+0\nreturn a + b\n```\n\nThe rest of the fix cannot be applied as a suggestion here:\n\n- `NOTES.md`: it adds the file\n\n```diff\n--- /dev/null\n+++ b/NOTES.md\n@@ -0,0 +1 @@\n+note\n```"
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
	big := PlannedFix{Misfits: f.Misfits, Overflow: &Overflow{Lines: 4000, Files: 3, Name: "fix-d1.patch", Attach: true}}
	if got := big.CommentBody("c", "[fix-d1.patch](/uploads/x/fix-d1.patch)"); !strings.Contains(got, "4000 changed lines in 3 files") || !strings.HasSuffix(got, "for `git am`: [fix-d1.patch](/uploads/x/fix-d1.patch)") || strings.Contains(got, "```diff") {
		t.Fatalf("attached %q", got)
	}
	if got := big.CommentBody("c", ""); !strings.Contains(got, "`fix-d1.patch` (attached when you confirm)") {
		t.Fatalf("previewed %q", got)
	}
	big.Overflow.Attach = false
	if got := big.CommentBody("c", ""); !strings.Contains(got, "too large to include in a comment (4000 changed lines in 3 files)") || strings.Contains(got, "attached") {
		t.Fatalf("counted %q", got)
	}
	if (PlannedFix{Comments: []Placed{{}}}).CommentBody("c", "") != "" {
		t.Fatal("a fix with nothing for its own comment changed it")
	}
}

// inDiff is the test change's diff, which changes calc.go and shows its
// three lines.
func inDiff(path string, start, end int) (bool, bool) {
	return path == "calc.go", path == "calc.go" && start >= 1 && end <= 3
}

// commitAll is an editor turn that writes files and commits them for a
// finding.
func commitAll(t *testing.T, spec agent.Spec, files map[string]string, finding string) {
	t.Helper()
	for name, body := range files {
		os.WriteFile(filepath.Join(spec.Dir, name), []byte(body), 0o644)
	}
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-qm", "Fix " + finding, "-m", "Vuoro-Finding: " + finding}} {
		cmd := exec.Command("git", args...)
		cmd.Dir, cmd.Env = spec.Dir, append([]string{"PATH=" + os.Getenv("PATH"), "HOME=" + spec.Dir}, spec.Editor.Env...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("git %v: %s", args, out)
		}
	}
}

// An editor started for suggestions (D59) is told to keep to existing lines,
// and its wave goes back as suggestions at the reviewed head: on each
// finding's own comment where the comment reaches, with a new file and lines
// an earlier commit changed as misfits. Delivery records each finding and
// the wave, saves the patches, and the wave cannot then be pushed.
func TestAnEditorWaveGoesBackAsSuggestions(t *testing.T) {
	m, d, ch, src := setup(t, func(call toolCaller, text string) string {
		call("propose_draft", map[string]any{"severity": "high", "title": "Add subtracts", "description": "d",
			"anchor": map[string]any{"path": "calc.go", "start_line": 3}})
		call("propose_draft", map[string]any{"severity": "low", "title": "Say what it is", "description": "d",
			"anchor": map[string]any{"path": "calc.go", "start_line": 1}})
		return "done"
	})
	d.edit = func(spec agent.Spec, text string) string {
		commitAll(t, spec, map[string]string{"calc.go": "package calc\n\nfunc Add(a, b int) int { return a + b }\n", "NOTES.md": "note\n"}, "d1")
		commitAll(t, spec, map[string]string{"calc.go": "package calc // sums\n\nfunc Add(a, b int) int { return b + a }\n"}, "d2")
		return "d1, d2: fixed"
	}
	r, err := m.Start(ch, src, []Reviewer{{Profile: "code", Tool: "fake", Model: "m1"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	wait(t, r, idle)
	m.Decide(r.ID, "d1", drafts.Kept, "41", false)
	m.Decide(r.ID, "d2", drafts.Kept, "42", false)
	who := Identity{Name: "Pat", Email: "pat@example.com"}
	ctx := context.Background()
	if _, err := m.StartEditor(ctx, r.ID, ch, who, EditorTask{Deliver: "carrier pigeon"}); err == nil {
		t.Fatal("an unknown delivery was accepted")
	}
	ed, err := m.StartEditor(ctx, r.ID, ch, who, EditorTask{Deliver: Suggestions})
	if err != nil {
		t.Fatal(err)
	}
	if ed.Deliver != Suggestions || ed.Base != ch.Diff.Head {
		t.Fatalf("editor %+v", ed)
	}
	wait(t, r, func(v RunView) bool { return idle(v) && len(v.Sessions) == 2 && v.Sessions[1].FirstComplete })
	d.mu.Lock()
	prompt := d.specs[len(d.specs)-1].SystemPrompt
	d.mu.Unlock()
	if !strings.Contains(prompt, "add, delete, or\nrename no file") {
		t.Fatalf("the editor was not told its fixes go back as suggestions: %q", prompt)
	}

	plan, err := m.PlanSuggestions(ctx, r.ID, SuggestOptions{Shows: inDiff})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Fixes) != 2 || plan.Fixes[0].Finding != "d1" || plan.Fixes[0].NoteID != "41" {
		t.Fatalf("plan %+v", plan)
	}
	d1, d2 := plan.Fixes[0], plan.Fixes[1]
	if p := d1.OnComment; p == nil || p.Start != 3 || p.End != 3 || p.Block != "```suggestion:-0+0\nfunc Add(a, b int) int { return a + b }\n```" || *p.Above != "" || p.Below != nil {
		t.Fatalf("d1 on its comment %+v", p)
	}
	if len(d1.Misfits) != 1 || d1.Misfits[0].Path != "NOTES.md" || !strings.Contains(d1.Diff, "+++ b/NOTES.md\n@@ -0,0 +1 @@\n+note") {
		t.Fatalf("d1 misfits %+v %q", d1.Misfits, d1.Diff)
	}
	// d2's first line rides on its comment; its line 3 was d1's too.
	if p := d2.OnComment; p == nil || p.Start != 1 || p.Text != "package calc // sums" {
		t.Fatalf("d2 on its comment %+v", p)
	}
	if len(d2.Misfits) != 1 || !strings.Contains(d2.Misfits[0].Reason, "an earlier fix") || !strings.Contains(d2.Diff, "+func Add(a, b int) int { return b + a }") {
		t.Fatalf("d2 misfits %+v %q", d2.Misfits, d2.Diff)
	}
	// On GitHub a suggestion rides only on its comment's own line.
	gh, err := m.PlanSuggestions(ctx, r.ID, SuggestOptions{Shows: inDiff, Span: true})
	if err != nil || gh.Fixes[0].OnComment == nil || gh.Fixes[0].OnComment.Block != "```suggestion\nfunc Add(a, b int) int { return a + b }\n```" {
		t.Fatalf("on GitHub %+v %v", gh, err)
	}

	// Delivery refuses work the person did not preview, writes through the
	// caller, and records what landed.
	if _, err := m.DeliverSuggestions(ctx, r.ID, strings.Repeat("0", 40), SuggestOptions{Shows: inDiff}, nil); err == nil {
		t.Fatal("work the person never previewed went back")
	}
	if _, err := m.DeliverSuggestions(ctx, r.ID, plan.SHA, SuggestOptions{Shows: inDiff}, func(p *SuggestionPlan) (int, error) {
		if err := m.RecordSuggested(r.ID, p.Fixes[0], []drafts.SuggestedHunk{p.Fixes[0].OnComment.Hunk("41")}); err != nil {
			return 0, err
		}
		return 1, errors.New("the forge refused d2's comment")
	}); err == nil {
		t.Fatal("a failed write delivered the wave")
	}
	if v := r.View(); v.Editor.Suggested != nil || v.Drafts[0].Suggested == nil || v.Drafts[1].Suggested != nil {
		t.Fatalf("after a partial delivery %+v %+v", v.Editor, v.Drafts)
	}
	wrote := 0
	if _, err := m.DeliverSuggestions(ctx, r.ID, plan.SHA, SuggestOptions{Shows: inDiff}, func(p *SuggestionPlan) (int, error) {
		for _, f := range p.Fixes {
			if f.Done {
				continue
			}
			wrote++
			if err := m.RecordSuggested(r.ID, f, []drafts.SuggestedHunk{f.OnComment.Hunk(f.NoteID)}); err != nil {
				return 0, err
			}
		}
		return wrote, nil
	}); err != nil || wrote != 1 {
		t.Fatalf("delivering again: %v, wrote %d", err, wrote)
	}
	v := r.View()
	if v.Editor.Suggested == nil || v.Editor.Suggested.SHA != plan.SHA || len(v.Patches) != 2 {
		t.Fatalf("the wave %+v, patches %+v", v.Editor, v.Patches)
	}
	if s := v.Drafts[0].Suggested; s == nil || s.Commit != d1.Commits[0].SHA || s.Misfits != 1 || s.Hunks[0].NoteID != "41" || s.Hunks[0].Text != "func Add(a, b int) int { return a + b }" {
		t.Fatalf("d1 suggested %+v", s)
	}
	if _, err := m.PushEditor(ctx, r.ID, src.Remote, forge.StackTarget{Branch: "feature", Head: ch.Diff.Head}, plan.SHA, nil); !errors.Is(err, ErrDelivered) {
		t.Fatalf("a wave sent as suggestions was pushed: %v", err)
	}
	if _, err := m.DeliverSuggestions(ctx, r.ID, plan.SHA, SuggestOptions{Shows: inDiff}, nil); !errors.Is(err, ErrDelivered) {
		t.Fatalf("a wave went back twice: %v", err)
	}
	// Both findings are suggested, so no wave has anything left to fix.
	if _, err := m.StartEditor(ctx, r.ID, ch, who, EditorTask{}); err == nil || !strings.Contains(err.Error(), "suggested") {
		t.Fatalf("a wave started with every finding suggested: %v", err)
	}
	if !slices.ContainsFunc(v.Patches, func(p Patch) bool { return p.SHA == plan.SHA }) {
		t.Fatal("the wave's last commit was not saved as a patch")
	}
}

// A misfit past the inline cap is never put in the comment (D59): on a
// forge that takes uploads the commit's patch is attached instead, and
// otherwise only its size is given.
func TestAMisfitPastTheCapIsAttachedOrCountedNeverInlined(t *testing.T) {
	m, d, ch, src := setup(t, func(call toolCaller, text string) string {
		call("propose_draft", map[string]any{"severity": "high", "title": "Add subtracts", "description": "d",
			"anchor": map[string]any{"path": "calc.go", "start_line": 3}})
		return "done"
	})
	d.edit = func(spec agent.Spec, text string) string {
		commitAll(t, spec, map[string]string{"BIG.md": strings.Repeat("War and Peace, again.\n", 3*maxInlineLines)}, "d1")
		return "d1: fixed"
	}
	r, err := m.Start(ch, src, []Reviewer{{Profile: "code", Tool: "fake", Model: "m1"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	wait(t, r, idle)
	m.Decide(r.ID, "d1", drafts.Kept, "41", false)
	ctx := context.Background()
	if _, err := m.StartEditor(ctx, r.ID, ch, Identity{Name: "Pat", Email: "pat@example.com"}, EditorTask{Deliver: Suggestions}); err != nil {
		t.Fatal(err)
	}
	wait(t, r, func(v RunView) bool { return idle(v) && len(v.Sessions) == 2 && v.Sessions[1].FirstComplete })
	for _, uploads := range []bool{true, false} {
		plan, err := m.PlanSuggestions(ctx, r.ID, SuggestOptions{Shows: inDiff, Uploads: uploads})
		if err != nil {
			t.Fatal(err)
		}
		f := plan.Fixes[0]
		o := f.Overflow
		if f.Diff != "" || o == nil || o.Lines != 3*maxInlineLines || o.Files != 1 || o.Name != "fix-d1.patch" || o.Attach != uploads || (o.Bytes > 0) != uploads {
			t.Fatalf("uploads %v: %+v %+v", uploads, f, o)
		}
		if body := f.CommentBody("c", ""); strings.Contains(body, "War and Peace") || len(body) > 1000 {
			t.Fatalf("the comment carried the misfit: %d bytes", len(body))
		}
		if uploads {
			raw, err := m.FixPatch(ctx, r.ID, f)
			if err != nil || !strings.HasPrefix(string(raw), "From ") || len(raw) != o.Bytes {
				t.Fatalf("patch %d bytes, %v", len(raw), err)
			}
		}
	}
}
