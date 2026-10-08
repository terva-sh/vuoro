package review

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/review/drafts"
)

// What one comment may carry (D59). A suggestion replaces at most
// maxSuggestLines lines and writes at most as many. The plain diff of what
// cannot be a suggestion is inlined up to maxInlineLines and maxInlineBytes;
// past that it is attached, or only counted. suggestReach is how far a
// change may be from its comment's line, or from the next change, and still
// join it.
const (
	maxSuggestLines = 100
	maxInlineLines  = 100
	maxInlineBytes  = 16 << 10
	suggestReach    = 3
	maxSuggestFile  = 4 << 20 // a file read whole to map lines
)

// SuggestOptions are what the planner needs to know about the change and its
// forge. Shows reads the change's diff now: whether it changes path, and
// whether a comment can sit on lines start to end, which on GitLab means
// the diff shows line end, and on GitHub that the lines lie in one hunk.
// Span says a suggestion spans its comment's lines (GitHub) rather than
// carrying offsets (GitLab), and Uploads that a file can be attached.
type SuggestOptions struct {
	Shows   func(path string, start, end int) (changed, shown bool)
	Span    bool
	Uploads bool
}

// SuggestionPlan is an editor wave as suggestions on the person's pending
// comments (D59), as the person previews it and as delivery writes it.
type SuggestionPlan struct {
	SHA   string       `json:"sha"` // the wave's last commit
	Fixes []PlannedFix `json:"fixes"`
	// Left are the given findings no commit names.
	Left []string `json:"left,omitempty"`
}

// PlannedFix is what goes back for one finding: the suggestion that rides on
// its own pending comment, new pending comments for the rest, and what
// cannot be a suggestion.
type PlannedFix struct {
	Finding string         `json:"finding"`
	Title   string         `json:"title"`
	NoteID  string         `json:"note_id"`
	Commits []EditorCommit `json:"commits"`
	// OnComment rides on the finding's own pending comment.
	OnComment *Placed  `json:"on_comment,omitempty"`
	Comments  []Placed `json:"comments,omitempty"` // each a new pending comment
	Misfits   []Misfit `json:"misfits,omitempty"`
	// Diff is the misfits as one plain diff, within the inline cap.
	Diff string `json:"diff,omitempty"`
	// Overflow is set when the misfits are over the inline cap.
	Overflow *Overflow `json:"overflow,omitempty"`
	// Done says an earlier delivery already sent this finding's fix.
	Done bool `json:"done,omitempty"`
}

// Placed is one suggestion: lines Start to End of Path at the reviewed head
// become Text. Line is the comment's line; on a forge whose suggestion spans
// its comment, the comment runs from Start to Line. Block is the suggestion
// in the forge's syntax.
type Placed struct {
	Path  string  `json:"path"`
	Start int     `json:"start"`
	End   int     `json:"end"`
	Line  int     `json:"line"`
	Text  string  `json:"text"`
	Block string  `json:"block"`
	Above *string `json:"above,omitempty"`
	Below *string `json:"below,omitempty"`
}

// Misfit is a change that cannot be a suggestion, and why.
type Misfit struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
	diff   string
}

// Overflow is a plain diff too large to inline. Attach says the commits'
// patch, Bytes long, is attached as Name; otherwise only Lines and Files are
// given.
type Overflow struct {
	Lines  int    `json:"lines"`
	Files  int    `json:"files"`
	Bytes  int    `json:"bytes,omitempty"`
	Name   string `json:"name"`
	Attach bool   `json:"attach"`
}

// ErrDelivered means the wave already went back, pushed or as suggestions.
var ErrDelivered = errors.New("review: this wave's work already went back; start another wave for more")

