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

// projectReader is the inbox's fake forge, which also lists its one project
// and administers its changes.
type projectReader struct {
	*inboxReader
	caps         *forge.Capabilities
	administered *[]forge.Changes
}

func (p projectReader) Capabilities() forge.Capabilities {
	if p.caps != nil {
		return *p.caps
	}
	return p.inboxReader.Capabilities()
}
func (p projectReader) Administer(_ context.Context, _ forge.Ref, c forge.Changes) error {
	*p.administered = append(*p.administered, c)
	return nil
}
func (p projectReader) Members(context.Context, string, string) ([]forge.Member, error) {
	return []forge.Member{{Username: "bob"}}, nil
}
func (p projectReader) Labels(context.Context, string) ([]string, error) {
	return []string{"backend"}, nil
}

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
	a, _ := projectAppWith(t, nil)
	return a
}

func projectAppWith(t *testing.T, caps *forge.Capabilities) (*App, *[]forge.Changes) {
	t.Helper()
	a, ps := inboxApp(t)
	dial := a.Dial
	administered := &[]forge.Changes{}
	a.Dial = func(kind, base string, s config.Secret) (forge.Provider, error) {
		p, err := dial(kind, base, s)
		if r, ok := p.(*inboxReader); ok && r == ps["lab.test"] {
			return projectReader{r, caps, administered}, err
		}
		return p, err
	}
	return a, administered
}

// addedProject adds team/repo on lab.test and reads it, and returns its key.
func addedProject(t *testing.T, a *App) string {
	t.Helper()
	var added struct {
		Project projects.Entry `json:"project"`
	}
	_ = json.Unmarshal(projectCall(t, a, "POST", "/api/projects/add", `{"host":"lab.test","path":"team/repo"}`, 200), &added)
	projectCall(t, a, "POST", "/api/projects/"+added.Project.Key+"/refresh", "{}", 200)
	return added.Project.Key
}

func TestAdministerAChangeInTheProject(t *testing.T) {
	a, administered := projectAppWith(t, &forge.Capabilities{Reviewers: true, Assignees: true, Labels: true, SetDraft: true})
	key := addedProject(t, a)
	var view projects.Display
	_ = json.Unmarshal(projectCall(t, a, "POST", "/api/projects/"+key+"/administer", `{"number":1,"changes":{"add_reviewers":["bob"],"remove_labels":["x"],"draft":true}}`, 200), &view)
	if len(*administered) != 1 || (*administered)[0].AddReviewers[0] != "bob" || view.Open != 1 {
		t.Fatalf("administered %+v, view %+v", *administered, view)
	}
	projectCall(t, a, "POST", "/api/projects/"+key+"/administer", `{"number":7,"changes":{"add_reviewers":["bob"]}}`, 404)
	projectCall(t, a, "POST", "/api/projects/"+key+"/administer", `{"number":1,"changes":{}}`, 400)
	if len(*administered) != 1 {
		t.Fatal("a change outside the project, or an empty edit, reached the forge")
	}
	var members []forge.Member
	_ = json.Unmarshal(projectCall(t, a, "GET", "/api/projects/"+key+"/members?query=b", "", 200), &members)
	if len(members) != 1 {
		t.Fatalf("members %+v", members)
	}
	projectCall(t, a, "GET", "/api/projects/"+key+"/labels", "", 200)
}

// A write the forge cannot do is refused before the adapter, part by part, so
// a hidden control cannot be reached by a request either (D44).
func TestAdministerRefusesWhatTheForgeCannotDo(t *testing.T) {
	a, administered := projectAppWith(t, &forge.Capabilities{Reviewers: true})
	key := addedProject(t, a)
	for _, body := range []string{
		`{"number":1,"changes":{"add_assignees":["bob"]}}`,
		`{"number":1,"changes":{"add_labels":["x"]}}`,
		`{"number":1,"changes":{"draft":false}}`,
		`{"number":1,"changes":{"add_reviewers":["bob"],"remove_labels":["x"]}}`,
	} {
		projectCall(t, a, "POST", "/api/projects/"+key+"/administer", body, 422)
	}
	if len(*administered) != 0 {
		t.Fatalf("a refused write reached the adapter: %+v", *administered)
	}
	projectCall(t, a, "POST", "/api/projects/"+key+"/administer", `{"number":1,"changes":{"add_reviewers":["bob"]}}`, 200)
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
	for _, route := range []string{"GET /api/projects", "POST /api/projects/search", "POST /api/projects/add", "POST /api/projects/remove", "GET /api/projects/k", "POST /api/projects/k/refresh", "POST /api/projects/k/administer", "GET /api/projects/k/members", "GET /api/projects/k/labels"} {
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
