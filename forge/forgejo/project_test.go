package forgejo

import (
	"context"
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

type projectCall struct {
	method, path string
	query        string
	body         map[string]any
}

// fakeForgejo answers with answer and records every request.
func fakeForgejo(t *testing.T, answer func(c projectCall) (int, any)) (*Client, *[]projectCall) {
	t.Helper()
	var calls []projectCall
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "token synthetic-forgejo-token" {
			t.Errorf("%s %s without the token", r.Method, r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(b, &body)
		c := projectCall{method: r.Method, path: strings.TrimPrefix(r.URL.Path, "/api/v1"), query: r.URL.RawQuery, body: body}
		calls = append(calls, c)
		status, out := answer(c)
		w.WriteHeader(status)
		if s, ok := out.(string); ok {
			io.WriteString(w, s)
			return
		}
		json.NewEncoder(w).Encode(out)
	}))
	t.Cleanup(srv.Close)
	c, err := New(srv.URL, config.SecretForTest("synthetic-forgejo-token"))
	if err != nil {
		t.Fatal(err)
	}
	return c, &calls
}

func page(c projectCall) string {
	for _, kv := range strings.Split(c.query, "&") {
		if v, ok := strings.CutPrefix(kv, "page="); ok {
			return v
		}
	}
	return ""
}

func TestForgejoListOpenReadsEveryPageAndRefusesARepeat(t *testing.T) {
	var host string
	pages := map[string][]int{"1": {1, 2}, "2": {3}}
	c, calls := fakeForgejo(t, func(c projectCall) (int, any) {
		rows := []any{}
		for _, n := range pages[page(c)] {
			rows = append(rows, map[string]any{"number": n, "html_url": fmt.Sprintf("https://%s/o/r/pulls/%d", host, n), "state": "open", "updated_at": "2026-10-27T00:00:00Z"})
		}
		return 200, rows
	})
	host = c.base.Host
	got, err := c.ListOpen(context.Background(), "o/r")
	if err != nil || len(got) != 3 || len(*calls) != 3 {
		t.Fatalf("%+v %v", got, err)
	}
	if q := (*calls)[0].query; !strings.Contains(q, "state=open") || !strings.Contains(q, "page=1") {
		t.Fatalf("listing query %s", q)
	}
	pages["2"] = []int{2}
	if _, err := c.ListOpen(context.Background(), "o/r"); err == nil {
		t.Fatal("a pull request listed twice was accepted")
	}
}

func TestForgejoAdministerKeepsOthersEdits(t *testing.T) {
	c, calls := fakeForgejo(t, func(c projectCall) (int, any) {
		switch {
		case c.method == "GET" && c.path == "/repos/o/r/pulls/4":
			// Someone else assigned cat and retitled the pull request since
			// the person looked.
			return 200, map[string]any{"title": "[WIP] Their new title", "assignees": []any{map[string]any{"login": "cat"}, map[string]any{"login": "dan"}}}
		case c.method == "GET" && c.path == "/repos/o/r/labels":
			if page(c) == "1" {
				return 200, []any{map[string]any{"id": 7, "name": "bug"}}
			}
			return 200, []any{}
		case c.method == "GET" && c.path == "/orgs/o/labels":
			return 404, map[string]any{"message": "not an org"}
		case c.method == "GET" && c.path == "/repos/o/r/issues/4/labels":
			return 200, []any{map[string]any{"id": 9, "name": "needs info"}}
		}
		return 200, map[string]any{}
	})
	ready := false
	ref := forge.Ref{Host: c.base.Host, Project: "o/r", Number: 4}
	err := c.Administer(context.Background(), ref, forge.Changes{AddReviewers: []string{"bob"}, RemoveReviewers: []string{"eve"}, AddAssignees: []string{"bob"},
		RemoveAssignees: []string{"dan"}, AddLabels: []string{"bug"}, RemoveLabels: []string{"needs info", "absent"}, Draft: &ready})
	if err != nil {
		t.Fatal(err)
	}
	var writes []string
	for _, cl := range *calls {
		if cl.method != "GET" {
			b, _ := json.Marshal(cl.body)
			writes = append(writes, cl.method+" "+cl.path+" "+string(b))
		}
	}
	want := []string{
		`POST /repos/o/r/pulls/4/requested_reviewers {"reviewers":["bob"]}`,
		`DELETE /repos/o/r/pulls/4/requested_reviewers {"reviewers":["eve"]}`,
		`PATCH /repos/o/r/issues/4 {"assignees":["cat","bob"]}`,
		`POST /repos/o/r/issues/4/labels {"labels":[7]}`,
		`DELETE /repos/o/r/issues/4/labels/9 null`,
		`PATCH /repos/o/r/pulls/4 {"title":"Their new title"}`,
	}
	if strings.Join(writes, "\n") != strings.Join(want, "\n") {
		t.Fatalf("writes:\n%s", strings.Join(writes, "\n"))
	}
	if err := c.Administer(context.Background(), ref, forge.Changes{AddReviewers: []string{"bob"}, AddLabels: []string{"made-up"}}); err == nil ||
		!strings.Contains(err.Error(), `no label "made-up"`) || !strings.Contains(err.Error(), "already added reviewers") {
		t.Fatalf("unknown label: %v", err)
	}
}

