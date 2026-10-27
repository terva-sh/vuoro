package github

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
)

const head = "1111111111111111111111111111111111111111"

// memStore is a DraftStore in memory.
type memStore struct {
	mu sync.Mutex
	m  map[string][]forge.Draft
}

func (s *memStore) Drafts(ref forge.Ref) ([]forge.Draft, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.m[ref.String()]), nil
}

func (s *memStore) SaveDrafts(ref forge.Ref, ds []forge.Draft) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m == nil {
		s.m = map[string][]forge.Draft{}
	}
	s.m[ref.String()] = slices.Clone(ds)
	return nil
}

type op struct {
	name string
	vars map[string]any
}

// github is a GitHub that keeps one person's pending review on one pull
// request, as the spike of 2026-10-08 showed it behaves.
type github struct {
	t       *testing.T
	mu      sync.Mutex
	ops     []op // every mutation, in order
	review  string
	body    string
	at      string   // the pending review's commit, the head when empty
	pending []string // the pending review's comment ids
	latest  map[string]any
	// publish, when set, makes GitHub answer a comment as published.
	publish bool
	// refuse, when set, makes GitHub answer a comment with null.
	refuse bool
	// submitted, when set, submits the review on GitHub after vuoro reads it
	// and before an edit or a delete arrives.
	submitted bool
	next      int
}

func (g *github) id(prefix string) string {
	g.next++
	return prefix + string(rune('a'+g.next))
}

func (g *github) handle(q string, vars map[string]any) any {
	g.mu.Lock()
	defer g.mu.Unlock()
	data := func(v map[string]any) any { return map[string]any{"data": v} }
	if !strings.HasPrefix(strings.TrimSpace(q), "mutation") {
		var reviews []any
		if g.review != "" {
			var cs []any
			for _, c := range g.pending {
				cs = append(cs, map[string]any{"id": c})
			}
			at := g.at
			if at == "" {
				at = head
			}
			reviews = append(reviews, map[string]any{"id": g.review, "body": g.body, "viewerDidAuthor": true, "commit": map[string]any{"oid": at}, "comments": map[string]any{"pageInfo": map[string]any{}, "nodes": cs}})
		}
		return data(map[string]any{"repository": map[string]any{"pullRequest": map[string]any{"id": "PR_1", "headRefOid": head, "viewerLatestReview": g.latest, "reviews": map[string]any{"nodes": reviews}}}})
	}
	name := strings.Fields(strings.SplitN(strings.SplitN(q, "{", 2)[1], "(", 2)[0])[0]
	g.ops = append(g.ops, op{name, vars})
	comment := func() any {
		if g.refuse {
			return nil
		}
		id := g.id("PRRC_")
		state, review := "PENDING", g.review
		if g.publish {
			state, review = "SUBMITTED", g.id("PRR_")
		} else {
			g.pending = append(g.pending, id)
		}
		return map[string]any{"id": id, "state": state, "body": vars["body"], "pullRequestReview": map[string]any{"id": review}}
	}
	switch name {
	case "addPullRequestReview":
		if vars["event"] != nil || strings.Contains(q, "event: APPROVE") {
			state := map[any]string{"APPROVE": "APPROVED", "REQUEST_CHANGES": "CHANGES_REQUESTED", "COMMENT": "COMMENTED", nil: "APPROVED"}[vars["event"]]
			return data(map[string]any{name: map[string]any{"pullRequestReview": map[string]any{"state": state}}})
		}
		if g.review != "" {
			return map[string]any{"data": map[string]any{name: nil}, "errors": []any{map[string]any{"type": "UNPROCESSABLE", "message": "User can only have one pending review per pull request"}}}
		}
		g.review = g.id("PRR_")
		return data(map[string]any{name: map[string]any{"pullRequestReview": map[string]any{"id": g.review, "state": "PENDING"}}})
	case "addPullRequestReviewThread":
		c := comment()
		if c == nil {
			return data(map[string]any{name: map[string]any{"thread": nil}})
		}
		return data(map[string]any{name: map[string]any{"thread": map[string]any{"comments": map[string]any{"nodes": []any{c}}}}})
	case "addPullRequestReviewThreadReply":
		return data(map[string]any{name: map[string]any{"comment": comment()}})
	case "updatePullRequestReviewComment", "deletePullRequestReviewComment":
		state := "PENDING"
		if g.submitted {
			state = "SUBMITTED"
		}
		if name == "deletePullRequestReviewComment" {
			g.pending = slices.DeleteFunc(g.pending, func(c string) bool { return c == vars["id"] })
		}
		return data(map[string]any{name: map[string]any{"pullRequestReviewComment": map[string]any{"id": vars["id"], "state": state, "body": vars["body"], "pullRequestReview": map[string]any{"id": g.review}}}})
	case "submitPullRequestReview":
		g.review, g.pending, g.body = "", nil, ""
		return data(map[string]any{name: map[string]any{"pullRequestReview": map[string]any{"state": "COMMENTED"}}})
	case "addComment":
		if g.refuse {
			return data(map[string]any{name: nil})
		}
		return data(map[string]any{name: map[string]any{"commentEdge": map[string]any{"node": map[string]any{"id": g.id("IC_")}}}})
	case "resolveReviewThread", "unresolveReviewThread":
		return data(map[string]any{name: map[string]any{"thread": map[string]any{"isResolved": name == "resolveReviewThread"}}})
	}
	g.t.Errorf("unexpected mutation %s", name)
	return data(map[string]any{})
}

