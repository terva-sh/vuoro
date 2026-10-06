package forgejo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/terva-sh/vuoro/forge"
)

// Forgejo's issue timeline shows a pending review's body to every reader of
// the repository, and keeps each body it was ever given. So the person's
// pending comments stay in vuoro until they submit, and Submit publishes the
// review in one call (D43, docs/plan.md section 5.3).

var (
	_ forge.Writer       = (*Client)(nil)
	_ forge.LocalDrafter = (*Client)(nil)
)

// errNoToken is a write attempted without the person's token.
var errNoToken = errors.New("forgejo: writing needs your token; name a token_command for this forge in the config file")

// errNoStore is a pending comment with nowhere private to keep it.
var errNoStore = fmt.Errorf("forgejo: pending comments need vuoro's state directory: %w", forge.ErrUnsupported)

// localPrefix marks a pending comment kept in vuoro. A pending review body
// started in Forgejo's web UI is reviewPrefix and its id; a comment of that
// review is its bare comment id.
const (
	localPrefix  = "local-"
	reviewPrefix = "review-"
)

// KeepDraftsIn gives the client somewhere private for pending comments.
func (c *Client) KeepDraftsIn(s forge.DraftStore) {
	c.draftMu.Lock()
	c.drafts = s
	c.draftMu.Unlock()
}

func (c *Client) Capabilities() forge.Capabilities {
	c.draftMu.Lock()
	write := !c.token.Empty() && c.drafts != nil
	c.draftMu.Unlock()
	// Forgejo's API has no route to resolve a thread, and a reviewer cannot
	// withdraw their own approval; it renders a suggestion block as code.
	return forge.Capabilities{Drafts: write, Reply: write, Approve: write, RequestChanges: write}
}

// store is the draft store, or an error saying why there is none.
func (c *Client) store() (forge.DraftStore, error) {
	if c.token.Empty() {
		return nil, errNoToken
	}
	if c.drafts == nil {
		return nil, errNoStore
	}
	return c.drafts, nil
}

func (c *Client) write(ctx context.Context, method, path string, body any) ([]byte, error) {
	if c.token.Empty() {
		return nil, errNoToken
	}
	return c.do(ctx, method, path, nil, body)
}

func prPath(ref forge.Ref) string { return fmt.Sprintf("%s/pulls/%d", repoPath(ref), ref.Number) }

func localID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return localPrefix + hex.EncodeToString(b)
}

// CreateDraft keeps a pending comment in vuoro, where only the person sees
// it until they submit.
func (c *Client) CreateDraft(_ context.Context, ref forge.Ref, d forge.NewDraft) (forge.Draft, error) {
	c.draftMu.Lock()
	defer c.draftMu.Unlock()
	s, err := c.store()
	if err != nil {
		return forge.Draft{}, err
	}
	if strings.TrimSpace(d.Body) == "" {
		return forge.Draft{}, errors.New("forgejo: the comment is empty")
	}
	if d.Anchor != nil && d.Anchor.NewLine <= 0 && d.Anchor.OldLine <= 0 {
		return forge.Draft{}, errors.New("forgejo: an inline comment needs a line")
	}
	all, err := s.Drafts(ref)
	if err != nil {
		return forge.Draft{}, err
	}
	nd := forge.Draft{ID: localID(), Body: d.Body, ThreadID: d.ThreadID}
	if d.ThreadID == "" {
		nd.Anchor = d.Anchor
	}
	if err := s.SaveDrafts(ref, append(all, nd)); err != nil {
		return forge.Draft{}, err
	}
	return nd, nil
}

// UpdateDraft edits a pending comment: in vuoro's store, or on Forgejo for a
// comment of a review the person started in its web UI.
func (c *Client) UpdateDraft(ctx context.Context, ref forge.Ref, id, body string) error {
	c.draftMu.Lock()
	defer c.draftMu.Unlock()
	if strings.TrimSpace(body) == "" {
		return errors.New("forgejo: the comment is empty")
	}
	switch {
	case strings.HasPrefix(id, localPrefix):
		return c.changeLocal(ref, id, func(d *forge.Draft) { d.Body = body })
	case strings.HasPrefix(id, reviewPrefix):
		return errWebBody
	}
	n, err := commentID(id)
	if err != nil {
		return err
	}
	// The issue-comment route edits any comment, published ones too, so
	// the id must be a comment of the person's pending review.
	if _, err := c.pendingComment(ctx, ref, n); err != nil {
		return err
	}
	_, err = c.write(ctx, http.MethodPatch, fmt.Sprintf("%s/issues/comments/%d", repoPath(ref), n), map[string]any{"body": body})
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
	n, err := commentID(id)
	if err != nil {
		return err
	}
	pending, err := c.pendingComment(ctx, ref, n)
	if err != nil {
		return err
	}
	_, err = c.write(ctx, http.MethodDelete, fmt.Sprintf("%s/reviews/%d/comments/%d", prPath(ref), pending.ID, n), nil)
	return err
}

