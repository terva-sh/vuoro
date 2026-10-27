package forgejo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
)

// memStore is a DraftStore in memory.
type memStore struct {
	mu     sync.Mutex
	drafts map[string][]forge.Draft
	fail   bool
}

func (m *memStore) Drafts(ref forge.Ref) ([]forge.Draft, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]forge.Draft(nil), m.drafts[ref.String()]...), nil
}
func (m *memStore) SaveDrafts(ref forge.Ref, ds []forge.Draft) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return errors.New("disk full")
	}
	if m.drafts == nil {
		m.drafts = map[string][]forge.Draft{}
	}
	m.drafts[ref.String()] = append([]forge.Draft(nil), ds...)
	return nil
}

type sent struct {
	Method, Path string
	Body         map[string]any
}

// writeForge is a Forgejo pull request, #2 of terva-sh/vuoro at head "head1",
// with one published review (11) holding a thread on a.go line 3, and, when
// pending is set, the signed-in person's own pending review (12) started in
// the web UI. It records every write, and fails those whose path is in fail.
type writeForge struct {
	t       *testing.T
	pending bool
	fail    map[string]bool
	mu      sync.Mutex
	writes  []sent
	// published are the reviews this fake has taken, as Forgejo lists them.
	published []string
	head      string // the PR's head, "head1" when empty
}

func (f *writeForge) serve(w http.ResponseWriter, r *http.Request) {
	root := "/api/v1/repos/terva-sh/vuoro"
	pr := root + "/pulls/2"
	if r.Method != http.MethodGet {
		b, _ := io.ReadAll(r.Body)
		var body map[string]any
		json.Unmarshal(b, &body)
		f.mu.Lock()
		f.writes = append(f.writes, sent{r.Method, r.URL.Path, body})
		f.mu.Unlock()
		if f.fail[r.URL.Path] {
			http.Error(w, `{"message":"boom"}`, 500)
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == pr+"/reviews" {
			f.mu.Lock()
			f.published = append(f.published, fmt.Sprintf(`{"id": %d, "user": {"login": "me"}, "state": %q, "commit_id": %q, "submitted_at": "2026-10-06T0%d:00:00Z"}`,
				100+len(f.published), body["event"], body["commit_id"], len(f.published)+1))
			f.mu.Unlock()
		}
		w.Write([]byte(`{"id": 99}`))
		return
	}
	switch r.URL.Path {
	case "/api/v1/user":
		w.Write([]byte(`{"login": "me"}`))
	case pr:
		f.mu.Lock()
		head := f.head
		f.mu.Unlock()
		if head == "" {
			head = "head1"
		}
		fmt.Fprintf(w, `{"head": {"ref": "feature", "sha": %q}}`, head)
	case pr + "/reviews":
		if r.URL.Query().Get("page") != "1" {
			w.Write([]byte(`[]`))
			return
		}
		reviews := []string{`{"id": 11, "user": {"login": "reviewer"}, "state": "COMMENT", "comments_count": 1}`}
		if f.pending {
			reviews = append(reviews, `{"id": 12, "user": {"login": "me"}, "state": "PENDING", "body": "Private summary.", "comments_count": 1}`)
		}
		f.mu.Lock()
		reviews = append(reviews, f.published...)
		f.mu.Unlock()
		w.Write([]byte("[" + strings.Join(reviews, ",") + "]"))
	case pr + "/reviews/12/comments":
		w.Write([]byte(`[{"id": 41, "pull_request_review_id": 12, "path": "a.go", "position": 4, "body": "Private inline."}]`))
	case pr + "/reviews/11/comments":
		w.Write([]byte(`[{"id": 31, "pull_request_review_id": 11, "path": "a.go", "position": 3, "original_position": 0, "body": "Why?"}]`))
	default:
		f.t.Errorf("unexpected read %s", r.URL)
		http.NotFound(w, r)
	}
}

func (f *writeForge) sent() []sent {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]sent(nil), f.writes...)
}

func writeClient(t *testing.T, f *writeForge) (*Client, forge.Ref, *memStore) {
	t.Helper()
	f.t = t
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	c, err := New(srv.URL, config.SecretForTest("synthetic-forgejo-token"))
	if err != nil {
		t.Fatal(err)
	}
	s := &memStore{}
	c.KeepDraftsIn(s)
	return c, forge.Ref{Host: c.base.Host, Project: "terva-sh/vuoro", Number: 2}, s
}

