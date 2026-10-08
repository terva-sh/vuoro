package github

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/terva-sh/vuoro/forge"
)

// The person's inline comments and their replies in review threads are kept
// in a GitHub pending review, which only they can see until they submit
// (D54). GitHub cannot change a pending review's body once the review
// exists, so a comment on the whole change, and a reply to one, stays in
// vuoro until Submit, which sends the body with the verdict. docs/plan.md
// section 5.4 has what the live spike showed.
//
// A comment or reply sent without the pending review's id is published at
// once, so every write here names the review, and a write that comes back
// as anything but pending is reported as published.

var (
	_ forge.Writer       = (*Client)(nil)
	_ forge.LocalDrafter = (*Client)(nil)
)

// errNoStore is a comment with nowhere private to keep it.
var errNoStore = fmt.Errorf("github: pending comments need vuoro's state directory: %w", forge.ErrUnsupported)

// localPrefix marks a pending comment kept in vuoro, and reviewPrefix the
// body of a pending review the person started in GitHub's window. A comment
// of the pending review is its GitHub id.
const (
	localPrefix  = "local-"
	reviewPrefix = "review-"
	threadPrefix = "PRRT_"
)

// KeepDraftsIn gives the client somewhere private for pending comments.
func (c *Client) KeepDraftsIn(s forge.DraftStore) {
	c.draftMu.Lock()
	c.drafts = s
	c.draftMu.Unlock()
}

func (c *Client) store() (forge.DraftStore, error) {
	if c.token.Empty() {
		return nil, ErrNoToken
	}
	if c.drafts == nil {
		return nil, errNoStore
	}
	return c.drafts, nil
}

func localID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return localPrefix + hex.EncodeToString(b)
}

// pending is the pull request as a write needs it: its id and head, the
// person's pending review, if any, and their latest published review.
type pending struct {
	pr, head string
	review   *pendingReview
	latest   *struct {
		State  string  `json:"state"`
		Commit *gqlOID `json:"commit"`
	}
}

type pendingReview struct {
	ID              string  `json:"id"`
	Body            string  `json:"body"`
	ViewerDidAuthor bool    `json:"viewerDidAuthor"`
	Commit          *gqlOID `json:"commit"`
	Comments        struct {
		PageInfo pageInfo `json:"pageInfo"`
		Nodes    []struct {
			ID string `json:"id"`
		} `json:"nodes"`
	} `json:"comments"`
}

const pendingQuery = `query($owner: String!, $name: String!, $number: Int!) { repository(owner: $owner, name: $name) { pullRequest(number: $number) {
	id headRefOid
	viewerLatestReview { state commit { oid } }
	reviews(states: [PENDING], first: 10) { nodes { id body viewerDidAuthor commit { oid } comments(first: 100) { pageInfo { hasNextPage } nodes { id } } } } } } }`

func (c *Client) pending(ctx context.Context, ref forge.Ref) (*pending, error) {
	owner, name := ownerName(ref)
	var out struct {
		Repository *struct {
			PullRequest *struct {
				ID                 string `json:"id"`
				HeadRefOID         string `json:"headRefOid"`
				ViewerLatestReview *struct {
					State  string  `json:"state"`
					Commit *gqlOID `json:"commit"`
				} `json:"viewerLatestReview"`
				Reviews struct {
					Nodes []pendingReview `json:"nodes"`
				} `json:"reviews"`
			} `json:"pullRequest"`
		} `json:"repository"`
	}
	if err := c.query(ctx, pendingQuery, map[string]any{"owner": owner, "name": name, "number": ref.Number}, &out); err != nil {
		return nil, err
	}
	if out.Repository == nil || out.Repository.PullRequest == nil {
		return nil, forge.ErrNotFound
	}
	p := out.Repository.PullRequest
	info := &pending{pr: p.ID, head: p.HeadRefOID, latest: p.ViewerLatestReview}
	for i := range p.Reviews.Nodes {
		if r := &p.Reviews.Nodes[i]; r.ViewerDidAuthor {
			if r.Comments.PageInfo.HasNextPage {
				return nil, errors.New("github: your pending review has more comments than vuoro reads")
			}
			info.review = r
		}
	}
	return info, nil
}

