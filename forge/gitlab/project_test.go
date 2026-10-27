package gitlab

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/terva-sh/vuoro/forge"
)

// projectFake serves project search, one project, and its open merge
// requests in pages of one, and refuses any request without the token.
func projectFake(t *testing.T, handle func(w http.ResponseWriter, r *http.Request, self string) bool) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+secret {
			w.WriteHeader(401)
			return
		}
		self := strings.Replace(srv.URL, "http://", "https://", 1)
		if !handle(w, r, self) {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestSearchAndReadAProject(t *testing.T) {
	srv := projectFake(t, func(w http.ResponseWriter, r *http.Request, self string) bool {
		switch r.URL.EscapedPath() {
		case "/api/v4/projects":
			q := r.URL.Query()
			if q.Get("membership") != "true" || q.Get("search") != "repo" || q.Get("search_namespaces") != "true" {
				t.Errorf("search query %v", q)
			}
			fmt.Fprintf(w, `[{"path_with_namespace":"team/sub/repo","name_with_namespace":"Team / Sub / Repo","web_url":%q}]`, self+"/team/sub/repo")
		case "/api/v4/projects/Team%2Fsub%2Frepo":
			fmt.Fprintf(w, `{"path_with_namespace":"team/sub/repo","name_with_namespace":"Team / Sub / Repo","web_url":%q}`, self+"/team/sub/repo")
		default:
			return false
		}
		return true
	})
	c, _ := client(t, srv, secret)
	found, err := c.SearchProjects(context.Background(), " repo ")
	if err != nil || len(found) != 1 || found[0].Path != "team/sub/repo" || found[0].Host != c.Host() {
		t.Fatalf("search %+v %v", found, err)
	}
	p, err := c.Project(context.Background(), "Team/sub/repo")
	if err != nil || p.Path != "team/sub/repo" {
		t.Fatalf("project %+v %v", p, err)
	}
	anon, _ := client(t, srv, "")
	if _, err := anon.SearchProjects(context.Background(), "repo"); err == nil {
		t.Fatal("an anonymous client searched")
	}
}

func TestAProjectAnsweringForAnotherIsRefused(t *testing.T) {
	srv := projectFake(t, func(w http.ResponseWriter, r *http.Request, self string) bool {
		fmt.Fprintf(w, `{"path_with_namespace":"team/repo","name_with_namespace":"x","web_url":%q}`, "https://elsewhere.test/team/repo")
		return true
	})
	c, _ := client(t, srv, secret)
	if _, err := c.Project(context.Background(), "team/repo"); err == nil {
		t.Fatal("a project on another host was accepted")
	}
}

func TestListOpenReadsEveryPage(t *testing.T) {
	calls := 0
	srv := projectFake(t, func(w http.ResponseWriter, r *http.Request, self string) bool {
		if r.URL.EscapedPath() != "/api/v4/projects/team%2Frepo/merge_requests" {
			return false
		}
		calls++
		if r.URL.Query().Get("state") != "opened" || r.URL.Query().Has("updated_after") {
			t.Error("incomplete listing query")
		}
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page > 3 {
			fmt.Fprint(w, `[]`)
			return true
		}
		fmt.Fprintf(w, `[{"iid":%d,"web_url":%q,"state":"opened","updated_at":"2026-10-27T00:00:00Z"}]`, page, self+"/team/repo/-/merge_requests/"+strconv.Itoa(page))
		return true
	})
	c, _ := client(t, srv, secret)
	got, err := c.ListOpen(context.Background(), "team/repo")
	if err != nil || len(got) != 3 || calls != 4 {
		t.Fatalf("listed %d in %d calls: %v", len(got), calls, err)
	}
	if got[2].Ref.Number != 3 || got[2].Updated.IsZero() {
		t.Fatalf("listed %+v", got[2])
	}
}

func TestListOpenRefusesAPartialOrForeignList(t *testing.T) {
	for _, mode := range []string{"foreign", "duplicate", "closed", "pagination", "null"} {
		t.Run(mode, func(t *testing.T) {
			srv := projectFake(t, func(w http.ResponseWriter, r *http.Request, self string) bool {
				page, _ := strconv.Atoi(r.URL.Query().Get("page"))
				row := func(iid int, project, state string) string {
					return fmt.Sprintf(`{"iid":%d,"web_url":%q,"state":%q,"updated_at":"2026-10-27T00:00:00Z"}`, iid, self+"/"+project+"/-/merge_requests/"+strconv.Itoa(iid), state)
				}
				switch {
				case mode == "foreign":
					fmt.Fprint(w, "["+row(1, "other/repo", "opened")+"]")
				case mode == "duplicate" && page <= 2:
					fmt.Fprint(w, "["+row(1, "team/repo", "opened")+"]")
				case mode == "closed":
					fmt.Fprint(w, "["+row(1, "team/repo", "closed")+"]")
				case mode == "pagination":
					w.Header().Set("X-Next-Page", "7")
					fmt.Fprint(w, "["+row(1, "team/repo", "opened")+"]")
				case mode == "null":
					fmt.Fprint(w, `null`)
				default:
					fmt.Fprint(w, `[]`)
				}
				return true
			})
			c, _ := client(t, srv, secret)
			if got, err := c.ListOpen(context.Background(), "team/repo"); err == nil {
				t.Fatalf("accepted %+v", got)
			}
		})
	}
}

func TestDependenciesAreKnownOnlyWhenGitLabAnswers(t *testing.T) {
	var answer string
	calls := 0
	srv := projectFake(t, func(w http.ResponseWriter, r *http.Request, self string) bool {
		if r.URL.Path != "/api/graphql" {
			return false
		}
		calls++
		b, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(b), "blockingMergeRequests") {
			t.Error("dependencies query not sent")
		}
		fmt.Fprint(w, strings.ReplaceAll(answer, "SELF", self))
		return true
	})
	c, ref := client(t, srv, secret)
	ref.Number = 7
	answer = `{"data":{"project":{"mergeRequest":{"iid":"7","blockingMergeRequests":{"hiddenCount":1,"visibleMergeRequests":[
		{"iid":"5","state":"opened","webUrl":"SELF/gitlab-org/cli/-/merge_requests/5"},
		{"iid":"4","state":"merged","webUrl":"SELF/gitlab-org/cli/-/merge_requests/4"}]}}}}}`
	got, err := c.Dependencies(context.Background(), ref)
	if err != nil || len(got) != 2 || got[0].Number != 5 || got[1].Project != "" {
		t.Fatalf("dependencies %+v %v", got, err)
	}
	answer = `{"data":{"project":{"mergeRequest":{"iid":"7","blockingMergeRequests":null}}}}`
	if _, err := c.Dependencies(context.Background(), ref); !errors.Is(err, forge.ErrUnsupported) {
		t.Fatalf("a null answer was not unknown: %v", err)
	}
	answer = `{"errors":[{"message":"Field 'blockingMergeRequests' doesn't exist on type 'MergeRequest'"}]}`
	if _, err := c.Dependencies(context.Background(), ref); !errors.Is(err, forge.ErrUnsupported) {
		t.Fatalf("a missing field was not unknown: %v", err)
	}
	before := calls
	if _, err := c.Dependencies(context.Background(), ref); !errors.Is(err, forge.ErrUnsupported) || calls != before {
		t.Fatal("a GitLab without the field was asked again")
	}
}

func TestFactsCarryLabelsPipelineAndConflicts(t *testing.T) {
	srv := projectFake(t, func(w http.ResponseWriter, r *http.Request, self string) bool {
		if r.URL.Path != "/api/graphql" {
			return false
		}
		fmt.Fprintf(w, `{"data":{"project":{"mergeRequest":{"iid":"3991","title":"t","state":"opened","conflicts":true,
			"labels":{"pageInfo":{"hasNextPage":false},"nodes":[{"title":"backend"},{"title":"needs review"}]},
			"headPipeline":{"status":"FAILED"},"webUrl":%q,
			"reviewers":{"nodes":[]},"assignees":{"nodes":[]},"participants":{"nodes":[]},
			"mergeRequestDiffs":{"nodes":[]},"discussions":{"nodes":[]}}}}}`, self+"/gitlab-org/cli/-/merge_requests/3991")
		return true
	})
	c, ref := client(t, srv, secret)
	ch, err := c.ReadFacts(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	if !ch.Conflicts || ch.Pipeline != "failed" || !slices.Equal(ch.Labels, []string{"backend", "needs review"}) {
		t.Fatalf("facts %v %q %v", ch.Conflicts, ch.Pipeline, ch.Labels)
	}
}
