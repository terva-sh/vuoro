package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/terva-sh/vuoro/forge"
)

func TestDiscoveryCombinesCompleteMembershipAndRoles(t *testing.T) {
	calls := 0
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.Header.Get("Authorization") != "Bearer "+secret {
			t.Error("unexpected method/auth")
			w.WriteHeader(403)
			return
		}
		if r.URL.Path == "/api/v4/user" {
			fmt.Fprint(w, `{"username":"me"}`)
			return
		}
		if r.URL.Path != "/api/v4/merge_requests" {
			t.Error("unexpected endpoint")
			w.WriteHeader(404)
			return
		}
		calls++
		q := r.URL.Query()
		if q.Get("scope") != "all" || q.Get("state") != "opened" || q.Has("updated_after") {
			t.Error("incomplete membership query")
		}
		role := ""
		for _, key := range []string{"author_username", "reviewer_username", "assignee_username[]"} {
			if q.Get(key) == "me" {
				role = key
			}
		}
		if role == "" {
			t.Error("no role filter")
		}
		page, _ := strconv.Atoi(q.Get("page"))
		// Clamp page size to one; omit headers to exercise empty-page termination.
		if page > 2 {
			fmt.Fprint(w, `[]`)
			return
		}
		fmt.Fprintf(w, `[{"iid":%d,"web_url":%q,"state":"opened","updated_at":"2026-10-02T00:00:00Z"}]`, page, strings.Replace(srv.URL, "http://", "https://", 1)+"/team/nested/repo/-/merge_requests/"+strconv.Itoa(page))
	}))
	defer srv.Close()
	c, _ := client(t, srv, secret)
	got, err := c.Discover(context.Background(), "me")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || calls != 9 {
		t.Fatalf("got %d members, %d calls", len(got), calls)
	}
	for _, m := range got {
		if !slices.Equal(m.Roles, []forge.Role{forge.AuthorRole, forge.ReviewerRole, forge.AssigneeRole}) || m.Ref.Project != "team/nested/repo" || m.Updated.IsZero() {
			t.Fatalf("member %+v", m)
		}
	}
}

func TestDiscoveryRejectsPartialOrWrongAccountResults(t *testing.T) {
	for _, mode := range []string{"account", "identity", "pagination", "later-error", "rate-limit", "null", "redirect"} {
		t.Run(mode, func(t *testing.T) {
			var srv *httptest.Server
			srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v4/user" {
					username := "me"
					if mode == "account" {
						username = "other"
					}
					fmt.Fprintf(w, `{"username":%q}`, username)
					return
				}
				if mode == "redirect" {
					http.Redirect(w, r, "https://elsewhere.test", 302)
					return
				}
				if mode == "rate-limit" {
					w.WriteHeader(429)
					fmt.Fprintf(w, `{"message":%q}`, strings.Repeat("x", 195)+secret)
					return
				}
				if mode == "null" {
					fmt.Fprint(w, `null`)
					return
				}
				if r.URL.Query().Get("page") == "2" {
					w.WriteHeader(500)
					return
				}
				target := strings.Replace(srv.URL, "http://", "https://", 1) + "/team/repo/-/merge_requests/1"
				if mode == "identity" {
					target = "https://elsewhere.test/team/repo/-/merge_requests/1"
				}
				next := "2"
				if mode == "pagination" {
					next = "1"
				}
				w.Header().Set("X-Next-Page", next)
				fmt.Fprintf(w, `[{"iid":1,"web_url":%q,"state":"opened"}]`, target)
			}))
			defer srv.Close()
			c, _ := client(t, srv, secret)
			got, err := c.Discover(context.Background(), "me")
			if err == nil || got != nil {
				t.Fatal("partial discovery succeeded")
			}
			if strings.Contains(fmt.Sprintf("%+v %#v", err, err), secret) || strings.Contains(err.Error(), "glpat-") {
				t.Fatal("credential or prefix leaked")
			}
		})
	}
}

func factsFixture(t *testing.T) map[string]any {
	t.Helper()
	b, err := os.ReadFile("testdata/mr3991.json")
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err = json.Unmarshal(b, &result); err != nil {
		t.Fatal(err)
	}
	return result
}
func fixtureMR(result map[string]any) map[string]any {
	return result["data"].(map[string]any)["project"].(map[string]any)["mergeRequest"].(map[string]any)
}

