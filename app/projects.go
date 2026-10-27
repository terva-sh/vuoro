package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/forge/adapters"
	"github.com/terva-sh/vuoro/projects"
)

// The project triage routes (docs/plan.md section 6.9). Like every route they
// sit behind the window guard, which the server applies to all of /api/.
func (a *App) projectRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/projects", a.listProjects)
	mux.HandleFunc("POST /api/projects/search", a.searchProjects)
	mux.HandleFunc("POST /api/projects/add", a.addProject)
	mux.HandleFunc("POST /api/projects/remove", a.removeProject)
	mux.HandleFunc("GET /api/projects/{key}", a.getProject)
	mux.HandleFunc("POST /api/projects/{key}/refresh", a.refreshProject)
	mux.HandleFunc("POST /api/projects/{key}/administer", a.administer)
	mux.HandleFunc("GET /api/projects/{key}/members", a.projectMembers)
	mux.HandleFunc("GET /api/projects/{key}/labels", a.projectLabels)
}

func (a *App) projects() (*projects.Service, error) {
	a.projectsMu.Lock()
	defer a.projectsMu.Unlock()
	if !a.projectsReady {
		a.projectsReady = true
		a.projectService, a.projectsErr = nil, nil
		if a.configErr() != nil {
			a.projectsErr = errors.New("The configuration could not load; check the configuration file.")
			return nil, a.projectsErr
		}
		a.projectService, a.projectsErr = projects.New(a.StateDir, a.config().Forges, func(ctx context.Context, f config.Forge) (forge.Provider, error) {
			u, err := url.Parse(f.URL)
			if err != nil {
				return nil, errors.New("invalid forge URL")
			}
			return a.client(ctx, u.Host, f.Kind)
		})
		if a.projectsErr != nil {
			a.projectsErr = errors.New("The project list could not load; check local state.")
			return nil, a.projectsErr
		}
	}
	return a.projectService, a.projectsErr
}

// projectSummary is one project in the list, without its changes.
type projectSummary struct {
	projects.Entry
	Open        int    `json:"open"`
	LastSuccess string `json:"last_success,omitempty"`
	Error       string `json:"error,omitempty"`
}

type projectList struct {
	Projects []projectSummary `json:"projects"`
	// Accounts are where a search looks, by name.
	Accounts []string `json:"accounts"`
}

func (a *App) projectList(s *projects.Service) projectList {
	out := projectList{Projects: []projectSummary{}, Accounts: []string{}}
	for _, f := range s.Accounts() {
		out.Accounts = append(out.Accounts, f.Name)
	}
	for _, e := range s.Entries() {
		sum := projectSummary{Entry: e}
		if snap, ok := s.Snapshot(e.Key); ok {
			sum.Open, sum.Error = len(snap.Items), snap.Error
			if !snap.LastSuccess.IsZero() {
				sum.LastSuccess = snap.LastSuccess.Format("2006-01-02T15:04:05Z07:00")
			}
		}
		out.Projects = append(out.Projects, sum)
	}
	return out
}

func (a *App) listProjects(w http.ResponseWriter, r *http.Request) {
	s, err := a.projects()
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a.projectList(s))
}

func (a *App) searchProjects(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query string `json:"query"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	s, err := a.projects()
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.Search(r.Context(), req.Query))
}

// addProject takes a pasted URL, or a host and path picked from a search.
func (a *App) addProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL  string `json:"url,omitempty"`
		Host string `json:"host,omitempty"`
		Path string `json:"path,omitempty"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	s, err := a.projects()
	if err != nil {
		writeError(w, err)
		return
	}
	host, path := strings.ToLower(req.Host), req.Path
	if req.URL != "" {
		u, err := forge.ChangeURL(req.URL)
		if err != nil {
			writeError(w, forge.ErrNotAProject)
			return
		}
		kind := ""
		if f := a.forgeFor(u.Host); f != nil {
			kind = f.Kind
		}
		p, _, err := adapters.ParseProjectURL(req.URL, kind)
		if err != nil {
			if errors.Is(err, forge.ErrUnsupported) {
				err = errors.New("no forge account is configured for " + u.Host + "; add one in Settings")
			}
			writeError(w, err)
			return
		}
		host, path = p.Host, p.Path
	}
	if host == "" || path == "" {
		writeError(w, errors.New("name a project by URL, or pick one from a search"))
		return
	}
	e, err := s.Add(r.Context(), host, path)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"project": e, "list": a.projectList(s)})
}

