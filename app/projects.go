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