// suggestable checks that the run's editor wave can go back as suggestions:
// somebody else's change, standing on the reviewed head, not yet delivered.
// r.mu is held.
func (r *Run) suggestable() (*EditorState, error) {
	e := r.editor
	switch {
	case !e.Active():
		return nil, errors.New("review: this review has no editor")
	case e.Own || e.Local || r.Mode != "":
		return nil, errors.New("review: suggestions go back on somebody else's change; on your own, the editor's fixes are pushed")
	case e.delivered():
		return nil, ErrDelivered
	case e.Base != r.Head:
		return nil, fmt.Errorf("review: this wave stands on the stacked %s's last push, not on the reviewed head, so its fixes go there", ChangeWord(r.Kind))
	}
	return e, nil
}

// PlanSuggestions reads the editor's wave as suggestions (D59). It writes
// nothing: the preview shows it, and delivery writes it.
func (m *Manager) PlanSuggestions(ctx context.Context, runID string, opt SuggestOptions) (*SuggestionPlan, error) {
	r := m.Get(runID)
	if r == nil {
		return nil, ErrNoRun
	}
	r.mu.Lock()
	e, err := r.suggestable()
	var items []string
	head := r.Head
	if e != nil {
		items = slices.Clone(e.Items)
	}
	r.mu.Unlock()
	if err != nil {
		return nil, err
	}
	detail, err := m.Editor(ctx, runID)
	if err != nil {
		return nil, err
	}
	if len(detail.Commits) == 0 {
		return nil, errors.New("review: the editor has committed nothing to send back")
	}
	gitDir, err := m.Repos.gitDir(r.repoKey)
	if err != nil {
		return nil, err
	}
	pl := &planner{
		word: ChangeWord(r.Kind), g: localGit{bin: m.Repos.Git, dir: gitDir}, head: head, opt: opt,
		earlier: map[string][][]diffHunk{}, broken: map[string]string{}, files: map[string][]string{},
	}
	v := r.View()
	byID := map[string]drafts.Draft{}
	for _, d := range v.Drafts {
		byID[d.ID] = d
	}
	plan := &SuggestionPlan{SHA: detail.Commits[len(detail.Commits)-1].SHA, Fixes: []PlannedFix{}}
	index := map[string]int{}
	named := map[string]bool{}
	for _, c := range detail.Commits {
		id := ""
		for _, f := range c.Findings {
			if slices.Contains(items, f) {
				id = f
				break
			}
		}
		if id == "" {
			return nil, fmt.Errorf("review: the commit %s names none of the findings given to the editor, so vuoro cannot tell which comment it belongs on; ask the editor to add its Vuoro-Finding trailer", short(c.SHA))
		}
		named[id] = true
		i, ok := index[id]
		if !ok {
			d := byID[id]
			i = len(plan.Fixes)
			index[id] = i
			plan.Fixes = append(plan.Fixes, PlannedFix{Finding: id, Title: d.Title, NoteID: d.NoteID, Done: d.Suggested != nil})
		}
		f := &plan.Fixes[i]
		f.Commits = append(f.Commits, c)
		if err := pl.commit(ctx, c, byID[id], f); err != nil {
			return nil, err
		}
	}
	for _, id := range items {
		if !named[id] {
			plan.Left = append(plan.Left, id)
		}
	}
	for i := range plan.Fixes {
		if err := pl.overflow(ctx, &plan.Fixes[i]); err != nil {
			return nil, err
		}
	}
	return plan, nil
}

// diffHunk is one change of a commit read with no context: Count lines from Old
// at the commit's parent became NewCount lines from New.
type diffHunk struct {
	Old, Count, New, NewCount int
	added                     []string
}

// edit is a change as a suggestion would make it: lines Start to End become
// Text.
type edit struct {
	Start, End int
	Text       []string
}

// planner turns commits into suggestions, one commit at a time, in order.
// earlier holds each file's hunks of the commits already read, so a later
// commit's lines can be mapped back to the reviewed head; broken holds files
// that cannot be mapped at all, and why.
type planner struct {
	word    string // what the forge calls a change
	g       localGit
	head    string
	opt     SuggestOptions
	earlier map[string][][]diffHunk
	broken  map[string]string
	files   map[string][]string // a file's lines at a commit, by "sha:path"
}

