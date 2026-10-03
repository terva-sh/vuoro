package forgejo

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func fixtureClient(t *testing.T, extra bool) (*Client, forge.Ref) {
	t.Helper()
	pr := fixture(t, "pr.json")
	fs := fixture(t, "files.json")
	diff := fixture(t, "pr.diff")
	timeline := fixture(t, "timeline.json")
	if extra {
		var events []apiEvent
		if err := json.Unmarshal(timeline, &events); err != nil {
			t.Fatal(err)
		}
		events = append(events, apiEvent{Type: "review_request", User: apiUser{Login: "author"}, Assignee: &apiUser{Login: "reviewer"}, Created: time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)})
		timeline, _ = json.Marshal(events)
	}
	var host string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("reader attempted %s", r.Method)
			w.WriteHeader(405)
			return
		}
		root := "/api/v1/repos/terva-sh/vuoro/pulls/2"
		switch r.URL.Path {
		case root:
			body := strings.ReplaceAll(string(pr), "ssh://git@forge.example:2222", "ssh://git@"+strings.Split(host, ":")[0]+":2222")
			w.Write([]byte(strings.ReplaceAll(body, "forge.example", host)))
		case root + ".diff":
			w.Write(diff)
		case root + "/files":
			// Deliberately clamp to one item per page despite the requested limit.
			var files []apiFile
			_ = json.Unmarshal(fs, &files)
			page := r.URL.Query().Get("page")
			switch page {
			case "1":
				json.NewEncoder(w).Encode(files[:1])
			case "2":
				json.NewEncoder(w).Encode(files[1:])
			default:
				w.Write([]byte("[]"))
			}
		case root + "/reviews":
			if r.URL.Query().Get("page") != "1" || !extra {
				w.Write([]byte("[]"))
				return
			}
			w.Write([]byte(`[
    {"id":10,"user":{"login":"reviewer"},"state":"APPROVED","commit_id":"old","dismissed":true,"stale":true,"submitted_at":"2026-10-02T10:00:00Z"},
    {"id":11,"user":{"login":"reviewer"},"state":"REQUEST_CHANGES","body":"Please fix this.","commit_id":"68ff96c709bfbd9d96709b4b976bf4c77f131148","comments_count":3,"submitted_at":"2026-10-02T11:00:00Z"},
    {"id":12,"user":{"login":"me"},"state":"PENDING","body":"Private summary.","comments_count":1}
   ]`))
		case root + "/reviews/11/comments":
			if r.URL.Query().Get("page") != "1" {
				w.Write([]byte("[]"))
				return
			}
			w.Write([]byte(`[
    {"id":21,"pull_request_review_id":11,"user":{"login":"reviewer"},"path":"README.md","position":3,"original_position":0,"commit_id":"head","original_commit_id":"base","body":"Fix this line.","resolver":{"login":"reviewer"},"created_at":"2026-10-02T11:00:00Z"},
    {"id":22,"pull_request_review_id":11,"user":{"login":"author"},"path":"README.md","position":3,"original_position":0,"commit_id":"head","original_commit_id":"base","body":"Done.","created_at":"2026-10-02T12:00:00Z"},
    {"id":23,"pull_request_review_id":11,"user":{"login":"reviewer"},"path":"README.md","position":0,"original_position":7,"commit_id":"head","original_commit_id":"base","body":"Removed line.","created_at":"2026-10-02T11:01:00Z"}
   ]`))
		case root + "/reviews/12/comments":
			if r.URL.Query().Get("page") != "1" {
				w.Write([]byte("[]"))
				return
			}
			w.Write([]byte(`[{"id":24,"pull_request_review_id":12,"user":{"login":"me"},"path":"README.md","position":3,"body":"Private inline."}]`))
		case "/api/v1/repos/terva-sh/vuoro/issues/2/timeline":
			if r.URL.Query().Get("page") != "1" {
				w.Write([]byte("[]"))
				return
			}
			w.Write(timeline)
		default:
			t.Errorf("unexpected request %s", r.URL)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)
	host = strings.TrimPrefix(srv.URL, "http://")
	c, err := New(srv.URL, config.Secret{})
	if err != nil {
		t.Fatal(err)
	}
	return c, forge.Ref{Host: host, Project: "terva-sh/vuoro", Number: 2}
}
func TestRecordedPullRequest(t *testing.T) {
	c, ref := fixtureClient(t, false)
	ch, err := c.Open(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	if ch.State != forge.Merged || ch.Kind != forge.Forgejo || ch.Diff.Head != "68ff96c709bfbd9d96709b4b976bf4c77f131148" || ch.Diff.Base == "" {
		t.Fatalf("bad metadata: %+v", ch)
	}
	if ch.Checkout.HeadRef != "refs/pull/2/head" || !strings.Contains(ch.Checkout.SSH, ":2222/") {
		t.Fatalf("checkout: %+v", ch.Checkout)
	}
	if len(ch.Files) != 2 || ch.Files[0].Omitted || !strings.Contains(ch.Files[0].Patch, "just design-export") {
		t.Fatalf("files: %+v", ch.Files)
	}
	if len(ch.Pushes) != 3 || ch.Pushes[2].Head != ch.Diff.Head || len(ch.Threads) != 1 || ch.Threads[0].Notes[0].System {
		t.Fatalf("timeline: %+v %+v", ch.Pushes, ch.Threads)
	}
	if ch.Capabilities != (forge.Capabilities{}) {
		t.Fatal("reader exposes writes")
	}
	if _, ok := any(c).(forge.Writer); ok {
		t.Fatal("Forgejo implements Writer")
	}
}
func TestReviewStateThreadsAndPrivateDrafts(t *testing.T) {
	c, ref := fixtureClient(t, true)
	ch, err := c.Open(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	if len(ch.Reviews) != 2 || !ch.Reviews[0].Stale || !ch.Reviews[0].Dismissed || ch.Reviewers[0].State != forge.RequestedChanges {
		t.Fatalf("reviews: %+v %+v", ch.Reviews, ch.Reviewers)
	}
	var inline []forge.Thread
	for _, th := range ch.Threads {
		if th.Anchor != nil {
			inline = append(inline, th)
		}
	}
	if len(inline) != 2 || len(inline[0].Notes) != 2 || !inline[0].Resolved || inline[0].Anchor.NewLine != 3 || inline[1].Anchor.OldLine != 7 || inline[1].Anchor.NewLine != 0 {
		t.Fatalf("threads: %+v", inline)
	}
	if inline[0].Anchor.Diff.Head != "head" || inline[0].Anchor.Diff.Base != "base" {
		t.Fatalf("anchor: %+v", inline[0].Anchor)
	}
	if len(ch.Drafts) != 2 || ch.Drafts[1].Body != "Private inline." {
		t.Fatalf("drafts: %+v", ch.Drafts)
	}
	for _, th := range ch.Threads {
		for _, n := range th.Notes {
			if strings.Contains(n.Body, "Private") {
				t.Fatal("pending review became a published thread")
			}
		}
	}
}
func TestTokensAndRedirectsStayOnTheConfiguredHost(t *testing.T) {
	t.Setenv("VUORO_TEST_FORGEJO_SECRET", "test-forge-secret")
	token, err := (&config.Forge{TokenEnv: "VUORO_TEST_FORGEJO_SECRET"}).Token(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	hit := false
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit = true }))
	defer other.Close()
	for _, redirect := range []bool{false, true} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "token test-forge-secret" {
				t.Error("missing authentication header")
			}
			if redirect {
				http.Redirect(w, r, other.URL, 302)
				return
			}
			w.WriteHeader(401)
			json.NewEncoder(w).Encode(map[string]string{"message": "echo test-forge-secret"})
		}))
		c, _ := New(srv.URL, token)
		ref := forge.Ref{Host: strings.TrimPrefix(srv.URL, "http://"), Project: "o/r", Number: 1}
		_, err = c.Open(context.Background(), ref)
		if err == nil || strings.Contains(err.Error(), "test-forge-secret") {
			t.Fatal("error missing or contains a credential")
		}
		srv.Close()
	}
	if hit {
		t.Fatal("redirect destination was reached")
	}
	c, _ := New("https://forge.example", token)
	_, err = c.Open(context.Background(), forge.Ref{Host: "elsewhere.example", Project: "o/r", Number: 1})
	if err == nil {
		t.Fatal("cross-host reference accepted")
	}
}
func TestNotFoundAndMalformedResponses(t *testing.T) {
	for _, status := range []int{404, 200} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status); w.Write([]byte("not json")) }))
		c, _ := New(srv.URL, config.Secret{})
		_, err := c.Open(context.Background(), forge.Ref{Host: strings.TrimPrefix(srv.URL, "http://"), Project: "o/r", Number: 1})
		if err == nil {
			t.Fatal("invalid response accepted")
		}
		if status == 404 && !errors.Is(err, forge.ErrNotFound) {
			t.Fatalf("not-found not classified: %v", err)
		}
		srv.Close()
	}
}
func TestTimelineFacts(t *testing.T) {
	c, ref := fixtureClient(t, false)
	// A second fixture transport exercises a re-request independently of reviews.
	c.http.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		body := "[]"
		if strings.HasSuffix(r.URL.Path, "/timeline") && r.URL.Query().Get("page") == "1" {
			body = `[{"type":"review_request","user":{"login":"author"},"assignee":{"login":"reviewer"},"created_at":"2026-10-02T13:00:00Z"}]`
		} else {
			return http.DefaultTransport.RoundTrip(r)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})
	ch, err := c.Open(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	if len(ch.Requests) != 1 || ch.Requests[0].Reviewer != "reviewer" || ch.Requests[0].By != "author" || ch.Requests[0].At != time.Date(2026, 10, 2, 13, 0, 0, 0, time.UTC) {
		t.Fatalf("requests: %+v", ch.Requests)
	}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestOutstandingReviewRequests(t *testing.T) {
	for _, tc := range []struct {
		name      string
		requestAt []int
		removed   bool
		stale     bool
		want      forge.ReviewState
	}{
		{name: "same-head re-request after approval", requestAt: []int{13, 9}, want: forge.Unreviewed},
		{name: "approval after request", requestAt: []int{9}, want: forge.Approved},
		{name: "request without timeline timestamp", want: forge.Unreviewed},
		{name: "removed request is only history", requestAt: []int{13}, removed: true, want: forge.Approved},
		{name: "stale review cannot answer request", requestAt: []int{9}, stale: true, want: forge.Unreviewed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, ref := fixtureClient(t, false)
			c.http.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
				var body []byte
				switch {
				case strings.HasSuffix(r.URL.Path, "/reviews"):
					body = []byte("[]")
					if r.URL.Query().Get("page") == "1" {
						body, _ = json.Marshal([]apiReview{{ID: 10, User: apiUser{Login: "reviewer"}, State: "APPROVED", Commit: "68ff96c709bfbd9d96709b4b976bf4c77f131148", Stale: tc.stale, Submitted: time.Date(2026, 10, 2, 11, 0, 0, 0, time.UTC)}})
					}
				case strings.HasSuffix(r.URL.Path, "/timeline"):
					body = []byte("[]")
					if r.URL.Query().Get("page") == "1" && len(tc.requestAt) > 0 {
						var events []apiEvent
						for _, hour := range tc.requestAt {
							events = append(events, apiEvent{Type: "review_request", User: apiUser{Login: "author"}, Assignee: &apiUser{Login: "reviewer"}, Created: time.Date(2026, 10, 2, hour, 0, 0, 0, time.UTC)})
						}
						body, _ = json.Marshal(events)
					}
				default:
					res, err := http.DefaultTransport.RoundTrip(r)
					if err != nil || !tc.removed || r.URL.Path != "/api/v1/repos/terva-sh/vuoro/pulls/2" {
						return res, err
					}
					defer res.Body.Close()
					var pr apiPR
					if err := json.NewDecoder(res.Body).Decode(&pr); err != nil {
						return nil, err
					}
					pr.Requested = nil
					body, _ = json.Marshal(pr)
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(body))), Header: http.Header{}}, nil
			})
			ch, err := c.Open(context.Background(), ref)
			if err != nil {
				t.Fatal(err)
			}
			if len(ch.Reviewers) != 1 || ch.Reviewers[0].State != tc.want {
				t.Fatalf("reviewers = %+v; want %s", ch.Reviewers, tc.want)
			}
			if len(ch.Reviews) != 1 || ch.Reviews[0].Verdict != forge.VerdictApproved {
				t.Fatalf("historical approval lost: %+v", ch.Reviews)
			}
		})
	}
}