func TestCapabilitiesNeedATokenAndAStore(t *testing.T) {
	c, _, _ := writeClient(t, &writeForge{})
	both := []string{forge.UpdateMerge, forge.UpdateRebase}
	admin := forge.Capabilities{Reviewers: true, Assignees: true, Labels: true, SetDraft: true, UpdateBranch: both}
	want := forge.Capabilities{Drafts: true, Reply: true, Approve: true, RequestChanges: true, Reviewers: true, Assignees: true, Labels: true, SetDraft: true, UpdateBranch: both}
	if got := c.Capabilities(); !reflect.DeepEqual(got, want) {
		t.Fatalf("with a token and a store: %+v", got)
	}
	bare, _ := New("https://forge.example", config.SecretForTest("t"))
	// With no store, only project triage's writes, which need just the
	// token, are offered.
	if !reflect.DeepEqual(bare.Capabilities(), admin) {
		t.Fatal("a client with no store offers review writes")
	}
	if _, err := bare.CreateDraft(context.Background(), forge.Ref{}, forge.NewDraft{Body: "x"}); !errors.Is(err, forge.ErrUnsupported) {
		t.Fatalf("a draft with no store: %v", err)
	}
}

// Pending comments stay in vuoro, and Submit publishes them as one review
// with the verdict, then posts each reply into its thread.
func TestDraftsStayLocalUntilOneReviewPublishesThem(t *testing.T) {
	f := &writeForge{}
	c, ref, s := writeClient(t, f)
	ctx := context.Background()
	at := &forge.Anchor{Path: "a.go", NewLine: 5, Diff: forge.DiffRefs{Head: "head1"}}
	removed := &forge.Anchor{Path: "a.go", OldLine: 2, Diff: forge.DiffRefs{Head: "head1"}}
	inline, err := c.CreateDraft(ctx, ref, forge.NewDraft{Body: "inline", Anchor: at})
	if err != nil || !strings.HasPrefix(inline.ID, localPrefix) {
		t.Fatalf("%+v %v", inline, err)
	}
	must := func(_ forge.Draft, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(c.CreateDraft(ctx, ref, forge.NewDraft{Body: "on a removed line", Anchor: removed}))
	must(c.CreateDraft(ctx, ref, forge.NewDraft{Body: "first general"}))
	dropped, _ := c.CreateDraft(ctx, ref, forge.NewDraft{Body: "dropped"})
	must(c.CreateDraft(ctx, ref, forge.NewDraft{Body: "second general"}))
	must(c.CreateDraft(ctx, ref, forge.NewDraft{Body: "a reply on a line", ThreadID: "31"}))
	must(c.CreateDraft(ctx, ref, forge.NewDraft{Body: "a reply in general", ThreadID: "comment-5"}))
	if err := c.UpdateDraft(ctx, ref, inline.ID, "inline, edited"); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteDraft(ctx, ref, dropped.ID); err != nil {
		t.Fatal(err)
	}
	if w := f.sent(); len(w) != 0 {
		t.Fatalf("a pending comment reached Forgejo before Submit: %+v", w)
	}
	if ds, _ := s.Drafts(ref); len(ds) != 6 {
		t.Fatalf("%d drafts kept", len(ds))
	}

	if err := c.Submit(ctx, ref, forge.VerdictChangesRequested); err != nil {
		t.Fatal(err)
	}
	w := f.sent()
	if len(w) != 3 {
		t.Fatalf("writes %+v", w)
	}
	review := w[0]
	if review.Method != "POST" || review.Path != "/api/v1/repos/terva-sh/vuoro/pulls/2/reviews" || review.Body["event"] != "REQUEST_CHANGES" ||
		review.Body["commit_id"] != "head1" || review.Body["body"] != "first general\n\nsecond general" {
		t.Fatalf("review %+v", review)
	}
	cs := review.Body["comments"].([]any)
	if len(cs) != 2 {
		t.Fatalf("comments %+v", cs)
	}
	if c0 := cs[0].(map[string]any); c0["path"] != "a.go" || c0["new_position"] != 5.0 || c0["old_position"] != nil || c0["body"] != "inline, edited" {
		t.Fatalf("inline %+v", c0)
	}
	if c1 := cs[1].(map[string]any); c1["old_position"] != 2.0 || c1["new_position"] != nil {
		t.Fatalf("removed line %+v", c1)
	}
	if r := w[1]; r.Path != "/api/v1/repos/terva-sh/vuoro/pulls/2/reviews/11/comments" || r.Body["path"] != "a.go" || r.Body["new_position"] != 3.0 || r.Body["body"] != "a reply on a line" {
		t.Fatalf("line reply %+v", r)
	}
	if r := w[2]; r.Path != "/api/v1/repos/terva-sh/vuoro/issues/2/comments" || r.Body["body"] != "a reply in general" {
		t.Fatalf("general reply %+v", r)
	}
	if ds, _ := s.Drafts(ref); len(ds) != 0 {
		t.Fatalf("drafts left after submit: %+v", ds)
	}
}

// Nothing is written when Submit cannot publish what the person sees.
func TestSubmitRefusesBeforeWriting(t *testing.T) {
	ctx := context.Background()
	for _, c := range []struct {
		name    string
		drafts  []forge.NewDraft
		verdict forge.Verdict
		want    string
	}{
		{"an inline comment on an earlier head", []forge.NewDraft{{Body: "x", Anchor: &forge.Anchor{Path: "a.go", NewLine: 1, Diff: forge.DiffRefs{Head: "old"}}}}, forge.VerdictCommented, "earlier head"},
		{"request changes with no comment on the change", []forge.NewDraft{{Body: "x", Anchor: &forge.Anchor{Path: "a.go", NewLine: 1, Diff: forge.DiffRefs{Head: "head1"}}}}, forge.VerdictChangesRequested, "comment on the whole change"},
		{"nothing to submit", nil, forge.VerdictCommented, "no pending comments"},
	} {
		f := &writeForge{}
		cl, ref, _ := writeClient(t, f)
		for _, d := range c.drafts {
			if _, err := cl.CreateDraft(ctx, ref, d); err != nil {
				t.Fatal(err)
			}
		}
		if err := cl.Submit(ctx, ref, c.verdict); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v", c.name, err)
		}
		if w := f.sent(); len(w) != 0 {
			t.Errorf("%s: wrote %+v", c.name, w)
		}
	}
}