func (g *github) names() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	var out []string
	for _, o := range g.ops {
		out = append(out, o.name)
	}
	return out
}

var ref = forge.Ref{Host: "", Project: "owner/repo", Number: 1}

func writer(t *testing.T) (*Client, *github, *memStore, forge.Ref) {
	t.Helper()
	g := &github{t: t}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &req)
		json.NewEncoder(w).Encode(g.handle(req.Query, req.Variables))
	}))
	t.Cleanup(srv.Close)
	c, err := New(srv.URL, config.SecretForTest("sentinel-token"))
	if err != nil {
		t.Fatal(err)
	}
	s := &memStore{}
	c.KeepDraftsIn(s)
	r := ref
	r.Host = strings.TrimPrefix(srv.URL, "http://")
	return c, g, s, r
}

var line = &forge.Anchor{Path: "calc.go", OldPath: "calc.go", NewLine: 3, Diff: forge.DiffRefs{Head: head}}

// A comment or reply that names no pending review is published at once, so
// every one names the person's pending review, starting it when there is
// none.
func TestEveryCommentNamesThePendingReview(t *testing.T) {
	c, g, _, ref := writer(t)
	ctx := context.Background()
	d, err := c.CreateDraft(ctx, ref, forge.NewDraft{Body: "Inline.", Anchor: line})
	if err != nil || d.Anchor != line || d.ID == "" {
		t.Fatalf("inline %+v %v", d, err)
	}
	r, err := c.CreateDraft(ctx, ref, forge.NewDraft{Body: "Reply.", ThreadID: "PRRT_x"})
	if err != nil || r.ThreadID != "PRRT_x" {
		t.Fatalf("reply %+v %v", r, err)
	}
	if got := strings.Join(g.names(), " "); got != "addPullRequestReview addPullRequestReviewThread addPullRequestReviewThreadReply" {
		t.Fatalf("mutations %s", got)
	}
	for _, o := range g.ops[1:] {
		if o.vars["review"] != g.review || g.review == "" {
			t.Fatalf("%s named review %v, not the pending %s", o.name, o.vars["review"], g.review)
		}
	}
	if o := g.ops[0]; o.vars["head"] != head {
		t.Fatalf("the pending review started at %v", o.vars["head"])
	}
	if o := g.ops[1]; o.vars["line"] != float64(3) || o.vars["side"] != "RIGHT" || o.vars["path"] != "calc.go" {
		t.Fatalf("inline at %v", o.vars)
	}
	// A removed line is placed on the old side.
	if _, err := c.CreateDraft(ctx, ref, forge.NewDraft{Body: "Old.", Anchor: &forge.Anchor{Path: "calc.go", OldLine: 7}}); err != nil {
		t.Fatal(err)
	}
	if o := g.ops[len(g.ops)-1]; o.vars["line"] != float64(7) || o.vars["side"] != "LEFT" || o.vars["review"] != g.review {
		t.Fatalf("old side at %v", o.vars)
	}
}

