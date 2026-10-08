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

// startEditor starts a run's editor: on a review, on the findings the person
// kept (D39); on a triage of their own change, on the threads they picked
// (D41). The change must still be at the head the run read.
func (a *App) startEditor(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL     string   `json:"url"`
		Items   []string `json:"items,omitempty"`
		Deliver string   `json:"deliver,omitempty"` // review.Suggestions, or empty for a stacked change (D59)
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
	task := review.EditorTask{Items: req.Items, Deliver: req.Deliver}
	var ch *forge.Change
	if run.Kind != review.Local { // a local range has no forge to read
		var err error
		if ch, err = a.open(ctx, req.URL); err != nil {
			writeError(w, err)
			return
		}
		task.Own = a.isAuthor(ch)
		if run.Mode == "triage" && !task.Own {
			writeError(w, errors.New("The editor fixes threads on your own change only."))
			return
		}
		if task.Deliver != "" && !ch.Capabilities.Suggestions {
			writeError(w, fmt.Errorf("suggestions: %w", forge.ErrUnsupported))
			return
		}
	}
	a.probeTools() // a run reloaded after a restart starts on a probed tool
	if _, err := a.Runs.StartEditor(ctx, run.ID, ch, a.identity(ctx), task); err != nil {
		writeError(w, err)
		return
	}
	if ch != nil && run.Mode == "" && !task.Own && ch.Capabilities.Suggestions {
		a.rememberDelivery(ch.Ref.Host, ch.Author.Username, task.Deliver)
	}
	a.getRun(w, r)
}

