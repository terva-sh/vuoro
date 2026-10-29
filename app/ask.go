package app

import (
	"errors"
	"net/http"

	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/forge/adapters"
	"github.com/terva-sh/vuoro/review"
)

// askRunRoutes are the generic run routes a conversation answers: it is
// read, talked to, compared with the change's head, and closed, and none of
// a review's routes act on it.
var askRunRoutes = map[string]bool{"": true, "close": true, "sessions": true, "head": true}

// askRoutes are the routes of asking about a change (section 6.11, D68).
func (a *App) askRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/asks", a.startAsk)
	mux.HandleFunc("POST /api/changes/asks", a.listAsks)
}

// startAsk starts a conversation about a change, from a canned prompt by
// name or from the person's own question.
func (a *App) startAsk(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL      string `json:"url"`
		Prompt   string `json:"prompt,omitempty"`
		Question string `json:"question,omitempty"`
		Tool     string `json:"tool,omitempty"`
		Model    string `json:"model,omitempty"`
		ReviewID string `json:"review_id,omitempty"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if a.Runs == nil {
		writeError(w, errors.New("Asking about a change is unavailable."))
		return
	}
	in := review.AskInput{Question: req.Question, Baseline: req.ReviewID}
	if req.Prompt != "" {
		list, _ := a.prompts()
		p, ok := review.FindPrompt(list, req.Prompt)
		if !ok {
			writeError(w, errors.New("There is no prompt by that name."))
			return
		}
		in.Prompt, in.Title, in.Question = p.Name, p.Title, p.Text
		if req.Tool == "" {
			req.Tool = p.Tool
		}
		if req.Model == "" {
			req.Model = p.Model
		}
	}
	ch, err := a.open(r.Context(), req.URL)
	if err != nil {
		writeError(w, err)
		return
	}
	src, err := a.source(ch)
	if err != nil {
		writeError(w, err)
		return
	}
	a.probeTools()
	run, err := a.Runs.StartAsk(ch, src, review.Reviewer{Tool: req.Tool, Model: req.Model}, in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, run.View())
}

// listAsks answers the conversations about the change at url, newest first,
// with the summaries made before Ask listed as explain. It reads no forge.
func (a *App) listAsks(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	out := []review.RunView{}
	if a.Runs == nil {
		writeJSON(w, http.StatusOK, out)
		return
	}
	u, err := forge.ChangeURL(req.URL)
	if err != nil {
		writeError(w, err)
		return
	}
	kind := ""
	if f := a.forgeFor(u.Host); f != nil {
		kind = f.Kind
	}
	ref, kind, err := adapters.ParseURL(req.URL, kind)
	if err != nil {
		writeError(w, err)
		return
	}
	for _, v := range a.Runs.AsksOf(kind, ref) {
		for i := range v.Sessions {
			v.Sessions[i].Log = nil // the list is a summary; the run has the log
		}
		out = append(out, v)
	}
	writeJSON(w, http.StatusOK, out)
}
