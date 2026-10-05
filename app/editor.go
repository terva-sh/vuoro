package app

import (
	"context"
	"errors"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/terva-sh/vuoro/review"
)

// startEditor starts a run's editor on the findings the person kept (D39),
// once the change is still at the head the run reviewed.
func (a *App) startEditor(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	run := a.Runs.Get(r.PathValue("id"))
	if run == nil {
		writeError(w, review.ErrNoRun)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
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
	a.probeTools() // a run reloaded after a restart starts on a probed tool
	if _, err := a.Runs.StartEditor(ctx, run.ID, ch.Diff.Head, a.identity(ctx)); err != nil {
		writeError(w, err)
		return
	}
	a.getRun(w, r)
}

// getEditor answers with the editor's commits and their diff.
func (a *App) getEditor(w http.ResponseWriter, r *http.Request) {
	d, err := a.Runs.Editor(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// discardEditor ends the editor and deletes its work.
func (a *App) discardEditor(w http.ResponseWriter, r *http.Request) {
	if err := a.Runs.DiscardEditor(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	a.getRun(w, r)
}

// identity is the person's git identity, which the editor's commits carry.
func (a *App) identity(ctx context.Context) review.Identity {
	bin := "git"
	if a.Runs.Repos != nil && a.Runs.Repos.Git != "" {
		bin = a.Runs.Repos.Git
	}
	get := func(key string) string {
		out, err := exec.CommandContext(ctx, bin, "config", "--global", "--get", key).Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(out))
	}
	return review.Identity{Name: get("user.name"), Email: get("user.email")}
}