// lines reads a file at a commit, as lines.
func (pl *planner) lines(ctx context.Context, sha, path string) ([]string, error) {
	key := sha + ":" + path
	if l, ok := pl.files[key]; ok {
		return l, nil
	}
	out, err := pl.g.run(ctx, maxSuggestFile, "show", "--no-color", "--end-of-options", key)
	if err != nil {
		return nil, err
	}
	l := strings.Split(out, "\n")
	if len(l) > 0 && l[len(l)-1] == "" {
		l = l[:len(l)-1]
	}
	pl.files[key] = l
	return l, nil
}

// commit adds one commit's changes to the finding's fix.
func (pl *planner) commit(ctx context.Context, c EditorCommit, d drafts.Draft, f *PlannedFix) error {
	raw, err := pl.g.run(ctx, maxPatch, "diff", "-U0", "--no-color", "--no-ext-diff", "-M", "--src-prefix=a/", "--dst-prefix=b/", c.SHA+"^", c.SHA)
	if errors.Is(err, errTooLarge) {
		f.Misfits = append(f.Misfits, Misfit{Path: "", Reason: "the commit is too large to read as suggestions"})
		return nil
	}
	if err != nil {
		return err
	}
	for _, fd := range parsePatch("\n---\n" + raw) {
		path := fd.NewPath
		reason := ""
		switch {
		case fd.New:
			reason = "it adds the file"
		case fd.Deleted:
			reason = "it deletes the file"
		case fd.Renamed:
			reason = "it renames the file from " + fd.OldPath
		case fd.Omitted:
			reason = "the file is binary"
		case pl.broken[path] != "":
			reason = pl.broken[path]
		}
		hunks := parseHunks(fd.Patch)
		if reason != "" {
			f.Misfits = append(f.Misfits, Misfit{Path: path, Reason: reason, diff: fileDiff(fd)})
			pl.broken[path] = "an earlier fix in this wave " + strings.TrimPrefix(reason, "it ")
			pl.broken[fd.OldPath] = pl.broken[path]
			continue
		}
		parent, err := pl.lines(ctx, c.SHA+"^", path)
		if err != nil {
			f.Misfits = append(f.Misfits, Misfit{Path: path, Reason: "the file is too large to read as suggestions", diff: fileDiff(fd)})
			pl.broken[path] = "an earlier fix in this wave changed a file too large to read"
			continue
		}
		eds := editsOf(hunks, parent)
		for _, ed := range joinEdits(eds, parent) {
			// Edits joined through lines an earlier commit changed are
			// placed one by one instead, so the ones that fit still do.
			parts := slices.DeleteFunc(slices.Clone(eds), func(x edit) bool { return x.Start < ed.Start || x.End > ed.End })
			if _, ok := pl.back(path, ed.Start, ed.End); !ok && len(parts) > 1 {
				for _, x := range parts {
					pl.place(ctx, path, x, parent, d, f)
				}
				continue
			}
			pl.place(ctx, path, ed, parent, d, f)
		}
		pl.earlier[path] = append(pl.earlier[path], hunks)
	}
	return nil
}

