package app

import (
	"errors"
	"net/http"

	"github.com/terva-sh/vuoro/review"
)

func (a *App) startSummary(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL      string `json:"url"`
		Tool     string `json:"tool,omitempty"`
		Model    string `json:"model,omitempty"`
		ReviewID string `json:"review_id,omitempty"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if a.Runs == nil {
		writeError(w, errors.New("Summaries are unavailable."))
		return
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
	run, err := a.Runs.StartSummary(ch, src, review.Reviewer{Tool: req.Tool, Model: req.Model}, req.ReviewID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, run.View())
}
