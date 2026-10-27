package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/forge/adapters"
	"github.com/terva-sh/vuoro/review"
)

// updateRoutes are the routes that update a change from its target (section
// 6.10, D66, D67).
func (a *App) updateRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/changes/update", a.startUpdate)
	mux.HandleFunc("POST /api/changes/updates", a.listUpdates)
	mux.HandleFunc("GET /api/runs/{id}/update", a.getUpdate)
	mux.HandleFunc("POST /api/runs/{id}/update/accept", a.acceptStop)
	mux.HandleFunc("POST /api/runs/{id}/update/discard", a.discardUpdate)
	mux.HandleFunc("POST /api/runs/{id}/update/push", a.pushUpdate)
}

// stacker is the forge client of ch as a Stacker, and where the change's
// branch is now. A fork's branch is refused, since vuoro never pushes there.
func (a *App) stacker(ctx context.Context, ch *forge.Change) (forge.Stacker, forge.StackTarget, error) {
	c, err := a.client(ctx, ch.Ref.Host, ch.Kind)
	if err != nil {
		return nil, forge.StackTarget{}, err
	}
	st, ok := c.(forge.Stacker)
	if !ok {
		return nil, forge.StackTarget{}, fmt.Errorf("%w: vuoro cannot push to a branch on this forge", forge.ErrUnsupported)
	}
	target, err := st.StackTarget(ctx, ch.Ref)
	return st, target, err
}

// startUpdate starts updating a change from its target, by merge unless the
// person asks for a rebase, which only their own change takes.
func (a *App) startUpdate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL      string `json:"url"`
		Strategy string `json:"strategy,omitempty"`
		Tool     string `json:"tool,omitempty"`
		Model    string `json:"model,omitempty"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if a.Runs == nil {
		writeError(w, errors.New("Updating a change is unavailable."))
		return
	}
	if req.Strategy == "" {
		req.Strategy = review.Merge
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	ch, err := a.open(ctx, req.URL)
	if err != nil {
		writeError(w, err)
		return
	}
	if _, _, err := a.stacker(ctx, ch); err != nil {
		writeError(w, err)
		return
	}
	a.probeTools()
	src, err := a.source(ch)
	if err != nil {
		writeError(w, err)
		return
	}
	run, err := a.Runs.StartUpdate(ch, src, review.UpdateInput{Strategy: req.Strategy, Own: a.isAuthor(ch), Who: a.identity(ctx),
		Editor: review.Reviewer{Tool: req.Tool, Model: req.Model}})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, run.View())
}

// listUpdates answers with the update runs of the change at url, newest
// first. It reads no forge, so the change page can ask on every load.
func (a *App) listUpdates(w http.ResponseWriter, r *http.Request) {
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
	ref, _, err := adapters.ParseURL(req.URL, kind)
	if err != nil {
		writeError(w, err)
		return
	}
	for _, run := range a.Runs.UpdatesOf(ref) {
		out = append(out, run.View())
	}
	writeJSON(w, http.StatusOK, out)
}

// updateRun is the update run the request names, or nil once it answered.
func (a *App) updateRun(w http.ResponseWriter, r *http.Request) *review.Run {
	run := a.Runs.Get(r.PathValue("id"))
	if run == nil || run.Mode != review.UpdateMode {
		writeError(w, review.ErrNoUpdate)
		return nil
	}
	return run
}

// getUpdate answers with an update, each stop's resolution, and the commits
// it would push.
func (a *App) getUpdate(w http.ResponseWriter, r *http.Request) {
	if a.updateRun(w, r) == nil {
		return
	}
	d, err := a.Runs.UpdateDetail(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// acceptStop accepts the editor's resolution of the update's open stop. A
// rebase then carries on to its next conflicting commit, which can take a
// while, so a closed window does not cut it short.
func (a *App) acceptStop(w http.ResponseWriter, r *http.Request) {
	if a.updateRun(w, r) == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 10*time.Minute)
	defer cancel()
	if _, err := a.Runs.AcceptStop(ctx, r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	a.getRun(w, r)
}

// discardUpdate ends an update that was not pushed and deletes its work.
func (a *App) discardUpdate(w http.ResponseWriter, r *http.Request) {
	if a.updateRun(w, r) == nil {
		return
	}
	if err := a.Runs.DiscardUpdate(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	a.getRun(w, r)
}

// pushUpdate pushes a ready update. It is the person's confirmed action and
// the only route that pushes one: the request must say confirm "push" and
// name the commit the person saw. The change's own branch takes it when the
// change is theirs; somebody else's change gets a draft change onto its
// branch.
func (a *App) pushUpdate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL         string `json:"url"`
		Confirm     string `json:"confirm"`
		SHA         string `json:"sha"`
		Title       string `json:"title,omitempty"`
		Description string `json:"description,omitempty"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	run := a.updateRun(w, r)
	if run == nil {
		return
	}
	if req.Confirm != "push" {
		writeError(w, errors.New("Pushing needs your confirmation."))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	ch, err := a.open(ctx, req.URL)
	if err != nil {
		writeError(w, err)
		return
	}
	if run.Ref != ch.Ref || run.Kind != ch.Kind {
		writeError(w, errors.New("This update belongs to another change."))
		return
	}
	st, target, err := a.stacker(ctx, ch)
	if err != nil {
		writeError(w, err)
		return
	}
	v := run.View()
	if v.Update.Own && !a.isAuthor(ch) {
		writeError(w, errors.New("vuoro pushes to your own change only."))
		return
	}
	src, err := a.source(ch)
	if err != nil {
		writeError(w, err)
		return
	}
	var open func(string) (string, error)
	if !v.Update.Own {
		title := strings.TrimSpace(req.Title)
		if title == "" {
			title = fmt.Sprintf("Merge %s into %s", v.Update.Target, v.Update.Source)
		}
		desc := req.Description
		if strings.TrimSpace(desc) == "" {
			desc = fmt.Sprintf("Merges `%s` into `%s` for %s, with the conflicts resolved by an agent and accepted by %s.",
				v.Update.Target, v.Update.Source, ch.WebURL, a.identity(ctx).Name)
		}
		open = func(branch string) (string, error) {
			return st.OpenStacked(ctx, ch.Ref, forge.NewStacked{Branch: branch, Target: target.Branch, Title: title, Description: desc})
		}
	}
	if _, err := a.Runs.PushUpdate(ctx, run.ID, src.Remote, target.Head, req.SHA, open); err != nil {
		writeError(w, err)
		return
	}
	a.getRun(w, r)
}
