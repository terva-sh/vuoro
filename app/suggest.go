package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/forge/patch"
	"github.com/terva-sh/vuoro/review"
	"github.com/terva-sh/vuoro/review/drafts"
)

// suggestionPreview is what the person reads before an editor wave goes back
// as suggestions (D59): each finding's comment as it will read, and each new
// comment.
type suggestionPreview struct {
	SHA   string       `json:"sha"`
	Fixes []fixPreview `json:"fixes"`
	Left  []string     `json:"left,omitempty"`
}

// fixPreview is one finding's part. Body is its pending comment once the fix
// is added, empty when the comment is unchanged, and Before the comment as
// it stands. Missing says the comment is gone from the forge, so a new one
// is written in its place.
type fixPreview struct {
	review.PlannedFix
	Before  string       `json:"before,omitempty"`
	Body    string       `json:"body,omitempty"`
	Missing bool         `json:"missing,omitempty"`
	New     []newComment `json:"new"`
}

// newComment is a pending comment a suggestion adds, on lines Start to Line.
type newComment struct {
	Path  string `json:"path"`
	Start int    `json:"start"`
	Line  int    `json:"line"`
	Body  string `json:"body"`
}

// suggester is what both suggestion routes read first: the change now, its
// writer, its uploader where the forge has one, and the plan.
type suggester struct {
	ch     *forge.Change
	wr     forge.Writer
	up     forge.Uploader
	opt    review.SuggestOptions
	drafts map[string]drafts.Draft
	pend   map[string]string // the person's pending comments on the forge, by id
}

// suggester reads the change and checks that the run's editor wave can go
// back as suggestions on it.
func (a *App) suggester(ctx context.Context, runID, url string) (*suggester, error) {
	run := a.Runs.Get(runID)
	if run == nil {
		return nil, review.ErrNoRun
	}
	if run.Kind == review.Local {
		return nil, errors.New("A local range has no comments to suggest on; its fixes are handed back as a branch.")
	}
	ch, wr, err := a.writer(ctx, url)
	if err != nil {
		return nil, err
	}
	switch {
	case run.Ref != ch.Ref || run.Kind != ch.Kind:
		return nil, errors.New("This run belongs to another change.")
	case a.isAuthor(ch):
		return nil, errors.New("This is your own change, so the editor's fixes are pushed to it, not suggested.")
	case ch.Diff.Head != run.View().Head:
		return nil, fmt.Errorf("The %s has moved since this run reviewed it, so suggestions would land on the wrong lines; review it again.", review.ChangeWord(ch.Kind))
	}
	if err := allowed(ch.Capabilities.Suggestions && ch.Capabilities.Drafts, "suggestions"); err != nil {
		return nil, err
	}
	s := &suggester{ch: ch, wr: wr, drafts: map[string]drafts.Draft{}, pend: map[string]string{}}
	s.up, _ = wr.(forge.Uploader)
	s.opt = review.SuggestOptions{Shows: shows(ch), Span: ch.Capabilities.SpanSuggestions, Uploads: s.up != nil}
	for _, d := range run.View().Drafts {
		s.drafts[d.ID] = d
	}
	for _, d := range ch.Drafts {
		s.pend[d.ID] = d.Body
	}
	return s, nil
}

// shows reads the change's diff for the planner: whether the change changes
// a file, and whether a comment can sit on its lines start to end: on its
// last line, which the diff must show, or, where a suggestion spans its
// comment, on all of them, inside one hunk.
func shows(ch *forge.Change) func(path string, start, end int) (bool, bool) {
	return func(path string, start, end int) (bool, bool) {
		for _, f := range ch.Files {
			if f.NewPath != path || f.Deleted {
				continue
			}
			if ch.Capabilities.SpanSuggestions {
				return true, patch.Span(f.Patch, start, end)
			}
			_, _, ok := patch.Line(f.Patch, true, end)
			return true, ok
		}
		return false, false
	}
}

// preview is a fix as the person reads it before it goes back.
func (s *suggester) preview(f review.PlannedFix) fixPreview {
	p := fixPreview{PlannedFix: f, New: []newComment{}}
	before, ok := s.pend[f.NoteID]
	if !ok {
		before, p.Missing = keepBody(s.drafts[f.Finding], s.ch.Capabilities), true
	}
	p.Before, p.Body = before, f.CommentBody(before, "")
	for _, c := range f.Comments {
		start := c.Line
		if s.opt.Span {
			start = c.Start
		}
		p.New = append(p.New, newComment{Path: c.Path, Start: start, Line: c.Line, Body: c.CommentBody(f.Title)})
	}
	return p
}

