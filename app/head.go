package app

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/terva-sh/vuoro/review"
)

func (a *App) compareHead(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if a.Runs == nil {
		writeError(w, review.ErrNoRun)
		return
	}
	run := a.Runs.Get(r.PathValue("id"))
	if run == nil {
		writeError(w, review.ErrNoRun)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	ch, err := a.open(ctx, req.URL)
	if err != nil {
		writeError(w, err)
		return
	}
	if run.Ref != ch.Ref || run.Kind != ch.Kind {
		writeError(w, errors.New("This run belongs to another change."))
		return
	}
	if run.Head == ch.Diff.Head {
		writeJSON(w, http.StatusOK, review.HeadState{Reviewed: run.Head, Current: ch.Diff.Head, Relation: "same"})
		return
	}
	src, err := a.source(ch)
	if err != nil {
		writeJSON(w, http.StatusOK, review.HeadState{Reviewed: run.Head, Current: ch.Diff.Head, Relation: "unavailable"})
		return
	}
	status, err := a.Runs.CompareHead(ctx, run.ID, ch, src)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}