func TestForgejoDraftIsATitlePrefix(t *testing.T) {
	for _, c := range []struct {
		title string
		draft bool
		want  string // the title written, or "" for no write
	}{
		{"Add it", true, "WIP: Add it"},
		{"WIP: Add it", true, ""},
		{"wip:Add it", false, "Add it"},
		{"[WIP] Add it", false, "Add it"},
		{"Add it", false, ""},
	} {
		var wrote string
		cl, _ := fakeForgejo(t, func(pc projectCall) (int, any) {
			if pc.method == "PATCH" {
				wrote = pc.body["title"].(string)
			}
			return 200, map[string]any{"title": c.title}
		})
		if err := cl.Administer(context.Background(), forge.Ref{Host: cl.base.Host, Project: "o/r", Number: 1}, forge.Changes{Draft: &c.draft}); err != nil || wrote != c.want {
			t.Errorf("%q draft=%v: wrote %q, %v", c.title, c.draft, wrote, err)
		}
	}
}

func TestForgejoSearchMembersLabelsAndTriageFacts(t *testing.T) {
	var host string
	c, calls := fakeForgejo(t, func(c projectCall) (int, any) {
		repo := map[string]any{"full_name": "O/R", "html_url": "https://" + host + "/O/R"}
		switch c.path {
		case "/repos/search":
			return 200, map[string]any{"ok": true, "data": []any{repo}}
		case "/repos/o/r", "/repos/O/R":
			return 200, repo
		case "/repos/o/r/assignees":
			return 200, []any{map[string]any{"login": "bob", "full_name": "Bob"}, map[string]any{"login": "cat", "full_name": "Cat"}}
		case "/repos/o/r/labels":
			if page(c) == "1" {
				return 200, []any{map[string]any{"id": 1, "name": "bug"}}
			}
			return 200, []any{}
		case "/orgs/o/labels":
			if page(c) == "1" {
				return 200, []any{map[string]any{"id": 2, "name": "org-wide"}, map[string]any{"id": 3, "name": "bug"}}
			}
			return 200, []any{}
		case "/repos/o/r/raw/CODEOWNERS":
			return 404, map[string]any{}
		case "/repos/o/r/raw/docs/CODEOWNERS":
			return 200, "* @o/core\n"
		case "/repos/o/r/commits":
			return 200, []any{map[string]any{"author": map[string]any{"login": "bob"}, "commit": map[string]any{"author": map[string]any{"name": "Bob B"}}},
				map[string]any{"author": nil, "commit": map[string]any{"author": map[string]any{"name": "Unlinked"}}}}
		}
		return 404, map[string]any{}
	})
	host = c.base.Host
	ctx := context.Background()
	if got, err := c.SearchProjects(ctx, "r"); err != nil || len(got) != 1 || got[0].Path != "O/R" || !strings.Contains((*calls)[0].query, "q=r") {
		t.Fatalf("search %+v %v", got, err)
	}
	if p, err := c.Project(ctx, "o/r"); err != nil || p.Path != "O/R" {
		t.Fatalf("project %+v %v", p, err)
	}
	if _, err := c.Project(ctx, "o/missing"); !errors.Is(err, forge.ErrNotFound) {
		t.Fatalf("missing %v", err)
	}
	if _, err := c.Project(ctx, "g/o/r"); !errors.Is(err, forge.ErrNotAProject) {
		t.Fatalf("three parts %v", err)
	}
	if m, err := c.Members(ctx, "o/r", "BO"); err != nil || len(m) != 1 || m[0].Username != "bob" {
		t.Fatalf("members %+v %v", m, err)
	}
	if l, err := c.Labels(ctx, "o/r"); err != nil || strings.Join(l, ",") != "bug,org-wide" {
		t.Fatalf("labels %v %v", l, err)
	}
	if text, err := c.CodeOwners(ctx, "o/r", "main"); err != nil || text != "* @o/core\n" {
		t.Fatalf("codeowners %q %v", text, err)
	}
	if who, err := c.RecentAuthors(ctx, "o/r", "main", "api"); err != nil || strings.Join(who, ",") != "bob,Unlinked" {
		t.Fatalf("authors %v %v", who, err)
	}
}