// holds says whether the pending review holds comment id.
func (p *pending) holds(id string) bool {
	return p.review != nil && slices.ContainsFunc(p.review.Comments.Nodes, func(n struct {
		ID string `json:"id"`
	}) bool {
		return n.ID == id
	})
}

// stands says whether the person's latest published review is event on
// head: a verdict already given, which a retry must not give again.
func (p *pending) stands(event string) bool {
	state := map[string]string{"APPROVE": "APPROVED", "REQUEST_CHANGES": "CHANGES_REQUESTED", "COMMENT": "COMMENTED"}[event]
	return p.latest != nil && p.latest.State == state && p.latest.Commit != nil && p.latest.Commit.OID == p.head
}

// review is the person's pending review, which it starts at the head when
// there is none.
func (c *Client) review(ctx context.Context, p *pending) (string, error) {
	if p.review != nil {
		return p.review.ID, nil
	}
	var out struct {
		Add struct {
			Review *struct {
				ID    string `json:"id"`
				State string `json:"state"`
			} `json:"pullRequestReview"`
		} `json:"addPullRequestReview"`
	}
	q := `mutation($pr: ID!, $head: GitObjectID!) { addPullRequestReview(input: {pullRequestId: $pr, commitOID: $head}) { pullRequestReview { id state } } }`
	if err := c.query(ctx, q, map[string]any{"pr": p.pr, "head": p.head}, &out); err != nil {
		return "", err
	}
	if out.Add.Review == nil || out.Add.Review.State != "PENDING" {
		return "", errors.New("github: GitHub did not start a pending review")
	}
	p.review = &pendingReview{ID: out.Add.Review.ID, ViewerDidAuthor: true, Commit: &gqlOID{OID: p.head}}
	return p.review.ID, nil
}

type gqlWritten struct {
	ID     string `json:"id"`
	State  string `json:"state"`
	Body   string `json:"body"`
	Review *struct {
		ID string `json:"id"`
	} `json:"pullRequestReview"`
}

// kept checks that a write landed in the person's pending review. GitHub
// answers a write it refuses with null, and publishes one that names no
// pending review; vuoro names one on every write, so either is an error.
func kept(w *gqlWritten, review string) (forge.Draft, error) {
	switch {
	case w == nil:
		return forge.Draft{}, errors.New("github: GitHub refused the comment; the line may not be in the diff, or the review was submitted elsewhere")
	case w.State != "PENDING" || w.Review == nil || w.Review.ID != review:
		return forge.Draft{}, fmt.Errorf("github: the comment %s was published rather than kept pending", w.ID)
	}
	return forge.Draft{ID: w.ID, Body: w.Body}, nil
}