// delivery says whether an editor's fixes on the run's change, by author,
// can go back as suggestions (D59), and how they went back last on a change
// by that author. It reads the client's capabilities, not the change, so
// showing a run makes no forge call.
func (a *App) delivery(w http.ResponseWriter, r *http.Request) {
	out := struct {
		Offered bool   `json:"offered"`
		Deliver string `json:"deliver"`
	}{}
	run := a.Runs.Get(r.PathValue("id"))
	if run == nil {
		writeError(w, review.ErrNoRun)
		return
	}
	author := r.URL.Query().Get("author")
	f := a.forgeFor(run.Ref.Host)
	if run.Kind != review.Local && run.Mode == "" && author != "" && f != nil && !strings.EqualFold(f.Username, author) {
		c, err := a.client(r.Context(), run.Ref.Host, run.Kind)
		if err != nil {
			writeError(w, err)
			return
		}
		caps := c.Capabilities()
		out.Offered = caps.Suggestions && caps.Drafts
		if out.Offered {
			a.prefsMu.Lock()
			out.Deliver = a.loadPrefs().Deliver[deliveryKey(run.Ref.Host, author)]
			a.prefsMu.Unlock()
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// isAuthor says the signed-in person wrote the change.
func (a *App) isAuthor(ch *forge.Change) bool {
	f := a.forgeFor(ch.Ref.Host)
	return f != nil && f.Username != "" && strings.EqualFold(f.Username, ch.Author.Username)
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
	Own         bool     `json:"own,omitempty"`   // pushes to the change's own branch (D41)
	Local       bool     `json:"local,omitempty"` // writes a new branch in the person's repository (D42)
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Opened      string   `json:"opened,omitempty"`    // the draft change already open
	NotDraft    bool     `json:"not_draft,omitempty"` // the forge opened it as an ordinary change

	head string // the source branch's head as the forge reported it
}

// stack reads where the editor's work would go, for the person to confirm.
// It writes nothing.
func (a *App) stack(ctx context.Context, runID, url string) (*forge.Change, forge.Stacker, stackPreview, error) {
	var p stackPreview
	run := a.Runs.Get(runID)
	if run == nil {
		return nil, nil, p, review.ErrNoRun
	}
	if run.Kind == review.Local {
		d, err := a.Runs.Editor(ctx, runID)
		if err != nil {
			return nil, nil, p, err
		}
		if len(d.Commits) == 0 {
			return nil, nil, p, errors.New("The editor has committed nothing to hand back.")
		}
		p = stackPreview{Branch: d.Branch, Target: d.Branch, SHA: d.Commits[len(d.Commits)-1].SHA, Commits: len(d.Commits), Local: true, Files: []string{}}
		for _, f := range d.Files {
			p.Files = append(p.Files, f.NewPath)
		}
		return nil, nil, p, nil
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
		return nil, nil, p, fmt.Errorf("%w: vuoro cannot open a stacked change on this forge", forge.ErrUnsupported)
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
	// A stacked change's target moved when it is no longer the reviewed head;
	// a later wave's base is the stacked branch, not the target (D50).
	p = stackPreview{Branch: d.Branch, Target: target.Branch, SHA: d.Commits[len(d.Commits)-1].SHA, Commits: len(d.Commits),
		Moved: target.Head != v.Head, Own: d.Own, Files: []string{}, head: target.Head}
	if d.Own {
		if !a.isAuthor(ch) {
			return nil, nil, p, errors.New("The editor pushes to your own change only.")
		}
		// The branch may have moved only by the editor's own last push.
		p.Moved = target.Head != d.Base && (d.Pushed == nil || target.Head != d.Pushed.SHA)
	} else {
		p.Title, p.Description = review.StackTitle(v), review.StackDescription(v, d, a.identity(ctx).Name)
	}
	for _, f := range d.Files {
		p.Files = append(p.Files, f.NewPath)
		if ciFile(ch.Kind, f.NewPath) || ciFile(ch.Kind, f.OldPath) {
			p.CI = append(p.CI, f.NewPath)
		}
	}
	prior := d.Pushed
	if prior == nil {
		prior = d.Continues // a later wave updates the change its first opened
	}
	if prior != nil && !d.Own {
		p.Opened, p.NotDraft = prior.URL, prior.NotDraft
	}
	return ch, st, p, nil
}

// ciFile says a path configures the forge's CI, which a pushed branch runs.
// Forgejo Actions reads .forgejo/workflows, and also .gitea/workflows and
// .github/workflows; .forgejo holds its other configuration too. GitHub
// Actions reads .github/workflows, and runs actions kept in .github/actions.
func ciFile(kind, path string) bool {
	switch kind {
	case forge.Forgejo:
		return strings.HasPrefix(path, ".forgejo/") || strings.HasPrefix(path, ".gitea/workflows/") || strings.HasPrefix(path, ".github/workflows/")
	case forge.GitHub:
		return strings.HasPrefix(path, ".github/workflows/") || strings.HasPrefix(path, ".github/actions/")
	}
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
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	ch, st, p, err := a.stack(ctx, r.PathValue("id"), req.URL)
	if err != nil {
		writeError(w, err)
		return
	}
	if p.Local {
		if _, err := a.Runs.PushEditor(ctx, r.PathValue("id"), "", forge.StackTarget{}, req.SHA, nil); err != nil {
			writeError(w, err)
			return
		}
		a.getRun(w, r)
		return
	}
	if p.Own && p.Moved {
		writeError(w, fmt.Errorf("%s has moved since triage. vuoro does not force a push, so run triage again on the new head.", p.Target))
		return
	}
	if !p.Own && strings.TrimSpace(req.Title) == "" {
		writeError(w, fmt.Errorf("The %s needs a title.", review.ChangeWord(ch.Kind)))
		return
	}
	src, err := a.source(ch)
	if err != nil {
		writeError(w, err)
		return
	}
	open := func() (string, error) {
		return st.OpenStacked(ctx, ch.Ref, forge.NewStacked{Branch: p.Branch, Target: p.Target, Title: req.Title, Description: req.Description})
	}
	if p.Own {
		open = nil // the change's own branch needs no new merge request
	}
	_, err = a.Runs.PushEditor(ctx, r.PathValue("id"), src.Remote, forge.StackTarget{Branch: p.Target, Head: p.head}, req.SHA, open)
	if err != nil {
		writeError(w, err)
		return
	}
	a.getRun(w, r)
}

// fixSource is what the change of a run reads as now, for the commits a
// finding can be marked fixed by: nil for a local range, which has no forge.
func (a *App) fixSource(ctx context.Context, runID, url string) (*review.Source, error) {
	run := a.Runs.Get(runID)
	if run == nil {
		return nil, review.ErrNoRun
	}
	if run.Kind == review.Local {
		return nil, nil
	}
	ch, err := a.open(ctx, url)
	if err != nil {
		return nil, err
	}
	if run.Ref != ch.Ref || run.Kind != ch.Kind {
		return nil, errors.New("This run belongs to another change.")
	}
	src, err := a.source(ch)
	if err != nil {
		return nil, err
	}
	return &src, nil
}

// getPatch answers a fix saved as a patch, read from the run's record with no
// git or forge call (D51).
func (a *App) getPatch(w http.ResponseWriter, r *http.Request) {
	p, err := a.Runs.Patch(r.PathValue("id"), r.PathValue("sha"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// fixCommits lists the commits a finding can be marked fixed by (D50).
func (a *App) fixCommits(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	src, err := a.fixSource(ctx, r.PathValue("id"), req.URL)
	if err != nil {
		writeError(w, err)
		return
	}
	commits, err := a.Runs.FixCommits(ctx, r.PathValue("id"), src)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, commits)
}

// markFixed is the person linking a finding to the commit that fixed it, or,
// with no sha, taking back a link they made. It is local state; nothing
// reaches the forge.
func (a *App) markFixed(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
		SHA string `json:"sha"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	var src *review.Source
	if req.SHA != "" {
		var err error
		if src, err = a.fixSource(ctx, r.PathValue("id"), req.URL); err != nil {
			writeError(w, err)
			return
		}
	}
	if err := a.Runs.MarkFixed(ctx, r.PathValue("id"), r.PathValue("did"), req.SHA, src); err != nil {
		writeError(w, err)
		return
	}
	a.getRun(w, r)
}