// A reply that fails stays pending and is named, and the review it followed
// is not published again on the next Submit.
func TestAFailedReplyStaysPending(t *testing.T) {
	f := &writeForge{fail: map[string]bool{"/api/v1/repos/terva-sh/vuoro/issues/2/comments": true}}
	c, ref, s := writeClient(t, f)
	ctx := context.Background()
	c.CreateDraft(ctx, ref, forge.NewDraft{Body: "general"})
	c.CreateDraft(ctx, ref, forge.NewDraft{Body: "line reply", ThreadID: "31"})
	failing, _ := c.CreateDraft(ctx, ref, forge.NewDraft{Body: "general reply", ThreadID: "comment-5"})
	another, _ := c.CreateDraft(ctx, ref, forge.NewDraft{Body: "another general reply", ThreadID: "comment-5"})
	err := c.Submit(ctx, ref, forge.VerdictCommented)
	// The error names each reply that failed, so the person can tell them apart.
	if err == nil || !strings.Contains(err.Error(), "2 of 3 replies") ||
		!strings.Contains(err.Error(), "your reply "+failing.ID+" to thread comment-5") ||
		!strings.Contains(err.Error(), "your reply "+another.ID+" to thread comment-5") {
		t.Fatalf("err %v", err)
	}
	if ds, _ := s.Drafts(ref); len(ds) != 2 || ds[0].ID != failing.ID || ds[1].ID != another.ID {
		t.Fatalf("pending after a partial submit: %+v", ds)
	}
	delete(f.fail, "/api/v1/repos/terva-sh/vuoro/issues/2/comments")
	if err := c.Submit(ctx, ref, forge.VerdictCommented); err != nil {
		t.Fatal(err)
	}
	reviews := 0
	for _, w := range f.sent() {
		if strings.HasSuffix(w.Path, "/pulls/2/reviews") {
			reviews++
		}
	}
	if reviews != 1 {
		t.Fatalf("the review was published %d times", reviews)
	}
}