// pendingComment is the person's pending review on ref when comment n is one
// of its comments, and an error saying it is no longer pending otherwise.
func (c *Client) pendingComment(ctx context.Context, ref forge.Ref, n int64) (*apiReview, error) {
	mine, err := c.myReviews(ctx, ref)
	if err != nil {
		return nil, err
	}
	if mine.pending != nil {
		cs, err := read[[]apiComment](ctx, c, fmt.Sprintf("%s/reviews/%d/comments", prPath(ref), mine.pending.ID))
		if err != nil {
			return nil, err
		}
		if slices.ContainsFunc(cs, func(cm apiComment) bool { return cm.ID == n }) {
			return mine.pending, nil
		}
	}
	return nil, fmt.Errorf("forgejo: comment %d is no longer pending; it may have been published: %w", n, forge.ErrNotFound)
}

// errWebBody is a change to the summary of a review the person started in
// Forgejo's web UI. Setting a pending review's body puts it in the timeline,
// where others can read it, so vuoro leaves it as it is; Submit keeps it.
var errWebBody = fmt.Errorf("forgejo: this summary belongs to a review you started in Forgejo, and changing it there would show it to others before you submit; Submit keeps it: %w", forge.ErrUnsupported)

func commentID(id string) (int64, error) {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("forgejo: bad pending comment id %q", id)
	}
	return n, nil
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
		return fmt.Errorf("forgejo: no pending comment %s: %w", id, forge.ErrNotFound)
	}
	if edit == nil {
		all = slices.Delete(all, i, i+1)
	} else {
		edit(&all[i])
	}
	return s.SaveDrafts(ref, all)
}

// me is the signed-in account, read once.
func (c *Client) me(ctx context.Context) (string, error) {
	c.meMu.Lock()
	defer c.meMu.Unlock()
	if c.login != "" {
		return c.login, nil
	}
	u, err := read[apiUser](ctx, c, "/user")
	if err != nil {
		return "", err
	}
	if u.Login == "" {
		return "", errors.New("forgejo: the signed-in account has no login")
	}
	c.login = u.Login
	return c.login, nil
}

// mine is the person's own reviews on a pull request: the pending one they
// started in Forgejo's web UI, if any, and their newest published verdict.
type mine struct {
	pending, verdict *apiReview
}

func (c *Client) myReviews(ctx context.Context, ref forge.Ref) (mine, error) {
	me, err := c.me(ctx)
	if err != nil {
		return mine{}, err
	}
	reviews, err := pages[apiReview](ctx, c, prPath(ref)+"/reviews")
	if err != nil {
		return mine{}, err
	}
	var m mine
	for i := range reviews {
		r := &reviews[i]
		if !strings.EqualFold(r.User.Login, me) {
			continue
		}
		switch r.State {
		case "PENDING":
			m.pending = r
		case "APPROVED", "REQUEST_CHANGES":
			if m.verdict == nil || !r.Submitted.Before(m.verdict.Submitted) {
				m.verdict = r
			}
		}
	}
	return m, nil
}

// stands says whether the person's newest verdict is event, undismissed, on
// head: a verdict already published, which a retry must not publish again.
func (m mine) stands(event, head string) bool {
	return m.verdict != nil && m.verdict.State == event && !m.verdict.Dismissed && !m.verdict.Stale && m.verdict.Commit == head
}

var events = map[forge.Verdict]string{
	forge.VerdictCommented:        "COMMENT",
	forge.VerdictApproved:         "APPROVED",
	forge.VerdictChangesRequested: "REQUEST_CHANGES",
}

type reviewComment struct {
	Path        string `json:"path"`
	Body        string `json:"body"`
	NewPosition int    `json:"new_position,omitempty"`
	OldPosition int    `json:"old_position,omitempty"`
}

// position places an anchor the way Forgejo does: a line on the new side by
// its new line, and a removed one by its old line.
func position(path, body string, newLine, oldLine int) reviewComment {
	rc := reviewComment{Path: path, Body: body}
	if newLine > 0 {
		rc.NewPosition = newLine
	} else {
		rc.OldPosition = oldLine
	}
	return rc
}

