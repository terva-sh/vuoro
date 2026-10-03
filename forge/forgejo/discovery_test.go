package forgejo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/turn"
)

func response(body string) *http.Response {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}
}

func TestDiscoveryCombinesRolesAndProvenHistoricalRequests(t *testing.T) {
	c, ref := fixtureClient(t, false)
	c.token = config.SecretForTest("synthetic-discovery-secret")
	searches := 0
	c.http.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.Method != "GET" {
			t.Fatal("discovery attempted a write")
		}
		switch {
		case r.URL.Path == "/api/v1/user":
			return response(`{"login":"reviewer"}`), nil
		case r.URL.Path == "/api/v1/repos/issues/search":
			searches++
			q := r.URL.Query()
			if q.Get("state") != "open" || q.Get("type") != "pulls" || q.Has("since") {
				t.Fatal("incomplete discovery query")
			}
			page, _ := strconv.Atoi(q.Get("page"))
			numbers := []int{2}
			if q.Get("reviewed") == "true" {
				numbers = []int{2, 3, 4, 5}
			}
			if page > len(numbers) {
				return response(`[]`), nil
			}
			n := numbers[page-1]
			return response(fmt.Sprintf(`[{"number":%d,"html_url":%q,"state":"open","pull_request":{},"repository":{"full_name":"terva-sh/vuoro"},"updated_at":"2026-10-02T00:00:00Z"}]`, n, "https://"+ref.Host+"/terva-sh/vuoro/pulls/"+strconv.Itoa(n))), nil
		case strings.Contains(r.URL.Path, "/pulls/"):
			if strings.HasSuffix(r.URL.Path, "/reviews") {
				if r.URL.Query().Get("page") != "1" {
					return response(`[]`), nil
				}
				return response(`[{"id":1,"user":{"login":"reviewer"},"state":"APPROVED","commit_id":"head","submitted_at":"2026-10-02T01:00:00Z"}]`), nil
			}
			var p apiPR
			json.Unmarshal(fixture(t, "pr.json"), &p)
			p.State = "open"
			p.Merged = false
			p.Requested = nil
			p.Head.SHA = "head"
			p.Base.Repo.CloneURL = "https://" + ref.Host + "/terva-sh/vuoro.git"
			p.Base.Repo.SSHURL = "ssh://git@" + strings.Split(ref.Host, ":")[0] + ":2222/terva-sh/vuoro.git"
			b, _ := json.Marshal(p)
			return response(string(b)), nil
		case strings.HasSuffix(r.URL.Path, "/timeline"):
			if r.URL.Query().Get("page") != "1" || strings.Contains(r.URL.Path, "/issues/4/") {
				return response(`[]`), nil
			}
			rows := []apiEvent{{Type: "review_request", Assignee: &apiUser{Login: "reviewer"}, Created: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)}}
			if strings.Contains(r.URL.Path, "/issues/3/") {
				rows = append(rows, apiEvent{Type: "pull_push", Body: `{ "commit_ids": ["head"] }`, Created: rows[0].Created.Add(2 * time.Hour)})
			}
			if strings.Contains(r.URL.Path, "/issues/5/") {
				rows = append(rows, apiEvent{Type: "review_request", Assignee: &apiUser{Login: "reviewer"}, RemovedAssignee: true, Created: rows[0].Created.Add(time.Hour)})
			}
			b, _ := json.Marshal(rows)
			return response(string(b)), nil
		}
		return nil, fmt.Errorf("unexpected request")
	})
	got, err := c.Discover(context.Background(), "reviewer")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || searches != 11 {
		t.Fatalf("members=%+v searches=%d", got, searches)
	}
	if !slices.Equal(got[0].Roles, []forge.Role{forge.AuthorRole, forge.ReviewerRole, forge.AssigneeRole}) || got[1].Ref.Number != 3 || !slices.Equal(got[1].Roles, []forge.Role{forge.ReviewerRole}) {
		t.Fatalf("roles %+v", got)
	}
	ch, err := c.ReadFacts(context.Background(), got[1].Ref)
	if err != nil {
		t.Fatal(err)
	}
	derived := turn.Derive(*ch, "reviewer", got[1].Roles)
	if derived.Owner != turn.Mine || !slices.ContainsFunc(derived.Reasons, func(r turn.Reason) bool { return r.Rule == "R2" }) {
		t.Fatalf("historical requested review lost its new-push turn: %+v", derived)
	}
}