// place makes one edit a suggestion, on the finding's own comment or a new
// one, or a misfit.
func (pl *planner) place(ctx context.Context, path string, ed edit, parent []string, d drafts.Draft, f *PlannedFix) {
	misfit := func(reason string) {
		f.Misfits = append(f.Misfits, Misfit{Path: path, Reason: reason, diff: editDiff(path, ed, parent)})
	}
	if ed.Start < 1 || len(ed.Text) == 0 {
		misfit("it empties the file, or fills an empty one")
		return
	}
	start, ok := pl.back(path, ed.Start, ed.End)
	if !ok {
		misfit("it changes lines an earlier fix in this wave also changed")
		return
	}
	end := start + ed.End - ed.Start
	if changed, _ := pl.opt.Shows(path, start, end); !changed {
		misfit("the " + pl.word + " does not change this file")
		return
	}
	head, err := pl.lines(ctx, pl.head, path)
	if err != nil || end > len(head) {
		misfit("the file cannot be read at the reviewed head")
		return
	}
	text := ed.Text
	if end-start+1 > maxSuggestLines || len(text) > maxSuggestLines {
		misfit(fmt.Sprintf("it is longer than a suggestion can be, %d lines", maxSuggestLines))
		return
	}
	p := Placed{Path: path, Start: start, End: end}
	// A suggestion rides on the finding's own comment when the comment can
	// reach it, and only one does. On GitHub the comment is on its last line,
	// or spans the range of the reviewer's suggestion it was kept with.
	if a := d.Anchor; f.OnComment == nil && a != nil && a.Side == drafts.Head && a.Path == path {
		l, first := a.EndLine, a.EndLine
		if d.Suggestion != nil {
			first = a.StartLine
		}
		switch {
		case pl.opt.Span && start == first && end == l:
			p.Line, p.Text = l, strings.Join(text, "\n")
			p.Block = drafts.Suggestion(p.Text, 0, 0, true)
		case !pl.opt.Span && l >= start-suggestReach && l <= end+suggestReach:
			// Reach the comment's line through unchanged head lines.
			if l < start {
				text = append(slices.Clone(head[l-1:start-1]), text...)
				p.Start = l
			}
			if l > end {
				text = append(slices.Clone(text), head[end:l]...)
				p.End = l
			}
			if p.End-p.Start+1 <= maxSuggestLines && len(text) <= maxSuggestLines {
				p.Line, p.Text = l, strings.Join(text, "\n")
				p.Block = drafts.Suggestion(p.Text, l-p.Start, p.End-l, false)
			}
		}
		if p.Block != "" {
			border(&p, head)
			f.OnComment = &p
			return
		}
		p, text = Placed{Path: path, Start: start, End: end}, ed.Text
	}
	// Otherwise it is a new comment on its last line, which the change's diff
	// must show; on GitHub the comment spans the lines, all in one hunk.
	p.Line, p.Text = end, strings.Join(text, "\n")
	if _, shown := pl.opt.Shows(path, start, end); !shown {
		misfit("the " + pl.word + "'s diff does not show its lines")
		return
	}
	p.Block = drafts.Suggestion(p.Text, end-start, 0, pl.opt.Span)
	border(&p, head)
	f.Comments = append(f.Comments, p)
}

// border records the head's lines just outside a suggestion, which tell an
// applied suggestion from the same text elsewhere.
func border(p *Placed, head []string) {
	if p.Start >= 2 {
		s := head[p.Start-2]
		p.Above = &s
	}
	if p.End < len(head) {
		s := head[p.End]
		p.Below = &s
	}
}

// back maps lines start to end of a file at the parent of the commit being
// read back to the reviewed head, through the hunks of the wave's earlier
// commits, and returns where they start. It fails when an earlier commit
// changed any of those lines, or removed lines between them.
func (pl *planner) back(path string, start, end int) (int, bool) {
	for i := len(pl.earlier[path]) - 1; i >= 0; i-- {
		hs := pl.earlier[path][i]
		delta := 0
		for _, h := range hs {
			last := h.New + h.NewCount - 1
			switch {
			case h.NewCount > 0 && h.New <= end && last >= start,
				h.NewCount == 0 && h.New >= start && h.New < end: // removed between them
				return 0, false
			case h.NewCount == 0 && h.New < start: // removed before them
				delta += h.Count
			case h.NewCount > 0 && last < start:
				delta += h.Count - h.NewCount
			}
		}
		start += delta
		end += delta
	}
	return start, true
}

