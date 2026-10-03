package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/forge/patch"
	"github.com/terva-sh/vuoro/review/drafts"
)

// The write routes. Each is one click of the person's in the window, behind
// the guard and its per-launch token, which no agent session holds: an
// agent's only credential is its drafts bearer, good for /mcp/ alone.
func (a *App) writeRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/changes/drafts", a.createDraft)
	mux.HandleFunc("POST /api/changes/drafts/{id}/update", a.updateDraft)
	mux.HandleFunc("POST /api/changes/drafts/{id}/delete", a.deleteDraft)
	mux.HandleFunc("POST /api/changes/submit", a.submit)
	mux.HandleFunc("POST /api/changes/resolve", a.resolve)
	mux.HandleFunc("POST /api/changes/approve", a.approve)
	mux.HandleFunc("POST /api/runs/{id}/drafts/{did}/keep", a.keep)
	mux.HandleFunc("POST /api/runs/{id}/drafts/{did}/drop", a.drop)
}

// place is a line the person or an agent points at: a file, a side, and a
// line number on that side.
type place struct {
	Path string      `json:"path"`
	Side drafts.Side `json:"side"`
	Line int         `json:"line"`
}

// anchor turns a place into the forge's anchor on the change as it is now.
func anchor(ch *forge.Change, p place) (*forge.Anchor, error) {
	head := p.Side != drafts.Base
	for _, f := range ch.Files {
		if f.NewPath != p.Path && !(!head && f.OldPath == p.Path) {
			continue
		}
		old, cur, ok := patch.Line(f.Patch, head, p.Line)
		if !ok {
			return nil, fmt.Errorf("line %d of %s is not in the diff on the %s side", p.Line, p.Path, p.Side)
		}
		return &forge.Anchor{Path: f.NewPath, OldPath: f.OldPath, NewLine: cur, OldLine: old, Diff: ch.Diff}, nil
	}
	return nil, fmt.Errorf("%s is not in the change", p.Path)
}

// writer opens the change again, so a write lands on today's head, and
// returns it with the client that writes to it.
func (a *App) writer(ctx context.Context, url string) (*forge.Change, forge.Writer, error) {
	ch, err := a.open(ctx, url)
	if err != nil {
		return nil, nil, err
	}
	c, err := a.client(ctx, ch.Ref.Host, ch.Kind)
	if err != nil {
		return nil, nil, err
	}
	wr, ok := c.(forge.Writer)
	if !ok {
		return nil, nil, forge.ErrUnsupported
	}
	return ch, wr, nil
}

// reply answers a write with the change as it now stands.
func (a *App) reply(w http.ResponseWriter, r *http.Request, url string) {
	ch, err := a.open(r.Context(), url)
	if err != nil {
		writeError(w, err)
		return
	}
	if service, err := a.inbox(); err == nil {
		// Its own error/status preserves stale facts if the targeted read fails.
		_ = service.RefreshChange(r.Context(), ch.Ref)
	}
	writeJSON(w, http.StatusOK, ch)
}

func (a *App) createDraft(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL      string `json:"url"`
		Body     string `json:"body"`
		ThreadID string `json:"thread_id,omitempty"`
		Place    *place `json:"place,omitempty"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Body) == "" {
		writeError(w, errors.New("write the comment first"))
		return
	}
	ch, wr, err := a.writer(r.Context(), req.URL)
	if err != nil {
		writeError(w, err)
		return
	}
	nd := forge.NewDraft{Body: req.Body, ThreadID: req.ThreadID}
	if req.Place != nil && req.ThreadID == "" {
		if nd.Anchor, err = anchor(ch, *req.Place); err != nil {
			writeError(w, err)
			return
		}
	}
	if _, err := wr.CreateDraft(r.Context(), ch.Ref, nd); err != nil {
		writeError(w, err)
		return
	}
	a.reply(w, r, req.URL)
}

type urlBody struct {
	URL  string `json:"url"`
	Body string `json:"body,omitempty"`
}

func (a *App) updateDraft(w http.ResponseWriter, r *http.Request) {
	var req urlBody
	if !readJSON(w, r, &req) {
		return
	}
	ch, wr, err := a.writer(r.Context(), req.URL)
	if err == nil {
		err = wr.UpdateDraft(r.Context(), ch.Ref, r.PathValue("id"), req.Body)
	}
	if err != nil {
		writeError(w, err)
		return
	}
	a.reply(w, r, req.URL)
}

func (a *App) deleteDraft(w http.ResponseWriter, r *http.Request) {
	var req urlBody
	if !readJSON(w, r, &req) {
		return
	}
	ch, wr, err := a.writer(r.Context(), req.URL)
	if err == nil {
		err = wr.DeleteDraft(r.Context(), ch.Ref, r.PathValue("id"))
	}
	if err != nil {
		writeError(w, err)
		return
	}
	a.reply(w, r, req.URL)
}

func (a *App) submit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL     string `json:"url"`
		Approve bool   `json:"approve,omitempty"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	ch, wr, err := a.writer(r.Context(), req.URL)
	if err == nil {
		err = wr.Submit(r.Context(), ch.Ref)
	}
	if err == nil && req.Approve {
		err = wr.Approve(r.Context(), ch.Ref, true)
	}
	if err != nil {
		writeError(w, err)
		return
	}
	a.reply(w, r, req.URL)
}