// Submit publishes the person's pending comments as one review with their
// verdict, then posts their replies into the threads they answer. Forgejo
// takes a reply as a comment on the thread's own review, which publishes at
// once, so replies follow the review one by one.
func (c *Client) Submit(ctx context.Context, ref forge.Ref, verdict forge.Verdict) error {
	event, ok := events[verdict]
	if !ok {
		return fmt.Errorf("forgejo: unknown verdict %q", verdict)
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
	pr, err := read[apiPR](ctx, c, prPath(ref))
	if err != nil {
		return err
	}
	m, err := c.myReviews(ctx, ref)
	if err != nil {
		return err
	}
	pending := m.pending

	var bodies []string
	if pending != nil && strings.TrimSpace(pending.Body) != "" {
		bodies = append(bodies, pending.Body)
	}
	comments := []reviewComment{}
	var replies []forge.Draft
	stale := 0
	for _, d := range local {
		switch {
		case d.ThreadID != "":
			replies = append(replies, d)
		case d.Anchor != nil:
			if d.Anchor.Diff.Head != "" && d.Anchor.Diff.Head != pr.Head.SHA {
				stale++
			}
			comments = append(comments, position(d.Anchor.Path, d.Body, d.Anchor.NewLine, d.Anchor.OldLine))
		default:
			bodies = append(bodies, d.Body)
		}
	}
	if stale > 0 {
		return fmt.Errorf("forgejo: %d of your inline comments were written on an earlier head of this pull request, so their lines may have moved; delete them and comment again on today's diff", stale)
	}
	body := strings.Join(bodies, "\n\n")
	content := body != "" || len(comments) > 0 || (pending != nil && pending.Comments > 0)
	// Only replies left, and the verdict already standing on this head: a
	// retry after a reply failed. Forgejo holds the verdict, so post only the
	// replies rather than publish it again.
	review := content || (verdict != forge.VerdictCommented && !m.stands(event, pr.Head.SHA))
	if review && verdict == forge.VerdictChangesRequested && strings.TrimSpace(body) == "" {
		return errors.New("forgejo: Forgejo needs a comment on the whole change to request changes; add one, then request changes")
	}
	if !review && len(replies) == 0 {
		return errors.New("forgejo: you have no pending comments to submit")
	}
	if review {
		if _, err := c.write(ctx, http.MethodPost, prPath(ref)+"/reviews", map[string]any{
			"event": event, "body": body, "commit_id": pr.Head.SHA, "comments": comments,
		}); err != nil {
			return err
		}
		// The review is public now: drop what it published, so a retry of
		// the replies never publishes it twice.
		if err := s.SaveDrafts(ref, replies); err != nil {
			return fmt.Errorf("forgejo: your review was published, but vuoro could not clear its pending comments; delete them before submitting again: %w", err)
		}
	}

	// Only replies are pending now: everything else either was published
	// with the review, or there was nothing else.
	remaining := slices.Clone(replies)
	var failed []string
	for _, d := range replies {
		if err := c.reply(ctx, ref, d); err != nil {
			failed = append(failed, err.Error())
			continue
		}
		remaining = slices.DeleteFunc(remaining, func(x forge.Draft) bool { return x.ID == d.ID })
		if err := s.SaveDrafts(ref, remaining); err != nil {
			return fmt.Errorf("forgejo: a reply was published, but vuoro could not clear it; delete it before submitting again: %w", err)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("forgejo: %d of %d replies were not posted and are still pending: %s", len(failed), len(replies), strings.Join(failed, "; "))
	}
	return nil
}

// reply posts d into the thread it answers. A thread on a line is a comment
// of its review at the thread's path and position, which Forgejo shows in the
// same conversation; a general thread takes an issue comment.
func (c *Client) reply(ctx context.Context, ref forge.Ref, d forge.Draft) error {
	if strings.HasPrefix(d.ThreadID, "comment-") || strings.HasPrefix(d.ThreadID, reviewPrefix) {
		_, err := c.write(ctx, http.MethodPost, fmt.Sprintf("%s/issues/%d/comments", repoPath(ref), ref.Number), map[string]any{"body": d.Body})
		return err
	}
	n, err := commentID(d.ThreadID)
	if err != nil {
		return err
	}
	reviews, err := pages[apiReview](ctx, c, prPath(ref)+"/reviews")
	if err != nil {
		return err
	}
	for _, r := range reviews {
		if r.State == "PENDING" || r.Comments == 0 {
			continue
		}
		cs, err := read[[]apiComment](ctx, c, fmt.Sprintf("%s/reviews/%d/comments", prPath(ref), r.ID))
		if err != nil {
			return err
		}
		for _, cm := range cs {
			if cm.ID != n {
				continue
			}
			_, err := c.write(ctx, http.MethodPost, fmt.Sprintf("%s/reviews/%d/comments", prPath(ref), r.ID), position(cm.Path, d.Body, cm.Position, cm.OldPosition))
			return err
		}
	}
	return fmt.Errorf("forgejo: the thread %s is no longer on this pull request", d.ThreadID)
}

// Approve approves with a review of its own. Withdrawing an approval is not
// something a reviewer can do on Forgejo.
func (c *Client) Approve(ctx context.Context, ref forge.Ref, approve bool) error {
	if !approve {
		return fmt.Errorf("forgejo: a reviewer cannot withdraw their own approval: %w", forge.ErrUnsupported)
	}
	if c.token.Empty() {
		return errNoToken
	}
	m, err := c.myReviews(ctx, ref)
	if err != nil {
		return err
	}
	// Forgejo merges a new review into the person's pending one, which
	// would publish it with the approval.
	if m.pending != nil {
		return errors.New("forgejo: you have a review in progress that you started in Forgejo; approving would publish it, so use Submit and approve")
	}
	pr, err := read[apiPR](ctx, c, prPath(ref))
	if err != nil {
		return err
	}
	_, err = c.write(ctx, http.MethodPost, prPath(ref)+"/reviews", map[string]any{"event": "APPROVED", "body": "", "commit_id": pr.Head.SHA})
	return err
}

func (c *Client) Resolve(context.Context, forge.Ref, string, bool) error {
	return fmt.Errorf("forgejo: Forgejo's API cannot resolve a thread: %w", forge.ErrUnsupported)
}