func TestDiscoveryRejectsWrongAccountsPartialPagesAndCrossHostRefs(t *testing.T) {
	for _, mode := range []string{"account", "late-error", "foreign", "null", "redirect"} {
		t.Run(mode, func(t *testing.T) {
			var srv *httptest.Server
			srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "token fake-secret" {
					t.Error("missing account credential")
				}
				if r.URL.Path == "/api/v1/user" {
					login := "me"
					if mode == "account" {
						login = "other"
					}
					fmt.Fprintf(w, `{"login":%q}`, login)
					return
				}
				if mode == "redirect" {
					http.Redirect(w, r, "https://elsewhere.test", 302)
					return
				}
				if mode == "null" {
					fmt.Fprint(w, `null`)
					return
				}
				if r.URL.Query().Get("page") == "2" {
					w.WriteHeader(429)
					fmt.Fprint(w, `{"message":"echo fake-secret"}`)
					return
				}
				target := strings.Replace(srv.URL, "http://", "https://", 1) + "/team/repo/pulls/1"
				if mode == "foreign" {
					target = "https://elsewhere.test/team/repo/pulls/1"
				}
				fmt.Fprintf(w, `[{"number":1,"html_url":%q,"state":"open","pull_request":{},"repository":{"full_name":"team/repo"}}]`, target)
			}))
			defer srv.Close()
			c, _ := New(srv.URL, config.SecretForTest("fake-secret"))
			got, err := c.Discover(context.Background(), "me")
			if err == nil || got != nil || strings.Contains(fmt.Sprintf("%+v %#v", err, err), "fake-secret") {
				t.Fatal("partial or unsafe discovery succeeded")
			}
		})
	}
}

func TestFactsReaderOmitsPrivateDraftsAndPatches(t *testing.T) {
	c, ref := fixtureClient(t, true)
	c.http.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/files") || strings.HasSuffix(r.URL.Path, ".diff") || strings.Contains(r.URL.Path, "/reviews/12/") {
			t.Fatal("facts downloaded patches or pending comments")
		}
		return http.DefaultTransport.RoundTrip(r)
	})
	ch, err := c.ReadFacts(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	if len(ch.Files) != 0 || len(ch.Drafts) != 0 || ch.CanMerge != nil || ch.Checkout.HeadRef != "refs/pull/2/head" || !strings.Contains(ch.Checkout.SSH, ":2222/") || len(ch.Reviews) != 2 {
		t.Fatal("facts/checkout incorrect")
	}
}

func TestLatestExcludedVerdictCannotResurrectEarlierApproval(t *testing.T) {
	ch := &forge.Change{State: forge.Open, Author: forge.User{Username: "author"}, Diff: forge.DiffRefs{Head: "head"}, Mergeability: "mergeable"}
	allowed := true
	ch.CanMerge = &allowed
	at := time.Date(2026, 10, 2, 1, 0, 0, 0, time.UTC)
	addReview(ch, apiReview{User: apiUser{Login: "reviewer"}, State: "APPROVED", Commit: "head", Submitted: at})
	addReview(ch, apiReview{User: apiUser{Login: "reviewer"}, State: "REQUEST_CHANGES", Commit: "head", Dismissed: true, Submitted: at.Add(time.Hour)})
	got := turn.Derive(*ch, "author", []forge.Role{forge.AuthorRole})
	if got.Owner == turn.Mine {
		t.Fatalf("excluded latest verdict resurrected approval: %+v", got)
	}
}