// CreateDraft keeps a pending comment. An inline comment and a reply in a
// review thread go into the person's pending review on GitHub; a comment on
// the whole change, or a reply to one, stays in vuoro until Submit.
func (c *Client) CreateDraft(ctx context.Context, ref forge.Ref, d forge.NewDraft) (forge.Draft, error) {
	c.draftMu.Lock()
	defer c.draftMu.Unlock()
	s, err := c.store()
	if err != nil {
		return forge.Draft{}, err
	}
	if strings.TrimSpace(d.Body) == "" {
		return forge.Draft{}, errors.New("github: the comment is empty")
	}
	if d.ThreadID == "" && d.Anchor == nil || d.ThreadID != "" && !strings.HasPrefix(d.ThreadID, threadPrefix) {
		all, err := s.Drafts(ref)
		if err != nil {
			return forge.Draft{}, err
		}
		nd := forge.Draft{ID: localID(), Body: d.Body, ThreadID: d.ThreadID}
		if err := s.SaveDrafts(ref, append(all, nd)); err != nil {
			return forge.Draft{}, err
		}
		return nd, nil
	}
	p, err := c.pending(ctx, ref)
	if err != nil {
		return forge.Draft{}, err
	}
	if d.ThreadID != "" {
		review, err := c.review(ctx, p)
		if err != nil {
			return forge.Draft{}, err
		}
		var out struct {
			Reply struct {
				Comment *gqlWritten `json:"comment"`
			} `json:"addPullRequestReviewThreadReply"`
		}
		q := `mutation($thread: ID!, $review: ID!, $body: String!) { addPullRequestReviewThreadReply(input: {pullRequestReviewThreadId: $thread, pullRequestReviewId: $review, body: $body}) { comment { id state body pullRequestReview { id } } } }`
		if err := c.query(ctx, q, map[string]any{"thread": d.ThreadID, "review": review, "body": d.Body}, &out); err != nil {
			return forge.Draft{}, err
		}
		nd, err := kept(out.Reply.Comment, review)
		nd.ThreadID = d.ThreadID
		return nd, err
	}
	a := d.Anchor
	if a.NewLine <= 0 && a.OldLine <= 0 {
		return forge.Draft{}, errors.New("github: an inline comment needs a line")
	}
	if a.Diff.Head != "" && a.Diff.Head != p.head {
		return forge.Draft{}, errors.New("github: the pull request has new commits since this diff was read; refresh it and comment again")
	}
	// A new thread's line is read against its review's commit, so a pending
	// review started before the latest push would misplace it.
	if p.review != nil && (p.review.Commit == nil || p.review.Commit.OID != p.head) {
		return forge.Draft{}, errors.New("github: your pending review on GitHub was started before the pull request's latest commits; submit it, or discard it on GitHub, and comment again")
	}
	review, err := c.review(ctx, p)
	if err != nil {
		return forge.Draft{}, err
	}
	line, side := a.NewLine, "RIGHT"
	if line <= 0 {
		line, side = a.OldLine, "LEFT"
	}
	var out struct {
		Add struct {
			Thread *struct {
				Comments struct {
					Nodes []gqlWritten `json:"nodes"`
				} `json:"comments"`
			} `json:"thread"`
		} `json:"addPullRequestReviewThread"`
	}
	vars := map[string]any{"review": review, "path": a.Path, "line": line, "side": side, "body": d.Body}
	q := `mutation($review: ID!, $path: String!, $line: Int!, $side: DiffSide!, $body: String!) { addPullRequestReviewThread(input: {pullRequestReviewId: $review, path: $path, line: $line, side: $side, body: $body}) { thread { comments(first: 1) { nodes { id state body pullRequestReview { id } } } } } }`
	// A comment spanning several lines, which a suggestion over them needs,
	// starts on the same side.
	if side == "RIGHT" && a.StartLine > 0 && a.StartLine < line {
		vars["start"], vars["startSide"] = a.StartLine, side
		q = `mutation($review: ID!, $path: String!, $line: Int!, $side: DiffSide!, $start: Int!, $startSide: DiffSide!, $body: String!) { addPullRequestReviewThread(input: {pullRequestReviewId: $review, path: $path, line: $line, side: $side, startLine: $start, startSide: $startSide, body: $body}) { thread { comments(first: 1) { nodes { id state body pullRequestReview { id } } } } } }`
	}
	if err := c.query(ctx, q, vars, &out); err != nil {
		return forge.Draft{}, err
	}
	var first *gqlWritten
	if out.Add.Thread != nil && len(out.Add.Thread.Comments.Nodes) > 0 {
		first = &out.Add.Thread.Comments.Nodes[0]
	}
	nd, err := kept(first, review)
	nd.Anchor = a
	return nd, err
}

// errWebBody is a change to the body of a review the person started in
// GitHub's window, which GitHub does not allow before the review is
// submitted. Submit keeps it.
var errWebBody = fmt.Errorf("github: this summary belongs to a review you started on GitHub, which GitHub lets you change only as you submit it; Submit keeps it: %w", forge.ErrUnsupported)

