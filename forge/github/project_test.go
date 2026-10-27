package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
)

// call is one request the fake GitHub received.
type call struct {
	method, path, query string
	body                map[string]any
}

// fakeGitHub answers GraphQL with gql and REST with rest, and records every
// request.
func fakeGitHub(t *testing.T, gql func(q string, vars map[string]any) any, rest func(c call) (int, any)) (*Client, *[]call) {
	t.Helper()
	var calls []call
	var c *Client
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sentinel-token" {
			t.Errorf("%s %s without the token", r.Method, r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(b, &body)
		if r.URL.Path == "/api/graphql" {
			vars, _ := body["variables"].(map[string]any)
			calls = append(calls, call{method: "GRAPHQL", query: body["query"].(string), body: vars})
			json.NewEncoder(w).Encode(map[string]any{"data": gql(body["query"].(string), vars)})
			return
		}
		cl := call{method: r.Method, path: strings.TrimPrefix(r.URL.Path, "/api/v3"), query: r.URL.RawQuery, body: body}
		calls = append(calls, cl)
		status, out := rest(cl)
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(out)
	}))
	t.Cleanup(srv.Close)
	c, err := New(srv.URL, config.SecretForTest("sentinel-token"))
	if err != nil {
		t.Fatal(err)
	}
	return c, &calls
}

func noGraphQL(t *testing.T) func(string, map[string]any) any {
	return func(q string, _ map[string]any) any { t.Errorf("unexpected query %s", q); return nil }
}

func TestListOpenReadsEveryPageAndRefusesARepeat(t *testing.T) {
	var host string
	pages := [][]int{{1, 2}, {3}}
	c, calls := fakeGitHub(t, func(q string, vars map[string]any) any {
		page := 0
		if vars["after"] != nil {
			page = 1
		}
		var nodes []any
		for _, n := range pages[page] {
			nodes = append(nodes, map[string]any{"number": n, "url": fmt.Sprintf("https://%s/o/r/pull/%d", host, n), "state": "OPEN", "updatedAt": "2026-10-27T00:00:00Z"})
		}
		return map[string]any{"repository": map[string]any{"pullRequests": map[string]any{"pageInfo": map[string]any{"hasNextPage": page == 0, "endCursor": "c1"}, "nodes": nodes}}}
	}, nil)
	host = c.web.Host
	got, err := c.ListOpen(context.Background(), "o/r")
	if err != nil || len(got) != 3 || got[2].Ref.Number != 3 || len(*calls) != 2 {
		t.Fatalf("%+v %v", got, err)
	}
	pages[1] = []int{2}
	if _, err := c.ListOpen(context.Background(), "o/r"); err == nil {
		t.Fatal("a pull request listed twice was accepted")
	}
}