func TestExcludedReviewsRetainCurrentVerdicts(t *testing.T) {
	for _, verdict := range []struct {
		state string
		want  forge.ReviewState
	}{{"APPROVED", forge.Approved}, {"REQUEST_CHANGES", forge.RequestedChanges}} {
		for _, excluded := range []string{"dismissed", "stale", "different-head"} {
			t.Run(verdict.state+"/"+excluded, func(t *testing.T) {
				c, ref := fixtureClient(t, true)
				c.http.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
					if !strings.HasSuffix(r.URL.Path, "/reviews") {
						return http.DefaultTransport.RoundTrip(r)
					}
					body := []byte("[]")
					if r.URL.Query().Get("page") == "1" {
						valid := apiReview{ID: 10, User: apiUser{Login: "reviewer"}, State: verdict.state, Commit: "68ff96c709bfbd9d96709b4b976bf4c77f131148", Submitted: time.Date(2026, 10, 2, 11, 0, 0, 0, time.UTC)}
						ignored := apiReview{ID: 11, User: valid.User, State: "COMMENT", Commit: valid.Commit, Dismissed: excluded == "dismissed", Stale: excluded == "stale", Submitted: valid.Submitted.Add(time.Hour)}
						if excluded == "different-head" {
							ignored.Commit = "old"
						}
						body, _ = json.Marshal([]apiReview{valid, ignored})
					}
					return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(body))), Header: http.Header{}}, nil
				})
				ch, err := c.Open(context.Background(), ref)
				if err != nil {
					t.Fatal(err)
				}
				if len(ch.Reviewers) != 1 || ch.Reviewers[0].State != verdict.want || len(ch.Reviews) != 2 {
					t.Fatalf("reviewers = %+v; want %s with both reviews retained: %+v", ch.Reviewers, verdict.want, ch.Reviews)
				}
			})
		}
	}
}

func TestAPushDuringTheReadCannotMixHeadsAndPatches(t *testing.T) {
	c, ref := fixtureClient(t, false)
	reads := 0
	c.http.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		res, err := http.DefaultTransport.RoundTrip(r)
		if err != nil {
			return res, err
		}
		if r.URL.Path == "/api/v1/repos/terva-sh/vuoro/pulls/2" {
			reads++
			if reads > 1 {
				b, err := io.ReadAll(res.Body)
				res.Body.Close()
				if err != nil {
					return nil, err
				}
				var p apiPR
				json.Unmarshal(b, &p)
				p.Head.SHA = "moved"
				b, _ = json.Marshal(p)
				res.Body = io.NopCloser(strings.NewReader(string(b)))
			}
		}
		return res, nil
	})
	_, err := c.Open(context.Background(), ref)
	if err == nil || !strings.Contains(err.Error(), "moved while reading") {
		t.Fatal("mixed-head snapshot accepted")
	}
}