// UpdateDraft edits a pending comment, in vuoro or in the pending review.
func (c *Client) UpdateDraft(ctx context.Context, ref forge.Ref, id, body string) error {
	c.draftMu.Lock()
	defer c.draftMu.Unlock()
	if strings.TrimSpace(body) == "" {
		return errors.New("github: the comment is empty")
	}
	switch {
	case strings.HasPrefix(id, localPrefix):
		return c.changeLocal(ref, id, func(d *forge.Draft) { d.Body = body })
	case strings.HasPrefix(id, reviewPrefix):
		return errWebBody
	}
	// The mutation edits published comments too, so the comment must be in
	// the person's pending review. GitHub has no conditional write, so a
	// review submitted on GitHub between the check and the write takes the
	// edit, and the answer says so.
	p, err := c.pendingComment(ctx, ref, id)
	if err != nil {
		return err
	}
	var out struct {
		Update struct {
			Comment *gqlWritten `json:"pullRequestReviewComment"`
		} `json:"updatePullRequestReviewComment"`
	}
	q := `mutation($id: ID!, $body: String!) { updatePullRequestReviewComment(input: {pullRequestReviewCommentId: $id, body: $body}) { pullRequestReviewComment { id state body pullRequestReview { id } } } }`
	if err := c.query(ctx, q, map[string]any{"id": id, "body": body}, &out); err != nil {
		return err
	}
	if w := out.Update.Comment; w != nil && w.State != "PENDING" {
		return fmt.Errorf("github: your review was submitted on GitHub as this edit was sent, so the published comment %s was edited", id)
	}
	_, err = kept(out.Update.Comment, p.review.ID)
	return err
}

// DeleteDraft removes a pending comment, wherever it is kept.
func (c *Client) DeleteDraft(ctx context.Context, ref forge.Ref, id string) error {
	c.draftMu.Lock()
	defer c.draftMu.Unlock()
	switch {
	case strings.HasPrefix(id, localPrefix):
		return c.changeLocal(ref, id, nil)
	case strings.HasPrefix(id, reviewPrefix):
		return errWebBody
	}
	if _, err := c.store(); err != nil {
		return err
	}
	if _, err := c.pendingComment(ctx, ref, id); err != nil {
		return err
	}
	// As with an edit, a review submitted between the check and the write
	// loses the comment; GitHub answers with its state, which says so.
	q := `mutation($id: ID!) { deletePullRequestReviewComment(input: {id: $id}) { pullRequestReviewComment { id state } } }`
	var out struct {
		Delete *struct {
			Comment *gqlWritten `json:"pullRequestReviewComment"`
		} `json:"deletePullRequestReviewComment"`
	}
	if err := c.query(ctx, q, map[string]any{"id": id}, &out); err != nil {
		return err
	}
	switch {
	case out.Delete == nil || out.Delete.Comment == nil:
		return fmt.Errorf("github: GitHub did not confirm deleting comment %s", id)
	case out.Delete.Comment.State != "PENDING":
		return fmt.Errorf("github: your review was submitted on GitHub as this delete was sent, so the published comment %s was deleted", id)
	}
	return nil
}

// pendingComment reads the pull request for a write to comment id, which
// must be in the person's pending review.
func (c *Client) pendingComment(ctx context.Context, ref forge.Ref, id string) (*pending, error) {
	if _, err := c.store(); err != nil {
		return nil, err
	}
	p, err := c.pending(ctx, ref)
	if err != nil {
		return nil, err
	}
	if !p.holds(id) {
		return nil, fmt.Errorf("github: comment %s is no longer pending; it may have been published: %w", id, forge.ErrNotFound)
	}
	return p, nil
}