func TestACommentGitHubPublishedOrRefusedIsAnError(t *testing.T) {
	c, g, _, ref := writer(t)
	g.publish = true
	if _, err := c.CreateDraft(context.Background(), ref, forge.NewDraft{Body: "Inline.", Anchor: line}); err == nil || !strings.Contains(err.Error(), "published") {
		t.Fatalf("published: %v", err)
	}
	g.publish, g.refuse = false, true
	if _, err := c.CreateDraft(context.Background(), ref, forge.NewDraft{Body: "Reply.", ThreadID: "PRRT_x"}); err == nil || !strings.Contains(err.Error(), "refused") {
		t.Fatalf("refused: %v", err)
	}
}

func TestAnInlineCommentOnAnEarlierHeadIsRefusedBeforeAnyWrite(t *testing.T) {
	c, g, _, ref := writer(t)
	stale := *line
	stale.Diff.Head = "2222222222222222222222222222222222222222"
	if _, err := c.CreateDraft(context.Background(), ref, forge.NewDraft{Body: "Inline.", Anchor: &stale}); err == nil || !strings.Contains(err.Error(), "new commits") {
		t.Fatalf("stale: %v", err)
	}
	if len(g.names()) != 0 {
		t.Fatalf("wrote %v", g.names())
	}
}

// A pending review started before the latest push reads a new thread's line
// against its own commit, so vuoro adds no thread to it.
func TestAnInlineCommentInAPendingReviewOnAnEarlierCommitIsRefused(t *testing.T) {
	c, g, _, ref := writer(t)
	g.review, g.at = "PRR_web", "2222222222222222222222222222222222222222"
	if _, err := c.CreateDraft(context.Background(), ref, forge.NewDraft{Body: "Inline.", Anchor: line}); err == nil || !strings.Contains(err.Error(), "started before") {
		t.Fatalf("inline: %v", err)
	}
	if len(g.names()) != 0 {
		t.Fatalf("wrote %v", g.names())
	}
	// A reply names its thread, which carries its own place.
	if _, err := c.CreateDraft(context.Background(), ref, forge.NewDraft{Body: "Reply.", ThreadID: "PRRT_x"}); err != nil {
		t.Fatal(err)
	}
}

// GitHub has no conditional write: a review submitted on GitHub between
// vuoro's check and an edit or a delete takes the write, and vuoro says so
// rather than reporting the draft changed.
func TestAnEditOrDeleteThatRacedASubmissionOnGitHubSaysSo(t *testing.T) {
	c, g, _, ref := writer(t)
	g.review, g.pending, g.submitted = "PRR_web", []string{"PRRC_1", "PRRC_2"}, true
	if err := c.UpdateDraft(context.Background(), ref, "PRRC_1", "Edited."); err == nil || !strings.Contains(err.Error(), "published comment PRRC_1 was edited") {
		t.Fatalf("edit: %v", err)
	}
	if err := c.DeleteDraft(context.Background(), ref, "PRRC_2"); err == nil || !strings.Contains(err.Error(), "published comment PRRC_2 was deleted") {
		t.Fatalf("delete: %v", err)
	}
}