func (a *App) removeProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key string `json:"key"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	s, err := a.projects()
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.Remove(req.Key); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a.projectList(s))
}

func (a *App) getProject(w http.ResponseWriter, r *http.Request) {
	s, err := a.projects()
	if err != nil {
		writeError(w, err)
		return
	}
	snap, ok := s.Snapshot(r.PathValue("key"))
	if !ok {
		writeError(w, errNoProject)
		return
	}
	writeJSON(w, http.StatusOK, snap.Display())
}

func (a *App) refreshProject(w http.ResponseWriter, r *http.Request) {
	s, err := a.projects()
	if err != nil {
		writeError(w, err)
		return
	}
	snap, err := s.Refresh(r.Context(), r.PathValue("key"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snap.Display())
}

var errNoProject = fmt.Errorf("that project is not in the list: %w", forge.ErrNotFound)

// admin returns a project and the client that administers its changes.
func (a *App) admin(r *http.Request) (*projects.Service, projects.Snapshot, forge.Provider, forge.Administrator, error) {
	s, err := a.projects()
	if err != nil {
		return nil, projects.Snapshot{}, nil, nil, err
	}
	snap, ok := s.Snapshot(r.PathValue("key"))
	if !ok {
		return nil, projects.Snapshot{}, nil, nil, errNoProject
	}
	c, err := a.client(r.Context(), snap.Project.Host, snap.Project.Kind)
	if err != nil {
		return nil, projects.Snapshot{}, nil, nil, err
	}
	ad, ok := c.(forge.Administrator)
	if !ok {
		return nil, projects.Snapshot{}, nil, nil, fmt.Errorf("changing reviewers, assignees and labels: %w", forge.ErrUnsupported)
	}
	return s, snap, c, ad, nil
}

// administer makes the person's administrative edits to one change in the
// project (D65), and answers with the project's view after a fresh read of
// that change. Each part is refused before the adapter is called when the
// forge cannot do it, so a hidden control cannot be reached by a request.
func (a *App) administer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Number  int           `json:"number"`
		Changes forge.Changes `json:"changes"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if req.Changes.Empty() {
		writeError(w, errors.New("nothing to change"))
		return
	}
	s, snap, c, ad, err := a.admin(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var ref forge.Ref
	for _, item := range snap.Items {
		if item.Change.Ref.Number == req.Number && req.Number > 0 {
			ref = item.Change.Ref
		}
	}
	if ref.Number == 0 {
		writeError(w, fmt.Errorf("that change is not open in this project: %w", forge.ErrNotFound))
		return
	}
	caps, ch := c.Capabilities(), req.Changes
	if err := firstErr(
		allowed(caps.Reviewers || len(ch.AddReviewers)+len(ch.RemoveReviewers) == 0, "changing reviewers"),
		allowed(caps.Assignees || len(ch.AddAssignees)+len(ch.RemoveAssignees) == 0, "changing assignees"),
		allowed(caps.Labels || len(ch.AddLabels)+len(ch.RemoveLabels) == 0, "changing labels"),
		allowed(caps.SetDraft || ch.Draft == nil, "marking a change draft or ready"),
	); err != nil {
		writeError(w, err)
		return
	}
	writeErr := ad.Administer(r.Context(), ref, ch)
	// Read the change again whether or not every edit landed: a partial
	// write still changed it, and the view must show what the forge holds.
	if err := s.RefreshChange(r.Context(), ref); err != nil && writeErr == nil {
		writeErr = err
	}
	if service, err := a.inbox(); err == nil {
		a.background.Go(func() { _ = service.RefreshChange(context.WithoutCancel(r.Context()), ref) })
	}
	if writeErr != nil {
		writeError(w, writeErr)
		return
	}
	snap, _ = s.Snapshot(snap.Project.Key)
	writeJSON(w, http.StatusOK, snap.Display())
}

func firstErr(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *App) projectMembers(w http.ResponseWriter, r *http.Request) {
	_, snap, _, ad, err := a.admin(r)
	if err != nil {
		writeError(w, err)
		return
	}
	members, err := ad.Members(r.Context(), snap.Project.Path, r.URL.Query().Get("query"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, members)
}

func (a *App) projectLabels(w http.ResponseWriter, r *http.Request) {
	_, snap, _, ad, err := a.admin(r)
	if err != nil {
		writeError(w, err)
		return
	}
	labels, err := ad.Labels(r.Context(), snap.Project.Path)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, labels)
}