func (a *App) resolve(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL      string `json:"url"`
		ThreadID string `json:"thread_id"`
		Resolved bool   `json:"resolved"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	ch, wr, err := a.writer(r.Context(), req.URL)
	if err == nil {
		err = wr.Resolve(r.Context(), ch.Ref, req.ThreadID, req.Resolved)
	}
	if err != nil {
		writeError(w, err)
		return
	}
	a.reply(w, r, req.URL)
}

func (a *App) approve(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL     string `json:"url"`
		Approve bool   `json:"approve"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	ch, wr, err := a.writer(r.Context(), req.URL)
	if err == nil {
		err = wr.Approve(r.Context(), ch.Ref, req.Approve)
	}
	if err != nil {
		writeError(w, err)
		return
	}
	a.reply(w, r, req.URL)
}

// keepBody is the comment an agent's draft becomes when the person keeps it
// unedited.
func keepBody(d drafts.Draft) string {
	if d.ThreadID != "" {
		return strings.TrimSpace(d.Description)
	}
	b := "**" + d.Title + "**\n\n" + strings.TrimSpace(d.Description)
	if e := strings.TrimSpace(d.Evidence); e != "" {
		f := drafts.Fence(e)
		b += "\n\n" + f + "\n" + e + "\n" + f
	}
	if s := drafts.SuggestionBlock(d); s != "" {
		b += "\n\n" + s
	}
	return b
}

// keep makes an agent's draft one of the person's pending comments on the
// forge, as they click Keep, with their edits if they made any.
func (a *App) keep(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL  string  `json:"url"`
		Body *string `json:"body,omitempty"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	runID, id := r.PathValue("id"), r.PathValue("did")
	d, head, err := a.Runs.Draft(runID, id)
	if err != nil {
		writeError(w, err)
		return
	}
	if d.Status != drafts.Pending || d.Decision != "" {
		writeError(w, fmt.Errorf("%s is no longer pending", id))
		return
	}
	if d.ThreadID != "" {
		a.keepReply(w, r, req.URL, req.Body)
		return
	}
	ch, wr, err := a.writer(r.Context(), req.URL)
	if err != nil {
		writeError(w, err)
		return
	}
	run := a.Runs.Get(runID)
	if run.Ref != ch.Ref || run.Kind != ch.Kind {
		writeError(w, errors.New("This draft belongs to another change."))
		return
	}
	body, edited := keepBody(d), false
	if req.Body != nil && *req.Body != body {
		body, edited = *req.Body, true
	}
	if strings.TrimSpace(body) == "" {
		writeError(w, errors.New("the comment is empty"))
		return
	}
	nd := forge.NewDraft{Body: body}
	if d.Anchor != nil {
		if ch.Diff.Head != head {
			writeError(w, fmt.Errorf("the change has moved since this run reviewed %.8s; run the review again before keeping an anchored finding", head))
			return
		}
		if nd.Anchor, err = anchor(ch, place{Path: d.Anchor.Path, Side: d.Anchor.Side, Line: d.Anchor.EndLine}); err != nil {
			writeError(w, err)
			return
		}
	}
	note, err := wr.CreateDraft(r.Context(), ch.Ref, nd)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := a.Runs.Decide(runID, id, drafts.Kept, note.ID, edited); err != nil {
		writeError(w, err)
		return
	}
	a.reply(w, r, req.URL)
}

func (a *App) drop(w http.ResponseWriter, r *http.Request) {
	var req urlBody
	if !readJSON(w, r, &req) {
		return
	}
	if err := a.Runs.Decide(r.PathValue("id"), r.PathValue("did"), drafts.Dropped, "", false); err != nil {
		writeError(w, err)
		return
	}
	a.reply(w, r, req.URL)
}