// A comment on the whole change, and a reply to one, stay in vuoro: GitHub
// cannot change a pending review's body.
func TestCommentsOnTheWholeChangeStayInVuoro(t *testing.T) {
	c, g, s, ref := writer(t)
	if _, err := c.CreateDraft(context.Background(), ref, forge.NewDraft{Body: "Summary."}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CreateDraft(context.Background(), ref, forge.NewDraft{Body: "Answer.", ThreadID: "IC_general"}); err != nil {
		t.Fatal(err)
	}
	ds, _ := s.Drafts(ref)
	if len(ds) != 2 || !strings.HasPrefix(ds[0].ID, localPrefix) || ds[1].ThreadID != "IC_general" || len(g.names()) != 0 {
		t.Fatalf("drafts %+v, mutations %v", ds, g.names())
	}
}

func TestOnlyAPendingCommentIsEditedOrDeleted(t *testing.T) {
	c, g, _, ref := writer(t)
	ctx := context.Background()
	d, err := c.CreateDraft(ctx, ref, forge.NewDraft{Body: "Inline.", Anchor: line})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.UpdateDraft(ctx, ref, d.ID, "Edited."); err != nil {
		t.Fatal(err)
	}
	before := len(g.names())
	// A published comment's id: the mutation would edit it, so it is never sent.
	if err := c.UpdateDraft(ctx, ref, "PRRC_published", "Edited."); !errors.Is(err, forge.ErrNotFound) {
		t.Fatalf("published: %v", err)
	}
	if err := c.DeleteDraft(ctx, ref, "PRRC_published"); !errors.Is(err, forge.ErrNotFound) {
		t.Fatalf("published: %v", err)
	}
	if len(g.names()) != before {
		t.Fatalf("wrote %v", g.names()[before:])
	}
	if err := c.DeleteDraft(ctx, ref, d.ID); err != nil || len(g.pending) != 0 {
		t.Fatalf("delete %v, pending %v", err, g.pending)
	}
	g.review, g.body = "PRR_web", "Started on GitHub."
	if err := c.UpdateDraft(ctx, ref, "review-PRR_web", "x"); !errors.Is(err, forge.ErrUnsupported) {
		t.Fatalf("a web review's body: %v", err)
	}
}

func TestSubmitPublishesThePendingReviewWithTheBodyAndThenTheReplies(t *testing.T) {
	c, g, s, ref := writer(t)
	ctx := context.Background()
	g.review, g.body, g.pending = "PRR_web", "Started on GitHub.", []string{"PRRC_1"}
	c.CreateDraft(ctx, ref, forge.NewDraft{Body: "Summary."})
	c.CreateDraft(ctx, ref, forge.NewDraft{Body: "Answer.", ThreadID: "IC_general"})
	if err := c.Submit(ctx, ref, forge.VerdictChangesRequested); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(g.names(), " "); got != "submitPullRequestReview addComment" {
		t.Fatalf("mutations %s", got)
	}
	sub := g.ops[0].vars
	if sub["review"] != "PRR_web" || sub["event"] != "REQUEST_CHANGES" || sub["body"] != "Started on GitHub.\n\nSummary." {
		t.Fatalf("submit %v", sub)
	}
	if g.ops[1].vars["body"] != "Answer." || g.ops[1].vars["pr"] != "PR_1" {
		t.Fatalf("reply %v", g.ops[1].vars)
	}
	if ds, _ := s.Drafts(ref); len(ds) != 0 {
		t.Fatalf("left %+v", ds)
	}
}

func TestSubmitWithoutAPendingReviewGivesTheVerdictAlone(t *testing.T) {
	c, g, _, ref := writer(t)
	ctx := context.Background()
	if err := c.Submit(ctx, ref, forge.VerdictCommented); err == nil || !strings.Contains(err.Error(), "no pending comments") {
		t.Fatalf("nothing: %v", err)
	}
	if err := c.Submit(ctx, ref, forge.VerdictApproved); err != nil {
		t.Fatal(err)
	}
	if o := g.ops[0]; o.name != "addPullRequestReview" || o.vars["event"] != "APPROVE" || o.vars["head"] != head {
		t.Fatalf("approve %+v", o)
	}
	// A verdict that already stands on the head is not given twice.
	g.latest = map[string]any{"state": "APPROVED", "commit": map[string]any{"oid": head}}
	if err := c.Submit(ctx, ref, forge.VerdictApproved); err == nil || len(g.names()) != 1 {
		t.Fatalf("again: %v %v", err, g.names())
	}
}

// Replies left after their review was published are a retry: they post, and
// the review is not published again.
func TestARetryPostsTheRepliesAndNoReview(t *testing.T) {
	c, g, s, ref := writer(t)
	s.SaveDrafts(ref, []forge.Draft{{ID: "local-1", Body: "Answer.", ThreadID: "IC_general", AfterReview: true}})
	if err := c.Submit(context.Background(), ref, forge.VerdictApproved); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(g.names(), " "); got != "addComment" {
		t.Fatalf("mutations %s", got)
	}
}

func TestApproveRefusesWhilePendingCommentsWouldPublish(t *testing.T) {
	c, g, _, ref := writer(t)
	ctx := context.Background()
	g.review = "PRR_mine"
	if err := c.Approve(ctx, ref, true); err == nil || !strings.Contains(err.Error(), "Submit and approve") || len(g.names()) != 0 {
		t.Fatalf("with a pending review: %v %v", err, g.names())
	}
	g.review = ""
	if err := c.Approve(ctx, ref, true); err != nil || g.names()[0] != "addPullRequestReview" {
		t.Fatalf("approve: %v %v", err, g.names())
	}
	if err := c.Approve(ctx, ref, false); !errors.Is(err, forge.ErrUnsupported) {
		t.Fatalf("withdraw: %v", err)
	}
}

func TestResolveOnlyAReviewThread(t *testing.T) {
	c, g, _, ref := writer(t)
	ctx := context.Background()
	if err := c.Resolve(ctx, ref, "PRRT_x", true); err != nil {
		t.Fatal(err)
	}
	if err := c.Resolve(ctx, ref, "PRRT_x", false); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(g.names(), " "); got != "resolveReviewThread unresolveReviewThread" {
		t.Fatalf("mutations %s", got)
	}
	if err := c.Resolve(ctx, ref, "IC_general", true); !errors.Is(err, forge.ErrUnsupported) {
		t.Fatalf("a general comment: %v", err)
	}
}

func TestWritesNeedAStoreAndAToken(t *testing.T) {
	c, err := New("https://github.com", config.SecretForTest("sentinel-token"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Capabilities().Drafts {
		t.Fatal("drafts offered with nowhere to keep them")
	}
	if _, err := c.CreateDraft(context.Background(), forge.Ref{}, forge.NewDraft{Body: "x"}); !errors.Is(err, forge.ErrUnsupported) {
		t.Fatalf("no store: %v", err)
	}
	c.KeepDraftsIn(&memStore{})
	if caps := c.Capabilities(); caps != (forge.Capabilities{Drafts: true, Reply: true, Resolve: true, Approve: true, RequestChanges: true, Suggestions: true, SpanSuggestions: true,
		Reviewers: true, Assignees: true, Labels: true, SetDraft: true}) {
		t.Fatalf("caps %+v", caps)
	}
}

// Open shows the person's pending review as drafts: a new thread, a reply in
// a published one, and a body started on GitHub, then the comments in vuoro.
func TestOpenShowsThePendingReviewAsDrafts(t *testing.T) {
	c, s, ref := serve(t, "14578")
	store := &memStore{}
	c.KeepDraftsIn(store)
	store.SaveDrafts(ref, []forge.Draft{{ID: "local-1", Body: "Summary."}})
	var published string
	s.edit(func(pr map[string]any) {
		reviews := pr["reviews"].(map[string]any)
		reviews["nodes"] = append(nodesOf(pr, "reviews"), map[string]any{"id": "PRR_p", "author": map[string]any{"login": "me", "__typename": "User"}, "state": "PENDING", "body": "Started on GitHub."})
		threads := pr["reviewThreads"].(map[string]any)
		first := nodesOf(pr, "reviewThreads")[0].(map[string]any)
		published = first["id"].(string)
		cs := first["comments"].(map[string]any)
		cs["nodes"] = append(cs["nodes"].([]any), map[string]any{"id": "PRRC_reply", "author": map[string]any{"login": "me"}, "body": "My reply.", "createdAt": "2026-10-08T00:00:00Z", "pullRequestReview": map[string]any{"id": "PRR_p", "state": "PENDING"}})
		threads["nodes"] = append(nodesOf(pr, "reviewThreads"), map[string]any{"id": "PRRT_new", "path": "a.go", "line": 2, "diffSide": "RIGHT", "comments": map[string]any{"pageInfo": map[string]any{}, "nodes": []any{
			map[string]any{"id": "PRRC_new", "author": map[string]any{"login": "me"}, "body": "My inline.", "createdAt": "2026-10-08T00:00:00Z", "commit": map[string]any{"oid": head}, "pullRequestReview": map[string]any{"id": "PRR_p", "state": "PENDING"}}}}})
	})
	ch, err := c.Open(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, d := range ch.Drafts {
		where := d.ThreadID
		if d.Anchor != nil {
			where = d.Anchor.Path
		}
		got = append(got, d.ID+"@"+where)
	}
	want := []string{"review-PRR_p@", "PRRC_reply@" + published, "PRRC_new@a.go", "local-1@"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("drafts %v, want %v", got, want)
	}
	if !ch.Capabilities.Drafts || !ch.Capabilities.Resolve {
		t.Fatalf("caps %+v", ch.Capabilities)
	}
	for _, th := range ch.Threads {
		if th.ID == "PRRT_new" {
			t.Fatal("a draft thread is in the published threads")
		}
	}
}

// A reply GitHub does not confirm stays pending, marked to follow the review,
// so a retry posts it and nothing else.
func TestAnUnconfirmedReplyIsKept(t *testing.T) {
	c, g, s, ref := writer(t)
	ctx := context.Background()
	c.CreateDraft(ctx, ref, forge.NewDraft{Body: "Summary."})
	c.CreateDraft(ctx, ref, forge.NewDraft{Body: "Answer.", ThreadID: "IC_general"})
	g.refuse = true
	if err := c.Submit(ctx, ref, forge.VerdictCommented); err == nil || !strings.Contains(err.Error(), "did not confirm") {
		t.Fatalf("unconfirmed: %v", err)
	}
	ds, _ := s.Drafts(ref)
	if len(ds) != 1 || ds[0].Body != "Answer." || !ds[0].AfterReview {
		t.Fatalf("kept %+v", ds)
	}
	g.refuse = false
	if err := c.Submit(ctx, ref, forge.VerdictCommented); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(g.names(), " "); got != "addPullRequestReview addComment addComment" {
		t.Fatalf("mutations %s", got)
	}
}

// A suggestion over several lines replaces the lines its comment spans, so
// such a comment names its first line on the same side, and a comment on
// one line names none.
func TestACommentSpanningLinesNamesItsStart(t *testing.T) {
	c, g, _, ref := writer(t)
	ctx := context.Background()
	span := *line
	span.StartLine, span.NewLine = 2, 4
	if _, err := c.CreateDraft(ctx, ref, forge.NewDraft{Body: "```suggestion\nx\n```", Anchor: &span}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CreateDraft(ctx, ref, forge.NewDraft{Body: "One line.", Anchor: line}); err != nil {
		t.Fatal(err)
	}
	spanning, one := g.ops[1], g.ops[2]
	if spanning.vars["start"] != float64(2) || spanning.vars["startSide"] != "RIGHT" || spanning.vars["line"] != float64(4) {
		t.Fatalf("spanning comment %v", spanning.vars)
	}
	if _, ok := one.vars["start"]; ok {
		t.Fatalf("a one-line comment named a start: %v", one.vars)
	}
	if !c.Capabilities().Suggestions || !c.Capabilities().SpanSuggestions {
		t.Fatal("GitHub does not offer suggestions")
	}
}
