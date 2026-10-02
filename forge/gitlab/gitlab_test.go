package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
)

const secret = "glpat-THE-SECRET-TOKEN"

// fake is a GitLab that serves the recorded merge request !3991 and the
// synthesized diffs, and refuses any request without the token.
func fake(t *testing.T) *httptest.Server {
	t.Helper()
	gql, err := os.ReadFile("testdata/mr3991.json")
	if err != nil {
		t.Fatal(err)
	}
	var pages [][]json.RawMessage
	b, _ := os.ReadFile("testdata/mr3991-diffs.json")
	if err := json.Unmarshal(b, &pages); err != nil {
		t.Fatal(err)
	}
	raw := map[string]string{
		"big.go@59ae78caddfe91cef7808d7fb7c41d04d5b80c77":   "package big\n\nfunc A() {}\n",
		"big.go@8526e3d7d20ebe488c05425d32b4563b70840df9":   "package big\n\nfunc A() {}\n\nfunc B() {}\n",
		"logo.png@59ae78caddfe91cef7808d7fb7c41d04d5b80c77": "\x89PNG\x00\x01",
		"logo.png@8526e3d7d20ebe488c05425d32b4563b70840df9": "\x89PNG\x00\x02",
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/graphql", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Variables["fullPath"] != "gitlab-org/cli" || req.Variables["iid"] != "3991" || !strings.Contains(req.Query, "mergeRequest(iid: $iid)") {
			w.Write([]byte(`{"data":{"project":null}}`))
			return
		}
		w.Write(gql)
	})
	mux.HandleFunc("GET /api/v4/projects/{project}/merge_requests/3991/diffs", func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.EscapedPath(), "gitlab-org%2Fcli") {
			http.Error(w, "project path not escaped", 400)
			return
		}
		page := 1
		fmt.Sscan(r.URL.Query().Get("page"), &page)
		if page < len(pages) {
			w.Header().Set("X-Next-Page", fmt.Sprint(page+1))
		}
		json.NewEncoder(w).Encode(pages[page-1])
	})
	mux.HandleFunc("GET /api/v4/projects/{project}/repository/files/{path}/raw", func(w http.ResponseWriter, r *http.Request) {
		body, ok := raw[r.PathValue("path")+"@"+r.URL.Query().Get("ref")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(body))
	})
	mux.HandleFunc("GET /api/v4/projects/{project}/merge_requests/3991/draft_notes", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"id": 7, "note": "my pending note", "discussion_id": null, "position": {"base_sha": "b", "start_sha": "s", "head_sha": "h", "old_path": "a.go", "new_path": "a.go", "new_line": 4, "old_line": null}}]`))
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+secret {
			http.Error(w, `{"message":"401 Unauthorized"}`, http.StatusUnauthorized)
			return
		}
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// client returns a client for srv. Open checks a ref's host against the
// client's, so the test refs use srv's host.
func client(t *testing.T, srv *httptest.Server, token string) (*Client, forge.Ref) {
	t.Helper()
	c, err := New(srv.URL, config.SecretForTest(token))
	if err != nil {
		t.Fatal(err)
	}
	return c, forge.Ref{Host: c.Host(), Project: "gitlab-org/cli", Number: 3991}
}

func TestAMergeRequestOpensWithItsDiffAndThreads(t *testing.T) {
	c, ref := client(t, fake(t), secret)
	ch, err := c.Open(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	if ch.Title != "fix(label): show the real total in the list header" || ch.Author.Username != "unstable-code" || ch.State != forge.Open {
		t.Fatalf("change %+v", ch)
	}
	if ch.Diff.Head != "8526e3d7d20ebe488c05425d32b4563b70840df9" || ch.Diff.Base == "" || ch.Diff.Start == "" {
		t.Fatalf("diff refs %+v", ch.Diff)
	}
	if ch.Mergeability != "requested_changes" {
		t.Fatalf("mergeability %q", ch.Mergeability)
	}

	states := map[string]forge.ReviewState{}
	for _, r := range ch.Reviewers {
		states[r.Username] = r.State
		if r.Username == "GitLabDuo" && !r.Bot {
			t.Error("GitLabDuo is not marked as a bot")
		}
	}
	if states["phikai"] != forge.RequestedChanges || states["rkumar555"] != forge.Unreviewed {
		t.Fatalf("reviewer states %v", states)
	}

	// The facts GitLab keeps only as system notes.
	var asked []string
	for _, r := range ch.Requests {
		asked = append(asked, r.Reviewer)
	}
	slices.Sort(asked)
	if !slices.Equal(asked, []string{"GitLabDuo", "rkumar555"}) {
		t.Fatalf("review requests %q", asked)
	}
	if len(ch.Reviews) != 1 || ch.Reviews[0].Verdict != forge.VerdictChangesRequested || ch.Reviews[0].At.IsZero() {
		t.Fatalf("reviews %+v", ch.Reviews)
	}
	if len(ch.Pushes) != 2 || !ch.Pushes[0].At.Before(ch.Pushes[1].At) || ch.Pushes[1].Head != ch.Diff.Head {
		t.Fatalf("pushes %+v", ch.Pushes)
	}

	// Discussions of system notes only became events, not threads.
	if len(ch.Threads) != 7 {
		t.Fatalf("%d threads, want 7", len(ch.Threads))
	}
	var anchored *forge.Thread
	for i, th := range ch.Threads {
		if th.Anchor != nil {
			anchored = &ch.Threads[i]
		}
	}
	if anchored == nil || anchored.Anchor.Path != "internal/commands/label/list/label_list.go" || !anchored.Resolvable || anchored.Resolved {
		t.Fatalf("anchored thread %+v", anchored)
	}
	if !slices.ContainsFunc(anchored.Notes, func(n forge.Note) bool { return n.System }) {
		t.Error("the system note inside the diff thread was lost")
	}

	if len(ch.Drafts) != 1 || ch.Drafts[0].Body != "my pending note" || ch.Drafts[0].Anchor.NewLine != 4 {
		t.Fatalf("drafts %+v", ch.Drafts)
	}
}

func TestATruncatedDiffFallsBackToReadingTheFiles(t *testing.T) {
	c, ref := client(t, fake(t), secret)
	ch, err := c.Open(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]forge.FileDiff{}
	for _, f := range ch.Files {
		files[f.NewPath] = f
	}
	if len(files) != 4 {
		t.Fatalf("%d files, want 4 across both pages", len(files))
	}
	if f := files["internal/commands/label/list/label_list.go"]; f.Rebuilt || !strings.Contains(f.Patch, "+// new") {
		t.Fatalf("normal file %+v", f)
	}
	big := files["big.go"]
	if !big.Rebuilt || big.Patch != "@@ -1,3 +1,5 @@\n package big\n \n func A() {}\n+\n+func B() {}\n" {
		t.Fatalf("collapsed file was not rebuilt: %+v", big)
	}
	if png := files["logo.png"]; !png.Omitted || png.Patch != "" {
		t.Fatalf("binary file %+v", png)
	}
	if mv := files["new/name.go"]; mv.Rebuilt || mv.Omitted || !mv.Renamed {
		t.Fatalf("a pure rename was fetched: %+v", mv)
	}
}

func TestNoTokenAppearsInAnyError(t *testing.T) {
	// A server that echoes the request's headers in its error, as a proxy
	// might.
	echo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, `{"message":"denied for %s"}`, r.Header.Get("Authorization"))
	}))
	defer echo.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, echo.URL+r.URL.Path, http.StatusFound)
	}))
	defer redirect.Close()
	gone := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	gone.Close()

	var errs []error
	for _, srv := range []*httptest.Server{echo, redirect, gone} {
		c, ref := client(t, srv, secret)
		_, err := c.Open(context.Background(), ref)
		if err == nil {
			t.Fatalf("%s: no error", srv.URL)
		}
		errs = append(errs, err)
	}
	// A wrong token against the real fake.
	c, ref := client(t, fake(t), "glpat-wrong-"+secret)
	_, err := c.Open(context.Background(), ref)
	var api *APIError
	if !errors.As(err, &api) || api.Status != http.StatusUnauthorized {
		t.Fatalf("wrong token: %v", err)
	}
	errs = append(errs, err)

	for _, err := range errs {
		if strings.Contains(err.Error(), secret) || strings.Contains(fmt.Sprintf("%+v %#v", err, err), secret) {
			t.Fatalf("the token appears in an error: %v", err)
		}
	}
	if !errors.Is(errs[1], errRedirect) {
		t.Fatalf("the redirect was followed: %v", errs[1])
	}
}

func TestAnUnknownMergeRequestSaysSo(t *testing.T) {
	c, ref := client(t, fake(t), secret)
	ref.Number = 1
	if _, err := c.Open(context.Background(), ref); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err %v", err)
	}
	ref = forge.Ref{Host: "elsewhere.example", Project: "gitlab-org/cli", Number: 3991}
	if _, err := c.Open(context.Background(), ref); err == nil {
		t.Fatal("opened a change on another host")
	}
}