func TestAdministerAddsAndRemovesWithoutReplacing(t *testing.T) {
	c, calls := fakeGitHub(t, func(q string, _ map[string]any) any {
		if strings.Contains(q, "convertPullRequestToDraft") {
			return map[string]any{"convertPullRequestToDraft": map[string]any{"pullRequest": map[string]any{"isDraft": true}}}
		}
		return map[string]any{"repository": map[string]any{"pullRequest": map[string]any{"id": "PR_1"}}}
	}, func(cl call) (int, any) {
		if cl.method == "POST" && strings.HasSuffix(cl.path, "/assignees") {
			return 201, map[string]any{"assignees": []any{map[string]any{"login": "Bob"}}}
		}
		if cl.method == "DELETE" && strings.HasSuffix(cl.path, "/labels/gone") {
			return 404, map[string]any{"message": "Label does not exist"}
		}
		return 200, map[string]any{}
	})
	draft := true
	ref := forge.Ref{Host: c.web.Host, Project: "o/r", Number: 4}
	err := c.Administer(context.Background(), ref, forge.Changes{AddReviewers: []string{"bob"}, RemoveReviewers: []string{"cat"}, AddAssignees: []string{"bob"},
		RemoveAssignees: []string{"dan"}, AddLabels: []string{"bug"}, RemoveLabels: []string{"needs info", "gone"}, Draft: &draft})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, cl := range *calls {
		if cl.method == "GRAPHQL" {
			got = append(got, "GRAPHQL "+strings.Fields(cl.query)[0])
		} else {
			got = append(got, cl.method+" "+cl.path)
		}
	}
	want := []string{"POST /repos/o/r/pulls/4/requested_reviewers", "DELETE /repos/o/r/pulls/4/requested_reviewers", "POST /repos/o/r/issues/4/assignees",
		"DELETE /repos/o/r/issues/4/assignees", "POST /repos/o/r/issues/4/labels", "DELETE /repos/o/r/issues/4/labels/needs info",
		"DELETE /repos/o/r/issues/4/labels/gone", "GRAPHQL query($owner:", "GRAPHQL mutation($id:"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("calls:\n%s", strings.Join(got, "\n"))
	}
	if (*calls)[0].body["reviewers"].([]any)[0] != "bob" || (*calls)[4].body["labels"].([]any)[0] != "bug" {
		t.Fatalf("bodies %+v", *calls)
	}

	// An assignee GitHub silently drops is a failure, naming what was done.
	err = c.Administer(context.Background(), ref, forge.Changes{AddReviewers: []string{"bob"}, AddAssignees: []string{"mallory"}})
	if err == nil || !strings.Contains(err.Error(), "mallory cannot be assigned") || !strings.Contains(err.Error(), "already added reviewers") {
		t.Fatalf("dropped assignee: %v", err)
	}
	if err := c.Administer(context.Background(), forge.Ref{Host: "elsewhere.test", Project: "o/r", Number: 4}, forge.Changes{AddLabels: []string{"x"}}); err == nil {
		t.Fatal("a change on another host was administered")
	}
}

func TestMembersLabelsAndTriageFacts(t *testing.T) {
	owners := base64.StdEncoding.EncodeToString([]byte("* @o/core\n"))
	c, calls := fakeGitHub(t, func(q string, vars map[string]any) any {
		switch {
		case strings.Contains(q, "assignableUsers"):
			if vars["query"] != "bo" {
				t.Errorf("members query %v", vars["query"])
			}
			return map[string]any{"repository": map[string]any{"assignableUsers": map[string]any{"pageInfo": map[string]any{}, "nodes": []any{map[string]any{"login": "bob", "name": "Bob"}}}}}
		case strings.Contains(q, "labels"):
			return map[string]any{"repository": map[string]any{"labels": map[string]any{"pageInfo": map[string]any{}, "nodes": []any{map[string]any{"name": "bug"}}}}}
		}
		return nil
	}, func(cl call) (int, any) {
		switch {
		case cl.path == "/repos/o/r/contents/.github/CODEOWNERS":
			return 404, map[string]any{"message": "Not Found"}
		case cl.path == "/repos/o/r/contents/CODEOWNERS":
			return 200, map[string]any{"encoding": "base64", "content": owners}
		case cl.path == "/repos/o/r/commits":
			return 200, []any{
				map[string]any{"author": map[string]any{"login": "bob"}, "commit": map[string]any{"author": map[string]any{"name": "Bob B"}}},
				map[string]any{"author": nil, "commit": map[string]any{"author": map[string]any{"name": "Unlinked"}}},
				map[string]any{"author": map[string]any{"login": "bob"}, "commit": map[string]any{"author": map[string]any{"name": "Bob B"}}},
			}
		}
		return 404, map[string]any{}
	})
	ctx := context.Background()
	if m, err := c.Members(ctx, "o/r", " bo "); err != nil || len(m) != 1 || m[0].Username != "bob" {
		t.Fatalf("members %+v %v", m, err)
	}
	if l, err := c.Labels(ctx, "o/r"); err != nil || len(l) != 1 || l[0] != "bug" {
		t.Fatalf("labels %+v %v", l, err)
	}
	if text, err := c.CodeOwners(ctx, "o/r", "main"); err != nil || text != "* @o/core\n" {
		t.Fatalf("codeowners %q %v", text, err)
	}
	if who, err := c.RecentAuthors(ctx, "o/r", "main", "api"); err != nil || strings.Join(who, ",") != "bob,Unlinked" {
		t.Fatalf("authors %v %v", who, err)
	}
	last := (*calls)[len(*calls)-1]
	if !strings.Contains(last.query, "path=api") || !strings.Contains(last.query, "sha=main") {
		t.Fatalf("commits query %s", last.query)
	}
}

