package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/terva-sh/vuoro/forge"
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

// stackPreview is what the confirm dialog shows before a push: where the
// branch goes, what it changes, and the title and description to edit.
type stackPreview struct {
	Branch      string   `json:"branch"`
	Target      string   `json:"target"`
	SHA         string   `json:"sha"`
	Commits     int      `json:"commits"`
	Files       []string `json:"files"`
	CI          []string `json:"ci,omitempty"` // files that change what CI runs
	Moved       bool     `json:"moved,omitempty"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Opened      string   `json:"opened,omitempty"` // the draft change already open
}

// stack reads where the editor's work would go, for the person to confirm.
// It writes nothing.
func (a *App) stack(ctx context.Context, runID, url string) (*forge.Change, forge.Stacker, stackPreview, error) {
	var p stackPreview
	run := a.Runs.Get(runID)
	if run == nil {
		return nil, nil, p, review.ErrNoRun
	}
	ch, err := a.open(ctx, url)
	if err != nil {
		return nil, nil, p, err
	}
	if run.Ref != ch.Ref || run.Kind != ch.Kind {
		return nil, nil, p, errors.New("This run belongs to another change.")
	}
	c, err := a.client(ctx, ch.Ref.Host, ch.Kind)
	if err != nil {
		return nil, nil, p, err
	}
	st, ok := c.(forge.Stacker)
	if !ok {
		return nil, nil, p, fmt.Errorf("%w: vuoro opens a stacked merge request on GitLab only, so far", forge.ErrUnsupported)
	}
	target, err := st.StackTarget(ctx, ch.Ref)
	if err != nil {
		return nil, nil, p, err
	}
	d, err := a.Runs.Editor(ctx, runID)
	if err != nil {
		return nil, nil, p, err
	}
	if len(d.Commits) == 0 {
		return nil, nil, p, errors.New("The editor has committed nothing to push.")
	}
	v := run.View()
	p = stackPreview{Branch: d.Branch, Target: target.Branch, SHA: d.Commits[len(d.Commits)-1].SHA, Commits: len(d.Commits),
		Moved: target.Head != d.Base, Title: review.StackTitle(v),
		Description: review.StackDescription(v, d, a.identity(ctx).Name), Files: []string{}}
	for _, f := range d.Files {
		p.Files = append(p.Files, f.NewPath)
		if ciFile(f.NewPath) || ciFile(f.OldPath) {
			p.CI = append(p.CI, f.NewPath)
		}
	}
	if d.Pushed != nil {
		p.Opened = d.Pushed.URL
	}
	return ch, st, p, nil
}

// ciFile says a path configures GitLab CI, which a pushed branch runs.
func ciFile(path string) bool {
	return strings.HasSuffix(path, ".gitlab-ci.yml") || strings.HasPrefix(path, ".gitlab/")
}

func (a *App) previewStack(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	_, _, p, err := a.stack(ctx, r.PathValue("id"), req.URL)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// pushEditor pushes the editor's commits and opens the stacked draft merge
// request. It is the person's confirmed action and the only route that
// pushes: the request must say confirm "push", and name the commit the
// person saw.
func (a *App) pushEditor(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL         string `json:"url"`
		Confirm     string `json:"confirm"`
		SHA         string `json:"sha"`
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if req.Confirm != "push" {
		writeError(w, errors.New("Pushing needs your confirmation."))
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		writeError(w, errors.New("The merge request needs a title."))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	ch, st, p, err := a.stack(ctx, r.PathValue("id"), req.URL)
	if err != nil {
		writeError(w, err)
		return
	}
	src, err := a.source(ch)
	if err != nil {
		writeError(w, err)
		return
	}
	_, err = a.Runs.PushEditor(ctx, r.PathValue("id"), src.Remote, p.Target, req.SHA, func() (string, error) {
		return st.OpenStacked(ctx, ch.Ref, forge.NewStacked{Branch: p.Branch, Target: p.Target, Title: req.Title, Description: req.Description})
	})
	if err != nil {
		writeError(w, err)
		return
	}
	a.getRun(w, r)
}