// parseHunks reads the hunks of a diff with no context.
func parseHunks(p string) []diffHunk {
	var out []diffHunk
	for _, l := range strings.Split(p, "\n") {
		if rest, ok := strings.CutPrefix(l, "@@ -"); ok {
			ranges, _, _ := strings.Cut(rest, " @@")
			a, b, _ := strings.Cut(ranges, " +")
			h := diffHunk{}
			h.Old, h.Count = hunkSpan(a)
			h.New, h.NewCount = hunkSpan(b)
			out = append(out, h)
			continue
		}
		if len(out) == 0 || l == "" {
			continue
		}
		if h := &out[len(out)-1]; l[0] == '+' {
			h.added = append(h.added, l[1:])
		}
	}
	return out
}

// hunkSpan reads "start,count" from a hunk header, where a missing count is 1.
func hunkSpan(s string) (int, int) {
	a, b, ok := strings.Cut(s, ",")
	start, _ := strconv.Atoi(a)
	if !ok {
		return start, 1
	}
	n, _ := strconv.Atoi(b)
	return start, n
}

// editsOf turns hunks into edits that replace at least one line, since a
// suggestion replaces lines: lines only added take the line before them, or
// the first line, and lines only removed take the line before them, or the
// line after.
func editsOf(hs []diffHunk, parent []string) []edit {
	var out []edit
	for _, h := range hs {
		switch {
		case h.Count > 0 && h.NewCount > 0:
			out = append(out, edit{h.Old, h.Old + h.Count - 1, h.added})
		case h.Count == 0 && h.Old >= 1 && h.Old <= len(parent):
			out = append(out, edit{h.Old, h.Old, append([]string{parent[h.Old-1]}, h.added...)})
		case h.Count == 0 && len(parent) > 0:
			out = append(out, edit{1, 1, append(slices.Clone(h.added), parent[0])})
		case h.NewCount == 0 && h.Old >= 2:
			out = append(out, edit{h.Old - 1, h.Old + h.Count - 1, []string{parent[h.Old-2]}})
		case h.NewCount == 0 && h.Old+h.Count <= len(parent):
			out = append(out, edit{h.Old, h.Old + h.Count, []string{parent[h.Old+h.Count-1]}})
		default: // the whole file emptied, or an empty file filled
			out = append(out, edit{h.Old, h.Old + max(h.Count, 1) - 1, h.added})
		}
	}
	return out
}

// joinEdits joins edits a few lines apart into one, through the unchanged
// lines between them.
func joinEdits(eds []edit, parent []string) []edit {
	var out []edit
	for _, ed := range eds {
		if n := len(out); n > 0 && ed.Start-out[n-1].End-1 <= suggestReach {
			last := &out[n-1]
			gap := []string{}
			if ed.Start > last.End+1 {
				gap = parent[last.End : ed.Start-1]
			}
			last.Text = append(append(slices.Clone(last.Text), gap...), ed.Text...)
			last.End = max(last.End, ed.End)
			continue
		}
		out = append(out, ed)
	}
	return out
}

// editDiff is an edit as a plain diff, for a comment.
func editDiff(path string, ed edit, parent []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "--- a/%s\n+++ b/%s\n@@ -%d,%d +%d,%d @@\n", path, path, ed.Start, ed.End-ed.Start+1, ed.Start, len(ed.Text))
	for i := ed.Start; i <= ed.End && i <= len(parent); i++ {
		b.WriteString("-" + parent[i-1] + "\n")
	}
	for _, l := range ed.Text {
		b.WriteString("+" + l + "\n")
	}
	return b.String()
}

// fileDiff is a whole file's change as a plain diff, for a comment.
func fileDiff(fd forge.FileDiff) string {
	from, to := "a/"+fd.OldPath, "b/"+fd.NewPath
	if fd.New {
		from = "/dev/null"
	}
	if fd.Deleted {
		to = "/dev/null"
	}
	head := fmt.Sprintf("--- %s\n+++ %s\n", from, to)
	if fd.Omitted {
		return ""
	}
	return head + fd.Patch
}