func TestRequestReviewRecordsAndRemovals(t *testing.T) {
	for _, removed := range []bool{false, true} {
		t.Run(fmt.Sprint(removed), func(t *testing.T) {
			c, ref := fixtureClient(t, false)
			c.http.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
				if strings.HasSuffix(r.URL.Path, "/reviews") {
					if r.URL.Query().Get("page") != "1" {
						return response(`[]`), nil
					}
					return response(`[{"id":1,"user":{"login":"extra"},"state":"REQUEST_REVIEW","submitted_at":"2026-10-02T01:00:00Z"}]`), nil
				}
				if strings.HasSuffix(r.URL.Path, "/timeline") {
					if r.URL.Query().Get("page") != "1" || !removed {
						return response(`[]`), nil
					}
					return response(`[{"type":"review_request","assignee":{"login":"extra"},"removed_assignee":true,"created_at":"2026-10-02T02:00:00Z"}]`), nil
				}
				return http.DefaultTransport.RoundTrip(r)
			})
			ch, err := c.ReadFacts(context.Background(), ref)
			if err != nil {
				t.Fatal(err)
			}
			has := slices.ContainsFunc(ch.Reviewers, func(r forge.Reviewer) bool { return r.Username == "extra" })
			if has == removed || len(ch.Requests) != 1 {
				t.Fatalf("requests/removals: %+v %+v", ch.Reviewers, ch.Requests)
			}
		})
	}
}

func TestNullTimelineIsEmptyButNullReviewsAreMalformed(t *testing.T) {
	for _, endpoint := range []string{"timeline", "reviews"} {
		t.Run(endpoint, func(t *testing.T) {
			c, ref := fixtureClient(t, false)
			c.http.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
				if strings.HasSuffix(r.URL.Path, "/"+endpoint) {
					return response(`null`), nil
				}
				return http.DefaultTransport.RoundTrip(r)
			})
			ch, err := c.ReadFacts(context.Background(), ref)
			if endpoint == "timeline" {
				if err != nil || len(ch.Threads) != 0 || len(ch.Pushes) != 0 {
					t.Fatal("empty timeline rejected")
				}
			} else if err == nil {
				t.Fatal("null review list accepted")
			}
		})
	}
}

func TestReviewCommentsAreACompleteListNotPaginated(t *testing.T) {
	c, ref := fixtureClient(t, true)
	reads := 0
	c.http.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "/reviews/") && strings.HasSuffix(r.URL.Path, "/comments") {
			reads++
			if r.URL.Query().Has("page") || r.URL.Query().Has("limit") {
				t.Fatal("non-paginated endpoint received page parameters")
			}
		}
		return http.DefaultTransport.RoundTrip(r)
	})
	ch, err := c.ReadFacts(context.Background(), ref)
	if err != nil || reads != 1 || len(ch.Threads) != 4 {
		t.Fatalf("complete comments: %d reads, %v", reads, err)
	}
}

func TestCrossSourceRerequestAfterRemovalSurvives(t *testing.T) {
	for _, requestTime := range []string{"2026-10-02T03:00:00Z", "0001-01-01T00:00:00Z"} {
		t.Run(requestTime, func(t *testing.T) {
			c, ref := fixtureClient(t, false)
			c.http.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
				if strings.HasSuffix(r.URL.Path, "/reviews") {
					if r.URL.Query().Get("page") != "1" {
						return response(`[]`), nil
					}
					return response(fmt.Sprintf(`[{"id":1,"user":{"login":"extra"},"state":"REQUEST_REVIEW","submitted_at":%q}]`, requestTime)), nil
				}
				if strings.HasSuffix(r.URL.Path, "/timeline") {
					if r.URL.Query().Get("page") != "1" {
						return response(`null`), nil
					}
					return response(`[{"type":"review_request","assignee":{"login":"extra"},"removed_assignee":true,"created_at":"2026-10-02T02:00:00Z"}]`), nil
				}
				return http.DefaultTransport.RoundTrip(r)
			})
			ch, err := c.ReadFacts(context.Background(), ref)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.ContainsFunc(ch.Reviewers, func(r forge.Reviewer) bool { return r.Username == "extra" }) {
				t.Fatal("older removal erased a new or undated request")
			}
		})
	}
}
