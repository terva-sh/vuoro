package app

import (
	"errors"
	"net/http"
	"strings"

	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/review"
	"github.com/terva-sh/vuoro/review/drafts"
)

func (a *App) startTriage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL   string `json:"url"`
		Tool  string `json:"tool,omitempty"`
		Model string `json:"model,omitempty"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if a.Runs == nil {
		writeError(w, errors.New("Agent triage is unavailable."))
		return
	}
	ch, err := a.open(r.Context(), req.URL)
	if err != nil {
		writeError(w, err)
		return
	}
	f := a.forgeFor(ch.Ref.Host)
	if f == nil || f.Username == "" || !strings.EqualFold(f.Username, ch.Author.Username) {
		writeError(w, errors.New("Thread triage is for a change authored by your configured account."))
		return
	}
	a.probeTools()
	src, err := a.source(ch)
	if err != nil {
		writeError(w, err)
		return
	}
	run, err := a.Runs.StartTriage(ch, src, review.Reviewer{Tool: req.Tool, Model: req.Model})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, run.View())
}

func (a *App) editReply(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Body     string          `json:"body"`
		Category drafts.Category `json:"category"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if a.Runs == nil {
		writeError(w, review.ErrNoRun)
		return
	}
	if err := a.Runs.EditReply(r.PathValue("id"), r.PathValue("did"), req.Body, req.Category); err != nil {
		writeError(w, err)
		return
	}
	a.getRun(w, r)
}
func (a *App) discardReply(w http.ResponseWriter, r *http.Request) {
	if a.Runs == nil {
		writeError(w, review.ErrNoRun)
		return
	}
	d, _, err := a.Runs.Draft(r.PathValue("id"), r.PathValue("did"))
	if err != nil || d.ThreadID == "" {
		writeError(w, review.ErrNoRun)
		return
	}
	if err := a.Runs.Decide(r.PathValue("id"), d.ID, drafts.Dropped, "", false); err != nil {
		writeError(w, err)
		return
	}
	a.getRun(w, r)
}

func (a *App) keepReply(w http.ResponseWriter, r *http.Request, url string, body *string) {
	runID, id := r.PathValue("id"), r.PathValue("did")
	err := a.Runs.AcceptReply(runID, id, func(d drafts.Draft) (string, bool, error) {
		ch, wr, err := a.writer(r.Context(), url)
		if err != nil {
			return "", false, err
		}
		f := a.forgeFor(ch.Ref.Host)
		if f == nil || !strings.EqualFold(f.Username, ch.Author.Username) {
			return "", false, errors.New("This reply is for your own change.")
		}
		if err := a.Runs.CheckReply(runID, d, ch); err != nil {
			return "", false, err
		}
		text, edited := keepBody(d), false
		if body != nil && *body != text {
			text, edited = *body, true
		}
		if strings.TrimSpace(text) == "" {
			return "", false, errors.New("Write a reply before adding it to pending comments.")
		}
		if len(text) > 20000 {
			return "", false, errors.New("A reply must be at most 20000 bytes.")
		}
		note, err := wr.CreateDraft(r.Context(), ch.Ref, forge.NewDraft{Body: text, ThreadID: d.ThreadID})
		return note.ID, edited, err
	})
	if err != nil {
		writeError(w, err)
		return
	}
	a.reply(w, r, url)
}