// overflow joins a fix's misfits into one plain diff, or, over the inline
// cap, counts them and sizes the patch that would be attached instead.
func (pl *planner) overflow(ctx context.Context, f *PlannedFix) error {
	var b strings.Builder
	n, files := 0, map[string]bool{}
	for _, m := range f.Misfits {
		b.WriteString(m.diff)
		files[m.Path] = true
		for _, l := range strings.Split(m.diff, "\n") {
			if (strings.HasPrefix(l, "+") || strings.HasPrefix(l, "-")) && !strings.HasPrefix(l, "+++") && !strings.HasPrefix(l, "---") {
				n++
			}
		}
	}
	d := strings.TrimRight(b.String(), "\n")
	if d == "" {
		return nil
	}
	if strings.Count(d, "\n")+1 <= maxInlineLines && len(d) <= maxInlineBytes {
		f.Diff = d
		return nil
	}
	o := &Overflow{Lines: n, Files: len(files), Name: "fix-" + f.Finding + ".patch"}
	if pl.opt.Uploads {
		raw, err := pl.patch(ctx, f.Commits)
		switch {
		case errors.Is(err, errTooLarge):
		case err != nil:
			return err
		default:
			o.Bytes, o.Attach = len(raw), true
		}
	}
	f.Overflow = o
	return nil
}

// patch is the commits as one patch for git am, under maxPatch.
func (pl *planner) patch(ctx context.Context, commits []EditorCommit) (string, error) {
	var b strings.Builder
	for _, c := range commits {
		raw, err := pl.g.run(ctx, maxPatch, "format-patch", "--stdout", "-1", "--no-signature", "--no-color", "-M", "--end-of-options", c.SHA)
		if err != nil {
			return "", err
		}
		if b.Len()+len(raw) > maxPatch {
			return "", errTooLarge
		}
		b.WriteString(raw)
	}
	return b.String(), nil
}

// FixPatch is a planned fix's commits as one patch, for the comment to attach.
func (m *Manager) FixPatch(ctx context.Context, runID string, f PlannedFix) ([]byte, error) {
	r := m.Get(runID)
	if r == nil {
		return nil, ErrNoRun
	}
	gitDir, err := m.Repos.gitDir(r.repoKey)
	if err != nil {
		return nil, err
	}
	raw, err := (&planner{g: localGit{bin: m.Repos.Git, dir: gitDir}}).patch(ctx, f.Commits)
	return []byte(raw), err
}