// previewSuggestions shows how the editor's wave would go back as
// suggestions. It writes nothing.
func (a *App) previewSuggestions(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	s, err := a.suggester(ctx, r.PathValue("id"), req.URL)
	if err != nil {
		writeError(w, err)
		return
	}
	plan, err := a.Runs.PlanSuggestions(ctx, r.PathValue("id"), s.opt)
	if err != nil {
		writeError(w, err)
		return
	}
	out := suggestionPreview{SHA: plan.SHA, Left: plan.Left, Fixes: []fixPreview{}}
	for _, f := range plan.Fixes {
		out.Fixes = append(out.Fixes, s.preview(f))
	}
	writeJSON(w, http.StatusOK, out)
}

// suggestEditor sends the editor's wave back as suggestions on the person's
// pending comments (D59). It is the person's confirmed action and the only
// route that does: the request must say confirm "suggest", and name the
// commit the person saw. It writes pending comments, and attaches a patch
// where one is too large to show; it pushes nothing and submits nothing.
func (a *App) suggestEditor(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL     string `json:"url"`
		Confirm string `json:"confirm"`
		SHA     string `json:"sha"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if req.Confirm != "suggest" {
		writeError(w, errors.New("Sending suggestions needs your confirmation."))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	runID := r.PathValue("id")
	s, err := a.suggester(ctx, runID, req.URL)
	if err != nil {
		writeError(w, err)
		return
	}
	_, err = a.Runs.DeliverSuggestions(ctx, runID, req.SHA, s.opt, func(plan *review.SuggestionPlan) (int, error) {
		n := 0
		var failed []string
		for _, f := range plan.Fixes {
			if f.Done {
				continue
			}
			wrote, err := a.writeFix(ctx, runID, s, f)
			n += wrote
			if err != nil {
				failed = append(failed, fmt.Sprintf("%s: %v", f.Finding, err))
			}
		}
		if len(failed) > 0 {
			return n, fmt.Errorf("Some suggestions did not go back, so the wave is not finished; send it again to retry them. %s", strings.Join(failed, "; "))
		}
		return n, nil
	})
	if err != nil {
		writeError(w, err)
		return
	}
	a.getRun(w, r)
}

// writeFix writes one finding's suggestions: its new comments first, then its
// own comment, so a failure leaves the finding to send again. It records the
// finding once all of them landed, and returns how many comments it wrote.
func (a *App) writeFix(ctx context.Context, runID string, s *suggester, f review.PlannedFix) (int, error) {
	p := s.preview(f)
	n := 0
	var hunks []drafts.SuggestedHunk
	for i, c := range f.Comments {
		at, err := anchor(s.ch, place{Path: c.Path, Side: drafts.Head, Line: c.Line})
		if err != nil {
			return n, err
		}
		if s.opt.Span && c.Start < c.Line {
			at.StartLine = c.Start
		}
		note, err := s.wr.CreateDraft(ctx, s.ch.Ref, forge.NewDraft{Body: p.New[i].Body, Anchor: at})
		if err != nil {
			return n, err
		}
		n++
		hunks = append(hunks, c.Hunk(note.ID))
	}
	link := ""
	if o := f.Overflow; o != nil && o.Attach {
		raw, err := a.Runs.FixPatch(ctx, runID, f)
		if err != nil {
			return n, err
		}
		if link, err = s.up.Upload(ctx, s.ch.Ref, o.Name, raw); err != nil {
			return n, err
		}
	}
	note := f.NoteID
	if body := f.CommentBody(p.Before, link); body != "" {
		if p.Missing {
			d := s.drafts[f.Finding]
			nd := forge.NewDraft{Body: body}
			if d.Anchor != nil {
				at, err := anchor(s.ch, place{Path: d.Anchor.Path, Side: d.Anchor.Side, Line: d.Anchor.EndLine})
				if err != nil {
					return n, err
				}
				nd.Anchor = at
			}
			created, err := s.wr.CreateDraft(ctx, s.ch.Ref, nd)
			if err != nil {
				return n, err
			}
			note = created.ID
		} else if err := s.wr.UpdateDraft(ctx, s.ch.Ref, f.NoteID, body); err != nil {
			return n, err
		}
		n++
	}
	if f.OnComment != nil {
		hunks = append(hunks, f.OnComment.Hunk(note))
	}
	return n, a.Runs.RecordSuggested(runID, f, hunks)
}