func TestSearchAndAddARepository(t *testing.T) {
	var host string
	c, _ := fakeGitHub(t, func(q string, vars map[string]any) any {
		repo := map[string]any{"nameWithOwner": "O/R", "url": "https://" + host + "/O/R"}
		switch {
		case strings.Contains(q, "search("):
			if !strings.HasPrefix(vars["q"].(string), "r in:name") {
				t.Errorf("search %v", vars["q"])
			}
			return map[string]any{"search": map[string]any{"nodes": []any{repo}}}
		case strings.Contains(q, "viewer"):
			return map[string]any{"viewer": map[string]any{"repositories": map[string]any{"nodes": []any{repo}}}}
		case vars["name"] == "missing":
			return map[string]any{"repository": nil}
		}
		return map[string]any{"repository": repo}
	}, nil)
	host = c.web.Host
	ctx := context.Background()
	for _, q := range []string{"r", ""} {
		if got, err := c.SearchProjects(ctx, q); err != nil || len(got) != 1 || got[0].Path != "O/R" {
			t.Fatalf("search %q: %+v %v", q, got, err)
		}
	}
	if p, err := c.Project(ctx, "o/r"); err != nil || p.Path != "O/R" {
		t.Fatalf("project %+v %v", p, err)
	}
	if _, err := c.Project(ctx, "o/missing"); !errors.Is(err, forge.ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
	if _, err := c.Project(ctx, "g/o/r"); !errors.Is(err, forge.ErrNotAProject) {
		t.Fatalf("three parts: %v", err)
	}
	if p, err := ParseProjectURL("https://github.com/o/r/pulls"); err != nil || p.Path != "o/r" {
		t.Fatalf("parse %+v %v", p, err)
	}
}

func TestFactsCarryWhatProjectTriageReads(t *testing.T) {
	c, s, ref := serve(t, "14578")
	s.edit(func(pr map[string]any) {
		reqs := pr["reviewRequests"].(map[string]any)
		reqs["nodes"] = append(reqs["nodes"].([]any), map[string]any{"requestedReviewer": map[string]any{"__typename": "Team", "slug": "core", "organization": map[string]any{"login": "owner"}}})
		pr["mergeable"] = "CONFLICTING"
	})
	ch, err := c.ReadFacts(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(ch.Labels, ",") != "enhancement" || ch.Pipeline != "success" || !ch.Conflicts || ch.SourceProject != "owner/repo" ||
		len(ch.Touched) != 2 || ch.Touched[0].Path == "" || strings.Join(ch.Teams, ",") != "owner/core" {
		t.Fatalf("labels %v pipeline %q conflicts %v source %q touched %+v teams %v", ch.Labels, ch.Pipeline, ch.Conflicts, ch.SourceProject, ch.Touched, ch.Teams)
	}
	for _, r := range ch.Reviewers {
		if strings.Contains(r.Username, "core") {
			t.Fatal("a team became a reviewer")
		}
	}
	c, _, ref = serve(t, "13899")
	if ch, err = c.ReadFacts(context.Background(), ref); err != nil || ch.Pipeline != "" || len(ch.Labels) != 0 {
		t.Fatalf("no checks or labels: %q %v %v", ch.Pipeline, ch.Labels, err)
	}
}
