package app

import (
	"context"
	"errors"
	"net/http"
	"time"

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
	var err error
	if req.Prompt != "" {
		list, _ := a.prompts()
		p, ok := review.FindPrompt(list, req.Prompt)
		if !ok {
			writeError(w, errors.New("There is no prompt by that name."))
			return
		}
		if in, err = fromPrompt(p, in); err != nil {
			writeError(w, err)
			return
		}
		req.Tool, req.Model = promptTool(p, req.Tool, req.Model)
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

// fromPrompt asks p's question. A prompt's scope wins over the tab's: one
// about the whole change drops a baseline, and one about what changed since a
// review needs one. A prompt with no scope follows the person's choice.
func fromPrompt(p review.Prompt, in review.AskInput) (review.AskInput, error) {
	in.Prompt, in.Title, in.Question = p.Name, p.Title, p.Text
	switch p.Scope {
	case "change":
		in.Baseline = ""
	case "since_review":
		if in.Baseline == "" {
			return in, errors.New("This prompt asks about the changes since a review. Choose a completed Vuoro review of this change.")
		}
	}
	return in, nil
}

// promptTool is the tool and model to ask p with. A prompt's tool applies
// unless the person picked one, and its model only on the tool it names, so a
// model never reaches a tool it was not written for.
func promptTool(p review.Prompt, tool, model string) (string, string) {
	if tool == "" {
		tool = p.Tool
	}
	if model == "" && (p.Tool == "" || p.Tool == tool) {
		model = p.Model
	}
	return tool, model
}

// refreshAsk gives a conversation a fresh forge read of its change before
// the person's follow-up, so its change, discussion, and pipeline tools
// answer from now. A read that fails leaves the last one, which says when it
// was made.
func (a *App) refreshAsk(ctx context.Context, id string) {
	if a.Runs == nil {
		return
	}
	run := a.Runs.Get(id)
	if run == nil || run.Mode != review.AskMode {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if ch, err := a.open(ctx, run.WebURL); err == nil {
		_ = a.Runs.SetAskFacts(run.ID, ch)
	}
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
