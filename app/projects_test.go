package app

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/projects"
	"github.com/terva-sh/vuoro/server"
)

// projectReader is the inbox's fake forge, which also lists its one project.
type projectReader struct{ *inboxReader }

func (p projectReader) SearchProjects(context.Context, string) ([]forge.Project, error) {
	return []forge.Project{{Host: p.host, Path: "team/repo", Name: "Team / Repo"}}, nil
}
func (p projectReader) Project(_ context.Context, path string) (forge.Project, error) {
	if path != "team/repo" {
		return forge.Project{}, forge.ErrNotFound
	}
	return forge.Project{Host: p.host, Path: "team/repo", Name: "Team / Repo", WebURL: "https://" + p.host + "/team/repo"}, nil
}
func (p projectReader) ListOpen(context.Context, string) ([]forge.Discovered, error) {
	return []forge.Discovered{{Ref: forge.Ref{Host: p.host, Project: "team/repo", Number: 1}}}, nil
}

func projectApp(t *testing.T) *App {
	t.Helper()
	a, ps := inboxApp(t)
	dial := a.Dial
	a.Dial = func(kind, base string, s config.Secret) (forge.Provider, error) {
		p, err := dial(kind, base, s)
		if r, ok := p.(*inboxReader); ok && r == ps["lab.test"] {
			return projectReader{r}, err
		}
		return p, err
	}
	return a
}

func projectCall(t *testing.T, a *App, method, path, body string, want int) []byte {
	t.Helper()
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
	if w.Code != want {
		t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body.String())
	}
	return w.Body.Bytes()
}

func TestAddAProjectByURLAndReadItsChanges(t *testing.T) {
	a := projectApp(t)
	var added struct {
		Project projects.Entry `json:"project"`
	}
	b := projectCall(t, a, "POST", "/api/projects/add", `{"url":"https://lab.test/team/repo/-/merge_requests/4"}`, 200)
	if err := json.Unmarshal(b, &added); err != nil || added.Project.Path != "team/repo" {
		t.Fatalf("added %s", b)
	}
	var view projects.Display
	b = projectCall(t, a, "POST", "/api/projects/"+added.Project.Key+"/refresh", "{}", 200)
	if err := json.Unmarshal(b, &view); err != nil || view.Open != 1 || view.Groups[0].Rows[0].Title != "A change" {
		t.Fatalf("refreshed %s", b)
	}
	if view.Groups[0].Place != projects.WaitingReview || view.Groups[0].Rows[0].Holder[0] != "me" {
		t.Fatalf("where it sits %+v", view.Groups[0])
	}
	projectCall(t, a, "GET", "/api/projects/nonesuch", "", 404)
	projectCall(t, a, "POST", "/api/projects/add", `{"url":"https://lab.test/other/repo"}`, 400)
	// Forgejo cannot list projects yet, and says so rather than adding one.
	projectCall(t, a, "POST", "/api/projects/add", `{"url":"https://jo.test/team/repo"}`, 400)
	var list projectList
	_ = json.Unmarshal(projectCall(t, a, "GET", "/api/projects", "", 200), &list)
	if len(list.Projects) != 1 || list.Projects[0].Open != 1 {
		t.Fatalf("list %+v", list)
	}
	_ = json.Unmarshal(projectCall(t, a, "POST", "/api/projects/remove", `{"key":"`+added.Project.Key+`"}`, 200), &list)
	if len(list.Projects) != 0 {
		t.Fatal("remove left the project")
	}
}

func TestSearchProjectsAcrossAccounts(t *testing.T) {
	a := projectApp(t)
	var got projects.SearchResult
	_ = json.Unmarshal(projectCall(t, a, "POST", "/api/projects/search", `{"query":"repo"}`, 200), &got)
	if len(got.Projects) != 1 || got.Projects[0].Account != "Work" || len(got.Errors) != 0 {
		t.Fatalf("search %+v", got)
	}
}

func TestProjectRoutesStayBehindWindowGuard(t *testing.T) {
	a := projectApp(t)
	h := server.New(server.Config{Addr: "127.0.0.1:9123", Token: "fixture-window-token", Dist: fstest.MapFS{}, API: a.Handler()})
	for _, route := range []string{"GET /api/projects", "POST /api/projects/search", "POST /api/projects/add", "POST /api/projects/remove", "GET /api/projects/k", "POST /api/projects/k/refresh"} {
		method, path, _ := strings.Cut(route, " ")
		for _, mode := range []string{"missing-token", "other-origin", "rebound-host", "agent-token"} {
			r := httptest.NewRequest(method, "http://127.0.0.1:9123"+path, strings.NewReader("{}"))
			r.Header.Set(server.TokenHeader, "fixture-window-token")
			switch mode {
			case "missing-token":
				r.Header.Del(server.TokenHeader)
			case "other-origin":
				r.Header.Set("Origin", "https://elsewhere.test")
			case "rebound-host":
				r.Host = "elsewhere.test"
			case "agent-token":
				r.Header.Set(server.TokenHeader, "fixture-drafts-token")
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != 403 {
				t.Fatalf("%s %s bypassed window guard", route, mode)
			}
		}
	}
}