// CommentBody is the finding's pending comment once its fix is added: the
// comment as it stands, then the suggestion that rides on it, and what
// cannot be a suggestion. link is the attached patch's Markdown, empty
// before it is uploaded. It is empty when nothing is added.
func (f PlannedFix) CommentBody(existing, link string) string {
	var parts []string
	if f.OnComment != nil {
		parts = append(parts, "Suggested fix:\n\n"+f.OnComment.Block)
	}
	if len(f.Misfits) > 0 {
		var why []string
		for _, m := range f.Misfits {
			w := m.Reason
			if m.Path != "" {
				w = "`" + m.Path + "`: " + w
			}
			if !slices.Contains(why, w) {
				why = append(why, w)
			}
		}
		rest := "The fix"
		if f.OnComment != nil || len(f.Comments) > 0 {
			rest = "The rest of the fix"
		}
		reasons := "- " + strings.Join(why, "\n- ")
		switch o := f.Overflow; {
		case f.Diff != "":
			fence := drafts.Fence(f.Diff)
			parts = append(parts, fmt.Sprintf("%s cannot be applied as a suggestion here:\n\n%s\n\n%sdiff\n%s\n%s", rest, reasons, fence, f.Diff, fence))
		case o != nil && o.Attach:
			if link == "" {
				link = "`" + o.Name + "` (attached when you confirm)"
			}
			parts = append(parts, fmt.Sprintf("%s cannot be applied as a suggestion here, and is too large to show (%s):\n\n%s\n\nIt is attached as a patch, for `git am`: %s", rest, size(o), reasons, link))
		case o != nil:
			parts = append(parts, fmt.Sprintf("%s cannot be applied as a suggestion here, and is too large to include in a comment (%s):\n\n%s", rest, size(o), reasons))
		default:
			parts = append(parts, fmt.Sprintf("%s cannot be applied as a suggestion here:\n\n%s", rest, reasons))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimRight(existing, "\n") + "\n\n" + strings.Join(parts, "\n\n")
}

func size(o *Overflow) string {
	return fmt.Sprintf("%d changed line%s in %d file%s", o.Lines, plural(o.Lines), o.Files, plural(o.Files))
}

// CommentBody is a new pending comment carrying part of a finding's fix.
func (p Placed) CommentBody(title string) string {
	return fmt.Sprintf("Part of the fix for **%s**:\n\n%s", title, p.Block)
}

// Hunk is the suggestion as the finding records it (D59).
func (p Placed) Hunk(noteID string) drafts.SuggestedHunk {
	return drafts.SuggestedHunk{Path: p.Path, Start: p.Start, End: p.End, Text: p.Text, Above: p.Above, Below: p.Below, NoteID: noteID}
}

// RecordSuggested marks a finding's fix as sent back, with its suggestions
// and the comments that carry them.
func (m *Manager) RecordSuggested(runID string, f PlannedFix, hunks []drafts.SuggestedHunk) error {
	r := m.Get(runID)
	if r == nil {
		return ErrNoRun
	}
	last := f.Commits[len(f.Commits)-1]
	err := r.store.MarkSuggested(f.Finding, drafts.SuggestedFix{Commit: last.SHA, Subject: last.Subject, At: time.Now().UTC(), Hunks: hunks, Misfits: len(f.Misfits)})
	if err == nil {
		r.save()
	}
	return err
}

// DeliverSuggestions sends the editor's wave back as suggestions (D59),
// called only from the person's confirmed action. sha is the wave's last
// commit as the person saw it in the preview, and anything newer is
// refused, so what goes back is what they read. write makes the forge's
// writes for the plan, recording each finding's fix as it lands, and
// returns how many comments it wrote. The wave is delivered once all of
// them landed; a failure leaves it to deliver again, which skips the
// findings already sent.
func (m *Manager) DeliverSuggestions(ctx context.Context, runID, sha string, opt SuggestOptions, write func(*SuggestionPlan) (int, error)) (*SuggestionPlan, error) {
	r := m.Get(runID)
	if r == nil {
		return nil, ErrNoRun
	}
	r.mu.Lock()
	e, err := r.suggestable()
	if err == nil && e.pushing {
		err = ErrPushing
	}
	if err == nil {
		e.pushing = true
	}
	r.mu.Unlock()
	if err != nil {
		return nil, err
	}
	defer func() {
		r.mu.Lock()
		e.pushing = false
		r.mu.Unlock()
	}()
	detail, err := m.Editor(ctx, runID)
	if err != nil {
		return nil, err
	}
	if n := len(detail.Commits); n == 0 || detail.Commits[n-1].SHA != sha {
		return nil, errors.New("review: the editor's work changed since you previewed it; preview it again")
	}
	if err := m.askUnnamed(r, e, detail, e.Base); err != nil {
		return nil, err
	}
	plan, err := m.PlanSuggestions(ctx, runID, opt)
	if err != nil {
		return nil, err
	}
	if plan.SHA != sha {
		return nil, errors.New("review: the editor's work changed since you previewed it; preview it again")
	}
	n, err := write(plan)
	if err != nil {
		return plan, err
	}
	return plan, m.FinishSuggestions(ctx, runID, sha, n)
}

// FinishSuggestions records the wave as delivered as suggestions, read at
// sha, and saves its commits as patches (D51), since nothing else keeps
// them once the run's editor repository goes.
func (m *Manager) FinishSuggestions(ctx context.Context, runID, sha string, comments int) error {
	r := m.Get(runID)
	if r == nil {
		return ErrNoRun
	}
	detail, err := m.Editor(ctx, runID)
	if err != nil {
		return err
	}
	r.mu.Lock()
	e, err := r.suggestable()
	if err == nil {
		e.Suggested = &Suggested{SHA: sha, At: time.Now().UTC(), Comments: comments}
		e.Commits = detail.Commits
	}
	r.mu.Unlock()
	if err != nil {
		return err
	}
	m.savePatches(ctx, r, detail.Commits, "editor")
	r.save()
	return nil
}

// landSuggestions records, for each suggested finding not yet fixed, the
// oldest commit since the review after which every one of its suggestions
// is in place, between the lines that bordered it (D59). src is the change
// as the forge reports it now; nothing is read when it has not moved, or
// when no finding waits on a suggestion. It is best effort: a read that
// fails leaves the findings suggested.
func (m *Manager) landSuggestions(ctx context.Context, r *Run, src Source) {
	r.mu.Lock()
	head := r.Head
	r.mu.Unlock()
	if src.Head == "" || src.Head == head {
		return
	}
	var waiting []drafts.Draft
	for _, d := range r.View().Drafts {
		if d.Suggested != nil && d.FixedIn == "" && len(d.Suggested.Hunks) > 0 {
			waiting = append(waiting, d)
		}
	}
	if len(waiting) == 0 {
		return
	}
	log, err := m.Repos.commitsSince(ctx, src, head)
	if err != nil {
		return
	}
	gitDir, err := m.Repos.gitDir(r.repoKey)
	if err != nil {
		return
	}
	var commits []EditorCommit
	for _, rec := range strings.Split(log, "\x1e") {
		if f := strings.SplitN(strings.TrimSpace(rec), "\x00", 2); len(f) == 2 && f[0] != "" {
			commits = append(commits, EditorCommit{SHA: f[0], Subject: f[1]})
		}
	}
	pl := &planner{g: localGit{bin: m.Repos.Git, dir: gitDir}, files: map[string][]string{}}
	applied := func(sha string, s *drafts.SuggestedFix) bool {
		for _, h := range s.Hunks {
			lines, err := pl.lines(ctx, sha, h.Path)
			if err != nil || !inPlace(lines, h) {
				return false
			}
		}
		return true
	}
	var cited []EditorCommit
	for _, d := range waiting {
		if !applied(src.Head, d.Suggested) {
			continue
		}
		for _, c := range commits {
			if applied(c.SHA, d.Suggested) {
				if r.store.CiteFix(d.ID, c.SHA, c.Subject, "suggestion", "") == nil {
					cited = append(cited, c)
				}
				break
			}
		}
	}
	if len(cited) > 0 {
		m.savePatches(ctx, r, cited, "suggestion")
		r.save()
	}
}

// inPlace says a suggestion's replacement is in a file, between the lines
// that bordered it at the reviewed head: at the file's start when nothing
// was above it, and at its end when nothing was below.
func inPlace(lines []string, h drafts.SuggestedHunk) bool {
	want := strings.Split(h.Text, "\n")
	if h.Above != nil {
		want = append([]string{*h.Above}, want...)
	}
	if h.Below != nil {
		want = append(want, *h.Below)
	}
	for i := 0; i+len(want) <= len(lines); i++ {
		if h.Above == nil && i > 0 {
			break
		}
		if slices.Equal(lines[i:i+len(want)], want) && (h.Below != nil || i+len(want) == len(lines)) {
			return true
		}
	}
	return false
}
