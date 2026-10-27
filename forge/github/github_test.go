package github

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/turn"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// server serves the recorded pull request number n as GitHub Enterprise
// Server would, at the loopback host the client is made for. It fails a test
// that writes, or that reads without the token.
type server struct {
	t       *testing.T
	pr      []byte // the read query's data, with github.com as the host
	host    string
	graphql []string // the queries sent, in order
	// answer, when set, replaces the reply to a query it returns true for.
	answer func(q string, vars map[string]any) (any, bool)
}

func serve(t *testing.T, n string) (*Client, *server, forge.Ref) {
	t.Helper()
	s := &server{t: t, pr: fixture(t, "pr"+n+".json")}
	files, compare := fixture(t, "files"+n+".json"), fixture(t, "compare"+n+".json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sentinel-token" {
			t.Errorf("%s %s without the token", r.Method, r.URL.Path)
		}
		switch {
		case r.Method == "POST" && r.URL.Path == "/api/graphql":
			var req struct {
				Query     string         `json:"query"`
				Variables map[string]any `json:"variables"`
			}
			b, _ := io.ReadAll(r.Body)
			if json.Unmarshal(b, &req) != nil {
				t.Error("invalid GraphQL request")
			}
			s.graphql = append(s.graphql, req.Query)
			if strings.HasPrefix(strings.TrimSpace(req.Query), "mutation") {
				t.Errorf("reader sent a mutation: %s", req.Query)
			}
			if s.answer != nil {
				if v, ok := s.answer(req.Query, req.Variables); ok {
					json.NewEncoder(w).Encode(v)
					return
				}
			}
			body := strings.NewReplacer("https://github.com/", "https://"+s.host+"/", "git@github.com:", "git@"+strings.Split(s.host, ":")[0]+":").Replace(string(s.pr))
			w.Write([]byte(body))
		case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/files"):
			if r.URL.Query().Get("page") == "1" {
				w.Write(files)
			} else {
				w.Write([]byte("[]"))
			}
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/compare/"):
			w.Write(compare)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)
	s.host = strings.TrimPrefix(srv.URL, "http://")
	c, err := New(srv.URL, config.SecretForTest("sentinel-token"))
	if err != nil {
		t.Fatal(err)
	}
	if c.rest != srv.URL+"/api/v3" || c.graphql != srv.URL+"/api/graphql" {
		t.Fatalf("Enterprise API at %s and %s", c.rest, c.graphql)
	}
	num := 0
	json.Unmarshal([]byte(n), &num)
	return c, s, forge.Ref{Host: s.host, Project: "owner/repo", Number: num}
}

// edit returns s's recorded pull request after change has edited it.
func (s *server) edit(change func(pr map[string]any)) {
	s.t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(s.pr, &doc); err != nil {
		s.t.Fatal(err)
	}
	change(doc["data"].(map[string]any)["repository"].(map[string]any)["pullRequest"].(map[string]any))
	s.pr, _ = json.Marshal(doc)
}

func nodesOf(pr map[string]any, conn string) []any {
	return pr[conn].(map[string]any)["nodes"].([]any)
}

func TestParseURL(t *testing.T) {
	ref, err := ParseURL("https://github.com/Owner/repo/pull/12/files")
	if err != nil || ref != (forge.Ref{Host: "github.com", Project: "Owner/repo", Number: 12}) {
		t.Fatalf("%+v %v", ref, err)
	}
	for _, bad := range []string{"https://github.com/o/r/pulls/12", "https://github.com/o/r/issues/12", "https://github.com/g/o/r/pull/1", "http://github.com/o/r/pull/1", "https://github.com/o/r/pull/x"} {
		if _, err := ParseURL(bad); err == nil {
			t.Errorf("%s parsed", bad)
		}
	}
}

func TestTheAPIIsFoundFromTheWebHost(t *testing.T) {
	c, err := New("https://github.com", config.SecretForTest("t"))
	if err != nil || c.rest != "https://api.github.com" || c.graphql != "https://api.github.com/graphql" {
		t.Fatalf("github.com: %+v %v", c, err)
	}
	c, err = New("https://GHE.example.com/", config.SecretForTest("t"))
	if err != nil || c.rest != "https://ghe.example.com/api/v3" || c.graphql != "https://ghe.example.com/api/graphql" {
		t.Fatalf("Enterprise: %+v %v", c, err)
	}
	if _, err := New("https://github.com/api", config.SecretForTest("t")); err == nil {
		t.Fatal("a base with a path was accepted")
	}
}

func TestOpenReadsAPullRequest(t *testing.T) {
	c, _, ref := serve(t, "14578")
	ch, err := c.Open(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	if ch.Kind != forge.GitHub || ch.WebURL != "http://"+ref.Host+"/owner/repo/pull/14578" || ch.Title != "Example change 14578" || ch.State != forge.Open {
		t.Fatalf("change %+v", ch)
	}
	if ch.Diff != (forge.DiffRefs{Base: "fc4b137cdef0a6bd28fd461b7cf9c84a5812a8cd", Start: ch.Diff.Start, Head: "7cb1cfff701d4d9ae4963327686c09a71b8faba8"}) || ch.Diff.Start == "" {
		t.Fatalf("diff %+v", ch.Diff)
	}
	if ch.Checkout.HTTPS != "https://"+ref.Host+"/owner/repo.git" || ch.Checkout.HeadRef != "refs/pull/14578/head" {
		t.Fatalf("checkout %+v", ch.Checkout)
	}
	if ch.Mergeability != "clean" || ch.CanMerge == nil || !*ch.CanMerge {
		t.Fatalf("merge %q %v", ch.Mergeability, ch.CanMerge)
	}
	if len(ch.Files) != 2 || ch.Files[0].Patch == "" || !strings.HasPrefix(ch.Files[0].Patch, "@@") {
		t.Fatalf("files %+v", ch.Files)
	}
	resolved := 0
	for _, th := range ch.Threads {
		if th.Resolved {
			resolved++
			if th.Anchor == nil || th.Anchor.Path == "" || (th.Anchor.NewLine == 0 && th.Anchor.OldLine == 0) || th.Anchor.Diff.Head == "" {
				t.Errorf("resolved thread without a place: %+v", th.Anchor)
			}
		}
	}
	if resolved != 5 {
		t.Fatalf("%d resolved threads, want 5", resolved)
	}
	bot := false
	for _, r := range ch.Reviewers {
		if r.Username == "review-bot1" {
			bot = r.Bot
		}
		if r.Username == "reviewer1" && r.State != forge.Reviewed {
			t.Errorf("reviewer1's approval was of an earlier head, but their state is %s", r.State)
		}
	}
	if !bot {
		t.Error("the review bot is not marked as a bot")
	}
	// With no draft store, only project triage's writes, which need just
	// the token, are offered.
	if !reflect.DeepEqual(ch.Capabilities, forge.Capabilities{Reviewers: true, Assignees: true, Labels: true, SetDraft: true, UpdateBranch: []string{forge.UpdateMerge, forge.UpdateRebase}}) {
		t.Errorf("a reader offers review writes: %+v", ch.Capabilities)
	}
}

func TestTurnsFromFixtures(t *testing.T) {
	for _, c := range []struct {
		n, me string
		role  forge.Role
		owner turn.Owner
		rules string
	}{
		// Asked again after requesting changes, and a push since.
		{"13899", "reviewer1", forge.ReviewerRole, turn.Mine, "R1 R2 R2b"},
		{"13899", "author", forge.AuthorRole, turn.Theirs, "waiting"},
		// Approved an earlier head.
		{"14578", "reviewer1", forge.ReviewerRole, turn.Mine, "R2"},
		{"14578", "author", forge.AuthorRole, turn.Theirs, "waiting"},
	} {
		client, _, ref := serve(t, c.n)
		ch, err := client.ReadFacts(context.Background(), ref)
		if err != nil {
			t.Fatal(err)
		}
		got := turn.Derive(*ch, c.me, []forge.Role{c.role})
		var rules []string
		for _, r := range got.Reasons {
			rules = append(rules, r.Rule)
		}
		if got.Owner != c.owner || strings.Join(rules, " ") != c.rules {
			t.Errorf("%s as %s: %s %v, want %s %s", c.n, c.me, got.Owner, rules, c.owner, c.rules)
		}
	}
}

func TestReadFactsReadsNoPatches(t *testing.T) {
	c, s, ref := serve(t, "13899")
	ch, err := c.ReadFacts(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	// The fixture server fails any REST read it is not expecting, and the
	// facts make none.
	if len(ch.Files) != 0 || ch.Diff.Base != "" || len(s.graphql) != 1 {
		t.Fatalf("facts read %d files, base %q, %d queries", len(ch.Files), ch.Diff.Base, len(s.graphql))
	}
}

// A pending review is the person's draft, and is not a fact.
func TestAPendingReviewIsNotAFact(t *testing.T) {
	c, s, ref := serve(t, "13899")
	before, err := c.ReadFacts(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	s.edit(func(pr map[string]any) {
		reviews := pr["reviews"].(map[string]any)
		reviews["nodes"] = append(nodesOf(pr, "reviews"), map[string]any{"id": "PRR_pending", "author": map[string]any{"login": "me", "__typename": "User"}, "state": "PENDING", "body": "Private summary.", "submittedAt": nil, "commit": map[string]any{"oid": "7031269eafb54c8b814883a4757db7eb9f2a6100"}})
		thread := nodesOf(pr, "reviewThreads")[0].(map[string]any)
		comments := thread["comments"].(map[string]any)
		comments["nodes"] = append(comments["nodes"].([]any), map[string]any{"id": "PRRC_pending", "author": map[string]any{"login": "me", "__typename": "User"}, "body": "Private reply.", "createdAt": "2026-07-22T00:00:00Z", "pullRequestReview": map[string]any{"id": "PRR_pending", "state": "PENDING"}})
	})
	after, err := c.ReadFacts(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Reviews) != len(before.Reviews) || len(after.Threads) != len(before.Threads) || len(after.Threads[0].Notes) != len(before.Threads[0].Notes) {
		t.Fatal("a pending review reached the facts")
	}
	b, _ := json.Marshal(after)
	if strings.Contains(string(b), "Private") {
		t.Fatal("a pending comment's text reached the facts")
	}
}

// A dismissed review keeps its verdict only in the timeline (plan §4.2), and
// no longer stands.
func TestADismissedReviewTakesItsVerdictFromTheTimeline(t *testing.T) {
	c, s, ref := serve(t, "13899")
	s.edit(func(pr map[string]any) {
		review := nodesOf(pr, "reviews")[0].(map[string]any)
		review["state"] = "DISMISSED"
		tl := pr["timelineItems"].(map[string]any)
		tl["nodes"] = append(nodesOf(pr, "timelineItems"), map[string]any{"__typename": "ReviewDismissedEvent", "previousReviewState": "APPROVED", "review": map[string]any{"id": review["id"]}})
	})
	ch, err := c.ReadFacts(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	r := ch.Reviews[0]
	if r.Reviewer != "reviewer1" || r.Verdict != forge.VerdictApproved || !r.Dismissed {
		t.Fatalf("dismissed review %+v", r)
	}
}

func TestAConnectionPastAHundredIsPagedAndAMovedPullRequestRefused(t *testing.T) {
	c, s, ref := serve(t, "13899")
	var first []any
	s.edit(func(pr map[string]any) {
		first = nodesOf(pr, "reviews")
		pr["reviews"] = map[string]any{"pageInfo": map[string]any{"hasNextPage": true, "endCursor": "c1"}, "nodes": first[:1]}
	})
	var updated any
	s.answer = func(q string, vars map[string]any) (any, bool) {
		if !strings.Contains(q, "$after") {
			return nil, false
		}
		if vars["after"] != "c1" {
			t.Errorf("paged after %v", vars["after"])
		}
		return map[string]any{"data": map[string]any{"repository": map[string]any{"pullRequest": map[string]any{
			"updatedAt": updated, "headRefOid": "7031269eafb54c8b814883a4757db7eb9f2a6100",
			"reviews": map[string]any{"pageInfo": map[string]any{"hasNextPage": false}, "nodes": first[1:]}}}}}, true
	}
	var doc struct {
		Data struct {
			Repository struct {
				PullRequest struct {
					UpdatedAt string `json:"updatedAt"`
				} `json:"pullRequest"`
			} `json:"repository"`
		} `json:"data"`
	}
	json.Unmarshal(s.pr, &doc)
	updated = doc.Data.Repository.PullRequest.UpdatedAt
	ch, err := c.ReadFacts(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	if len(ch.Reviews) != len(first) {
		t.Fatalf("%d reviews after paging, want %d", len(ch.Reviews), len(first))
	}
	updated = "2030-01-01T00:00:00Z"
	if _, err := c.ReadFacts(context.Background(), ref); !errors.Is(err, errMoved) {
		t.Fatalf("a pull request that moved between pages: %v", err)
	}
}

func TestReadingNeedsAToken(t *testing.T) {
	c, err := New("https://github.com", config.Secret{})
	if err != nil {
		t.Fatal(err)
	}
	c.http.Transport = roundTrip(func(*http.Request) (*http.Response, error) {
		t.Fatal("a request went out without a token")
		return nil, nil
	})
	if _, err := c.Open(context.Background(), forge.Ref{Host: "github.com", Project: "o/r", Number: 1}); !errors.Is(err, ErrNoToken) {
		t.Fatalf("anonymous read: %v", err)
	}
}

func TestAMissingPullRequestIsNotFound(t *testing.T) {
	c, s, ref := serve(t, "13899")
	s.answer = func(string, map[string]any) (any, bool) {
		return map[string]any{"data": map[string]any{"repository": nil}, "errors": []any{map[string]any{"type": "NOT_FOUND", "message": "Could not resolve to a Repository with the name 'owner/repo'."}}}, true
	}
	if _, err := c.ReadFacts(context.Background(), ref); !errors.Is(err, forge.ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
}

func TestErrorsNeverCarryTheToken(t *testing.T) {
	c, s, ref := serve(t, "13899")
	s.answer = func(string, map[string]any) (any, bool) {
		return map[string]any{"errors": []any{map[string]any{"message": "bad credentials sentinel-token"}}}, true
	}
	_, err := c.ReadFacts(context.Background(), ref)
	if err == nil || strings.Contains(err.Error(), "sentinel-token") {
		t.Fatalf("error %v", err)
	}
}

// GitHub pairs a thread's line with its comment's commit: the head for a
// current thread, and for an outdated one the last commit it still applied
// to, which is where its line is.
func TestAThreadIsAnchoredAtTheCommitItsLineIsOn(t *testing.T) {
	c, _, ref := serve(t, "14578")
	ch, err := c.ReadFacts(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	var current, outdated bool
	for _, th := range ch.Threads {
		if a := th.Anchor; a != nil && a.Path == "pkg/cmd/extension/browse/model.go" {
			switch {
			case a.NewLine == 283 && a.Diff.Head == ch.Diff.Head:
				current = true
			case a.NewLine == 423 && a.Diff.Head == "f3adaae4dc873c3d27d99413a9d93e1f1eeec28b":
				outdated = true
			}
		}
	}
	if !current || !outdated {
		t.Fatalf("current %v, outdated %v", current, outdated)
	}
}

func TestAThreadPastAHundredCommentsIsPagedByItsID(t *testing.T) {
	c, s, ref := serve(t, "14578")
	var id string
	s.edit(func(pr map[string]any) {
		thread := nodesOf(pr, "reviewThreads")[0].(map[string]any)
		id = thread["id"].(string)
		thread["comments"].(map[string]any)["pageInfo"] = map[string]any{"hasNextPage": true, "endCursor": "t1"}
	})
	s.answer = func(q string, vars map[string]any) (any, bool) {
		if !strings.Contains(q, "node(id: $id)") {
			return nil, false
		}
		if vars["id"] != id || vars["after"] != "t1" {
			t.Errorf("paged %v", vars)
		}
		return map[string]any{"data": map[string]any{"node": map[string]any{"comments": map[string]any{"pageInfo": map[string]any{"hasNextPage": false},
			"nodes": []any{map[string]any{"id": "PRRC_late", "author": map[string]any{"login": "reviewer9", "__typename": "User"}, "body": "Late note.", "createdAt": "2026-10-07T00:00:00Z"}}}}}}, true
	}
	ch, err := c.ReadFacts(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, th := range ch.Threads {
		if th.ID == id {
			found = th.Notes[len(th.Notes)-1].ID == "PRRC_late"
		}
	}
	if !found {
		t.Fatal("the thread's later page is missing")
	}
	// The thread's page has no pull request fields, so the read is checked
	// for a move at the end.
	if last := s.graphql[len(s.graphql)-1]; !strings.Contains(last, "baseRefOid state") {
		t.Fatalf("no check after paging a thread: %s", last)
	}
}

func TestFilesAreReadToTheCountAndNoFurther(t *testing.T) {
	var pages []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		pages = append(pages, page)
		n := 100
		if page == "2" {
			n = 50
		}
		var files []apiFile
		for i := range n {
			files = append(files, apiFile{Filename: "f" + page + "-" + strconv.Itoa(i), Status: "modified", Patch: "@@ -1 +1 @@\n-a\n+b", Changes: 2})
		}
		json.NewEncoder(w).Encode(files)
	}))
	defer srv.Close()
	c, _ := New(srv.URL, config.SecretForTest("t"))
	ref := forge.Ref{Host: strings.TrimPrefix(srv.URL, "http://"), Project: "o/r", Number: 1}
	if fs, err := c.files(context.Background(), ref, 150); err != nil || len(fs) != 150 {
		t.Fatalf("150 files: %d %v", len(fs), err)
	}
	// A list that ends on a full page stops at the count, without asking for
	// a page that is not there.
	pages = nil
	if fs, err := c.files(context.Background(), ref, 100); err != nil || len(fs) != 100 || len(pages) != 1 {
		t.Fatalf("100 files: %d %v, pages %v", len(fs), err, pages)
	}
	if _, err := c.files(context.Background(), ref, 151); !errors.Is(err, errMoved) {
		t.Fatalf("a count the list does not reach: %v", err)
	}
	if _, err := c.files(context.Background(), ref, maxFiles+1); err == nil || !strings.Contains(err.Error(), "more files than GitHub lists") {
		t.Fatalf("past GitHub's list: %v", err)
	}
}