// changeLocal edits the stored draft id with edit, or removes it when edit is
// nil. The caller holds draftMu.
func (c *Client) changeLocal(ref forge.Ref, id string, edit func(*forge.Draft)) error {
	s, err := c.store()
	if err != nil {
		return err
	}
	all, err := s.Drafts(ref)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(all, func(d forge.Draft) bool { return d.ID == id })
	if i < 0 {
		return fmt.Errorf("github: no pending comment %s: %w", id, forge.ErrNotFound)
	}
	if edit == nil {
		all = slices.Delete(all, i, i+1)
	} else {
		edit(&all[i])
	}
	return s.SaveDrafts(ref, all)
}

var events = map[forge.Verdict]string{
	forge.VerdictCommented:        "COMMENT",
	forge.VerdictApproved:         "APPROVE",
	forge.VerdictChangesRequested: "REQUEST_CHANGES",
}

// Submit publishes the person's pending review with their verdict, and the
// comments kept in vuoro as its body, then posts their replies to comments
// on the whole change, which GitHub takes as new comments, one by one.
func (c *Client) Submit(ctx context.Context, ref forge.Ref, verdict forge.Verdict) error {
	event, ok := events[verdict]
	if !ok {
		return fmt.Errorf("github: unknown verdict %q", verdict)
	}
	c.draftMu.Lock()
	defer c.draftMu.Unlock()
	s, err := c.store()
	if err != nil {
		return err
	}
	local, err := s.Drafts(ref)
	if err != nil {
		return err
	}
	p, err := c.pending(ctx, ref)
	if err != nil {
		return err
	}
	var bodies []string
	if p.review != nil && strings.TrimSpace(p.review.Body) != "" {
		bodies = append(bodies, p.review.Body)
	}
	var replies []forge.Draft
	for _, d := range local {
		if d.ThreadID != "" {
			replies = append(replies, d)
		} else {
			bodies = append(bodies, d.Body)
		}
	}
	body := strings.Join(bodies, "\n\n")
	content := body != "" || (p.review != nil && len(p.review.Comments.Nodes) > 0)
	// Replies left after their review was published are a retry: post them
	// and publish no review. With fresh replies only, a verdict publishes
	// unless it already stands.
	retry := len(replies) > 0 && !slices.ContainsFunc(replies, func(d forge.Draft) bool { return !d.AfterReview })
	review := content || (verdict != forge.VerdictCommented && !retry && !p.stands(event))
	if !review && len(replies) == 0 {
		return errors.New("github: you have no pending comments to submit")
	}
	if review {
		var out struct {
			Submit *struct {
				Review *struct {
					State string `json:"state"`
				} `json:"pullRequestReview"`
			} `json:"submitPullRequestReview"`
			Add *struct {
				Review *struct {
					State string `json:"state"`
				} `json:"pullRequestReview"`
			} `json:"addPullRequestReview"`
		}
		vars := map[string]any{"event": event, "body": body}
		var q string
		if p.review != nil {
			q = `mutation($review: ID!, $event: PullRequestReviewEvent!, $body: String) { submitPullRequestReview(input: {pullRequestReviewId: $review, event: $event, body: $body}) { pullRequestReview { state } } }`
			vars["review"] = p.review.ID
		} else {
			q = `mutation($pr: ID!, $head: GitObjectID!, $event: PullRequestReviewEvent!, $body: String) { addPullRequestReview(input: {pullRequestId: $pr, commitOID: $head, event: $event, body: $body}) { pullRequestReview { state } } }`
			vars["pr"], vars["head"] = p.pr, p.head
		}
		if err := c.query(ctx, q, vars, &out); err != nil {
			return err
		}
		submitted := out.Submit
		if submitted == nil {
			submitted = out.Add
		}
		if submitted == nil || submitted.Review == nil || submitted.Review.State == "PENDING" {
			return errors.New("github: GitHub did not publish the review")
		}
		// The review is public now: drop what it published, and mark the
		// replies as following it, so a retry never publishes it twice.
		for i := range replies {
			replies[i].AfterReview = true
		}
		if err := s.SaveDrafts(ref, replies); err != nil {
			return fmt.Errorf("github: your review was published, but vuoro could not clear its pending comments; delete them before submitting again: %w", err)
		}
	}
	remaining := slices.Clone(replies)
	var failed []string
	for _, d := range replies {
		var out struct {
			Add *struct {
				Edge *struct {
					Node *struct {
						ID string `json:"id"`
					} `json:"node"`
				} `json:"commentEdge"`
			} `json:"addComment"`
		}
		q := `mutation($pr: ID!, $body: String!) { addComment(input: {subjectId: $pr, body: $body}) { commentEdge { node { id } } } }`
		err := c.query(ctx, q, map[string]any{"pr": p.pr, "body": d.Body}, &out)
		if err == nil && (out.Add == nil || out.Add.Edge == nil || out.Add.Edge.Node == nil || out.Add.Edge.Node.ID == "") {
			// Kept until GitHub names the comment it posted.
			err = errors.New("GitHub did not confirm the comment")
		}
		if err != nil {
			failed = append(failed, fmt.Sprintf("your reply %s to thread %s: %v", d.ID, d.ThreadID, err))
			continue
		}
		remaining = slices.DeleteFunc(remaining, func(x forge.Draft) bool { return x.ID == d.ID })
		if err := s.SaveDrafts(ref, remaining); err != nil {
			return fmt.Errorf("github: a reply was published, but vuoro could not clear it; delete it before submitting again: %w", err)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("github: %d of %d replies were not posted and are still pending: %s", len(failed), len(replies), strings.Join(failed, "; "))
	}
	return nil
}

// Resolve resolves a review thread, or reopens it. A comment on the whole
// change is not a thread GitHub can resolve.
func (c *Client) Resolve(ctx context.Context, ref forge.Ref, threadID string, resolved bool) error {
	if c.token.Empty() {
		return ErrNoToken
	}
	if !strings.HasPrefix(threadID, threadPrefix) {
		return fmt.Errorf("github: only a thread on a line can be resolved: %w", forge.ErrUnsupported)
	}
	name := "resolveReviewThread"
	if !resolved {
		name = "unresolveReviewThread"
	}
	var out map[string]*struct {
		Thread *struct {
			IsResolved bool `json:"isResolved"`
		} `json:"thread"`
	}
	q := "mutation($id: ID!) { " + name + "(input: {threadId: $id}) { thread { isResolved } } }"
	if err := c.query(ctx, q, map[string]any{"id": threadID}, &out); err != nil {
		return err
	}
	if r := out[name]; r == nil || r.Thread == nil || r.Thread.IsResolved != resolved {
		return errors.New("github: GitHub did not change the thread")
	}
	return nil
}

// Approve approves with a review of its own. Withdrawing an approval is not
// something a reviewer can do on GitHub: only a dismissal, by someone with
// write access, takes one back.
func (c *Client) Approve(ctx context.Context, ref forge.Ref, approve bool) error {
	if !approve {
		return fmt.Errorf("github: a reviewer cannot withdraw their own approval: %w", forge.ErrUnsupported)
	}
	if c.token.Empty() {
		return ErrNoToken
	}
	p, err := c.pending(ctx, ref)
	if err != nil {
		return err
	}
	// GitHub allows one pending review, and approving would have to submit
	// it, publishing the person's comments with the approval.
	if p.review != nil {
		return errors.New("github: you have pending comments on this pull request; approving would publish them, so use Submit and approve")
	}
	if p.stands("APPROVE") {
		return nil
	}
	var out struct {
		Add *struct {
			Review *struct {
				State string `json:"state"`
			} `json:"pullRequestReview"`
		} `json:"addPullRequestReview"`
	}
	q := `mutation($pr: ID!, $head: GitObjectID!) { addPullRequestReview(input: {pullRequestId: $pr, commitOID: $head, event: APPROVE}) { pullRequestReview { state } } }`
	if err := c.query(ctx, q, map[string]any{"pr": p.pr, "head": p.head}, &out); err != nil {
		return err
	}
	if out.Add == nil || out.Add.Review == nil || out.Add.Review.State != "APPROVED" {
		return errors.New("github: GitHub did not record the approval")
	}
	return nil
}