// A reply that fails after a verdict was published is retried without
// publishing the verdict again, even after a push, and without asking again
// for what the published review already said.
func TestARetryAfterAVerdictPostsOnlyTheReplies(t *testing.T) {
	for _, v := range []forge.Verdict{forge.VerdictApproved, forge.VerdictChangesRequested} {
		f := &writeForge{fail: map[string]bool{"/api/v1/repos/terva-sh/vuoro/issues/2/comments": true}}
		c, ref, s := writeClient(t, f)
		ctx := context.Background()
		c.CreateDraft(ctx, ref, forge.NewDraft{Body: "general"})
		c.CreateDraft(ctx, ref, forge.NewDraft{Body: "general reply", ThreadID: "comment-5"})
		if err := c.Submit(ctx, ref, v); err == nil || !strings.Contains(err.Error(), "1 of 1 replies") {
			t.Fatalf("%s: %v", v, err)
		}
		delete(f.fail, "/api/v1/repos/terva-sh/vuoro/issues/2/comments")
		// The author pushes before the retry, so the published verdict no
		// longer stands on today's head.
		f.mu.Lock()
		f.head = "head2"
		f.mu.Unlock()
		if err := c.Submit(ctx, ref, v); err != nil {
			t.Fatalf("%s retry: %v", v, err)
		}
		reviews, replies := 0, 0
		for _, w := range f.sent() {
			switch {
			case strings.HasSuffix(w.Path, "/pulls/2/reviews"):
				reviews++
			case strings.HasSuffix(w.Path, "/issues/2/comments"):
				replies++
			}
		}
		if reviews != 1 || replies != 2 {
			t.Fatalf("%s: %d reviews and %d reply attempts", v, reviews, replies)
		}
		if ds, _ := s.Drafts(ref); len(ds) != 0 {
			t.Fatalf("%s: left %+v", v, ds)
		}
	}

	// Fresh replies with a verdict still publish the verdict.
	f := &writeForge{}
	c, ref, _ := writeClient(t, f)
	ctx := context.Background()
	c.CreateDraft(ctx, ref, forge.NewDraft{Body: "a reply", ThreadID: "comment-5"})
	if err := c.Submit(ctx, ref, forge.VerdictApproved); err != nil {
		t.Fatal(err)
	}
	if w := f.sent(); len(w) != 2 || w[0].Body["event"] != "APPROVED" {
		t.Fatalf("fresh replies with an approval: %+v", w)
	}
}

// A pending review the person started in Forgejo's web UI: its comments are
// edited and deleted on Forgejo, its summary is left alone and kept by
// Submit, and a bare approval is refused because it would publish it.
func TestAReviewStartedInForgejo(t *testing.T) {
	f := &writeForge{pending: true}
	c, ref, _ := writeClient(t, f)
	ctx := context.Background()
	if err := c.UpdateDraft(ctx, ref, "41", "edited on Forgejo"); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteDraft(ctx, ref, "41"); err != nil {
		t.Fatal(err)
	}
	// A published comment's id is not a pending comment, whatever route
	// would edit it.
	if err := c.UpdateDraft(ctx, ref, "31", "x"); !errors.Is(err, forge.ErrNotFound) {
		t.Fatalf("editing a published comment: %v", err)
	}
	if err := c.DeleteDraft(ctx, ref, "31"); !errors.Is(err, forge.ErrNotFound) {
		t.Fatalf("deleting a published comment: %v", err)
	}
	if err := c.UpdateDraft(ctx, ref, "review-12", "x"); !errors.Is(err, forge.ErrUnsupported) {
		t.Fatalf("editing the web summary: %v", err)
	}
	if err := c.DeleteDraft(ctx, ref, "../12"); err == nil {
		t.Fatal("a path was taken for a comment id")
	}
	if err := c.Approve(ctx, ref, true); err == nil || !strings.Contains(err.Error(), "Submit and approve") {
		t.Fatalf("a bare approval over a pending review: %v", err)
	}
	c.CreateDraft(ctx, ref, forge.NewDraft{Body: "mine"})
	if err := c.Submit(ctx, ref, forge.VerdictApproved); err != nil {
		t.Fatal(err)
	}
	w := f.sent()
	if len(w) != 3 || w[0].Method != "PATCH" || w[0].Path != "/api/v1/repos/terva-sh/vuoro/issues/comments/41" || w[0].Body["body"] != "edited on Forgejo" ||
		w[1].Method != "DELETE" || w[1].Path != "/api/v1/repos/terva-sh/vuoro/pulls/2/reviews/12/comments/41" {
		t.Fatalf("writes %+v", w)
	}
	if w[2].Body["event"] != "APPROVED" || w[2].Body["body"] != "Private summary.\n\nmine" {
		t.Fatalf("submit %+v", w[2])
	}
}

func TestApproveAndWhatForgejoCannotDo(t *testing.T) {
	f := &writeForge{}
	c, ref, _ := writeClient(t, f)
	ctx := context.Background()
	if err := c.Approve(ctx, ref, true); err != nil {
		t.Fatal(err)
	}
	if w := f.sent(); len(w) != 1 || w[0].Body["event"] != "APPROVED" || w[0].Body["commit_id"] != "head1" {
		t.Fatalf("approve %+v", w)
	}
	if err := c.Approve(ctx, ref, false); !errors.Is(err, forge.ErrUnsupported) {
		t.Fatalf("withdraw %v", err)
	}
	if err := c.Resolve(ctx, ref, "31", true); !errors.Is(err, forge.ErrUnsupported) {
		t.Fatalf("resolve %v", err)
	}
	if len(f.sent()) != 1 {
		t.Fatal("an unsupported action wrote")
	}
}