func TestFactsReaderOmitsPatchesAndDraftsAndReadsOwnPermission(t *testing.T) {
	result := factsFixture(t)
	fixtureMR(result)["userPermissions"] = map[string]any{"canMerge": false}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/graphql" || r.Method != "POST" {
			t.Error("facts downloaded patches/drafts")
			w.WriteHeader(500)
			return
		}
		json.NewEncoder(w).Encode(result)
	}))
	defer srv.Close()
	c, ref := client(t, srv, secret)
	got, err := c.ReadFacts(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Files) != 0 || len(got.Drafts) != 0 || got.CanMerge == nil || *got.CanMerge {
		t.Fatal("facts/permission incorrect")
	}
}

func TestFactsPaginationMustBeCompleteAndConsistent(t *testing.T) {
	for _, mode := range []string{"success", "cursor-missing", "cursor-repeat", "nested-notes", "reviewers", "diffs", "head-moved", "updated"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				result := factsFixture(t)
				mr := fixtureMR(result)
				d := mr["discussions"].(map[string]any)
				next := calls == 1
				cursor := "next"
				switch mode {
				case "cursor-missing":
					cursor = ""
				case "cursor-repeat":
					next = true
				case "nested-notes":
					d["nodes"].([]any)[0].(map[string]any)["notes"].(map[string]any)["pageInfo"] = map[string]any{"hasNextPage": true}
				case "reviewers":
					mr["reviewers"].(map[string]any)["pageInfo"] = map[string]any{"hasNextPage": true}
				case "diffs":
					mr["mergeRequestDiffs"].(map[string]any)["pageInfo"] = map[string]any{"hasNextPage": true}
				case "head-moved":
					if calls == 2 {
						mr["diffRefs"].(map[string]any)["headSha"] = "new"
					}
				case "updated":
					if calls == 2 {
						mr["updatedAt"] = "2027-01-01T00:00:00Z"
					}
				}
				if calls > 1 {
					d["nodes"] = []any{}
				}
				d["pageInfo"] = map[string]any{"hasNextPage": next, "endCursor": cursor}
				json.NewEncoder(w).Encode(result)
			}))
			defer srv.Close()
			c, ref := client(t, srv, secret)
			got, err := c.ReadFacts(context.Background(), ref)
			if mode == "success" {
				if err != nil || len(got.Threads) != 7 || calls != 2 {
					t.Fatalf("complete pagination: %v", err)
				}
			} else if err == nil || got != nil {
				t.Fatal("partial or mixed facts succeeded")
			}
		})
	}
}

func TestSystemEventsInMixedThreadsAreOrderedAndMarkBots(t *testing.T) {
	result := factsFixture(t)
	mr := fixtureMR(result)
	mr["discussions"].(map[string]any)["nodes"] = []any{
		map[string]any{"id": "thread", "notes": map[string]any{"nodes": []any{
			map[string]any{"author": map[string]any{"username": "robot", "bot": true}, "body": "unapproved this merge request", "system": true, "createdAt": "2026-10-02T02:00:00Z"},
			map[string]any{"author": map[string]any{"username": "human"}, "body": "reply", "createdAt": "2026-10-02T01:00:00Z"},
			map[string]any{"author": map[string]any{"username": "robot", "bot": true}, "body": "approved this merge request", "system": true, "createdAt": "2026-10-02T00:00:00Z"},
		}}},
	}
	b, _ := json.Marshal(result)
	var raw gqlResponse
	json.Unmarshal(b, &raw)
	got := convert(forge.Ref{}, raw.Data.Project.MergeRequest)
	if len(got.Threads) != 1 || len(got.Reviews) != 1 || !got.Reviews[0].Bot || !got.Reviews[0].Dismissed || !got.Reviews[0].At.Equal(time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("events %+v", got.Reviews)
	}
}

func TestGraphQLErrorsCannotEchoCredentials(t *testing.T) {
	for _, mode := range []string{"graphql-error", "invalid-timestamp", "identity"} {
		t.Run(mode, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				result := factsFixture(t)
				switch mode {
				case "graphql-error":
					result["errors"] = []any{map[string]any{"message": "echo " + secret}}
				case "invalid-timestamp":
					fixtureMR(result)["createdAt"] = secret
				case "identity":
					fixtureMR(result)["iid"] = "1"
				}
				json.NewEncoder(w).Encode(result)
			}))
			defer srv.Close()
			c, ref := client(t, srv, secret)
			_, err := c.ReadFacts(context.Background(), ref)
			if err == nil || strings.Contains(fmt.Sprintf("%+v %#v", err, err), secret) {
				t.Fatal("invalid facts accepted or credential leaked")
			}
		})
	}
}
