package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/terva-sh/vuoro/forge"
)

var (
	_ forge.Writer  = (*Client)(nil)
	_ forge.Stacker = (*Client)(nil)
)

func mrPath(ref forge.Ref) string {
	return fmt.Sprintf("%s/merge_requests/%d", projectPath(ref.Project), ref.Number)
}

// errNoToken is a write attempted without the person's token.
var errNoToken = errors.New("gitlab: writing needs your token; name a token_command for this forge in the config file")

func (c *Client) write(ctx context.Context, method, path string, body any) ([]byte, error) {
	if c.token.Empty() {
		return nil, errNoToken
	}
	b, _, err := c.do(ctx, method, path, nil, body)
	return b, err
}

// CreateDraft adds a draft note, GitLab's pending comment, which only its
// author sees until it is published.
func (c *Client) CreateDraft(ctx context.Context, ref forge.Ref, d forge.NewDraft) (forge.Draft, error) {
	body := map[string]any{"note": d.Body}
	switch {
	case d.ThreadID != "":
		body["in_reply_to_discussion_id"] = d.ThreadID
	case d.Anchor != nil:
		a := d.Anchor
		old := a.OldPath
		if old == "" {
			old = a.Path
		}
		pos := map[string]any{
			"position_type": "text",
			"base_sha":      a.Diff.Base, "start_sha": a.Diff.Start, "head_sha": a.Diff.Head,
			"new_path": a.Path, "old_path": old,
		}
		// GitLab places a comment on an added line by new_line, a removed
		// one by old_line, and an unchanged one by both.
		if a.NewLine > 0 {
			pos["new_line"] = a.NewLine
		}
		if a.OldLine > 0 {
			pos["old_line"] = a.OldLine
		}
		body["position"] = pos
	}
	b, err := c.write(ctx, "POST", mrPath(ref)+"/draft_notes", body)
	if err != nil {
		return forge.Draft{}, err
	}
	var rd restDraft
	if err := json.Unmarshal(b, &rd); err != nil {
		return forge.Draft{}, fmt.Errorf("gitlab: draft note: %w", err)
	}
	return forge.Draft{ID: strconv.Itoa(rd.ID), Body: rd.Note, ThreadID: rd.DiscussionID, Anchor: d.Anchor}, nil
}

func draftPath(ref forge.Ref, id string) (string, error) {
	if _, err := strconv.Atoi(id); err != nil {
		return "", fmt.Errorf("gitlab: bad draft note id %q", id)
	}
	return mrPath(ref) + "/draft_notes/" + id, nil
}

func (c *Client) UpdateDraft(ctx context.Context, ref forge.Ref, id, body string) error {
	p, err := draftPath(ref, id)
	if err != nil {
		return err
	}
	_, err = c.write(ctx, "PUT", p, map[string]any{"note": body})
	return err
}

func (c *Client) DeleteDraft(ctx context.Context, ref forge.Ref, id string) error {
	p, err := draftPath(ref, id)
	if err != nil {
		return err
	}
	_, err = c.write(ctx, "DELETE", p, nil)
	return err
}

// Submit publishes every draft note of the person's in one batch. Asking for
// changes is the batch's reviewer_state; approving is a second call, after the
// comments it answers are published.
func (c *Client) Submit(ctx context.Context, ref forge.Ref, verdict forge.Verdict) error {
	var body map[string]any
	switch verdict {
	case forge.VerdictCommented, forge.VerdictApproved:
	case forge.VerdictChangesRequested:
		body = map[string]any{"reviewer_state": "requested_changes"}
	default:
		return fmt.Errorf("gitlab: unknown verdict %q", verdict)
	}
	if _, err := c.write(ctx, "POST", mrPath(ref)+"/draft_notes/bulk_publish", body); err != nil {
		return err
	}
	switch verdict {
	case forge.VerdictApproved:
		return c.Approve(ctx, ref, true)
	case forge.VerdictChangesRequested:
		return c.requestedChanges(ctx, ref)
	}
	return nil
}

// errChangesNotRecorded is a bulk publish that succeeded without recording
// the person's request for changes.
var errChangesNotRecorded = errors.New("gitlab: your comments were published, but GitLab did not record that you requested changes. It records that only for the merge request's reviewers, and a GitLab older than this feature ignores it. Request changes in GitLab")

// requestedChanges reads the merge request back and confirms the person's
// reviewer state, because GitLab answers a bulk publish with success whether
// or not it understood reviewer_state, and a verdict the person chose must
// not be lost without their knowing.
func (c *Client) requestedChanges(ctx context.Context, ref forge.Ref) error {
	b, _, err := c.do(ctx, "GET", "/api/v4/user", nil, nil)
	if err != nil {
		return err
	}
	var me struct {
		Username string `json:"username"`
	}
	if json.Unmarshal(b, &me) != nil || me.Username == "" {
		return errors.New("gitlab: your comments were published, but vuoro could not read who you are to confirm the requested changes")
	}
	ch, err := c.ReadFacts(ctx, ref)
	if err != nil {
		return err
	}
	for _, r := range ch.Reviewers {
		if strings.EqualFold(r.Username, me.Username) && r.State == forge.RequestedChanges {
			return nil
		}
	}
	return errChangesNotRecorded
}

func (c *Client) Resolve(ctx context.Context, ref forge.Ref, threadID string, resolved bool) error {
	if threadID == "" {
		return errors.New("gitlab: no thread to resolve")
	}
	_, err := c.write(ctx, "PUT", mrPath(ref)+"/discussions/"+url.PathEscape(threadID), map[string]any{"resolved": resolved})
	return err
}

func (c *Client) Approve(ctx context.Context, ref forge.Ref, approve bool) error {
	verb := "/approve"
	if !approve {
		verb = "/unapprove"
	}
	_, err := c.write(ctx, "POST", mrPath(ref)+verb, nil)
	// GitLab answers 401, not 403, when this person may not approve this
	// merge request, which reads as a bad token unless it is said.
	if ae := (*APIError)(nil); errors.As(err, &ae) && ae.Status == http.StatusUnauthorized {
		return fmt.Errorf("%w. GitLab refuses an approval this way when you may not approve this merge request, such as your own when the project forbids that, as well as when the token is not accepted", err)
	}
	return err
}

// StackTarget reads the merge request's source branch and head, and refuses
// one from a fork.
func (c *Client) StackTarget(ctx context.Context, ref forge.Ref) (forge.StackTarget, error) {
	b, _, err := c.do(ctx, "GET", mrPath(ref), nil, nil)
	if err != nil {
		return forge.StackTarget{}, err
	}
	var mr struct {
		SourceProject int64  `json:"source_project_id"`
		TargetProject int64  `json:"target_project_id"`
		SourceBranch  string `json:"source_branch"`
		SHA           string `json:"sha"`
	}
	if err := json.Unmarshal(b, &mr); err != nil {
		return forge.StackTarget{}, fmt.Errorf("gitlab: merge request: %w", err)
	}
	if mr.SourceProject != mr.TargetProject {
		return forge.StackTarget{}, forge.ErrFork
	}
	if !forge.ValidBranch(mr.SourceBranch) {
		return forge.StackTarget{}, errors.New("gitlab: the merge request's source branch has an unsafe name")
	}
	return forge.StackTarget{Branch: mr.SourceBranch, Head: mr.SHA}, nil
}

// OpenStacked opens a draft merge request in the project. GitLab marks one
// draft by its title.
func (c *Client) OpenStacked(ctx context.Context, ref forge.Ref, s forge.NewStacked) (string, error) {
	title := s.Title
	if !strings.HasPrefix(strings.ToLower(title), "draft:") {
		title = "Draft: " + title
	}
	b, err := c.write(ctx, "POST", projectPath(ref.Project)+"/merge_requests", map[string]any{
		"source_branch": s.Branch, "target_branch": s.Target, "title": title,
		"description": s.Description, "remove_source_branch": true,
	})
	if err != nil {
		return "", err
	}
	var mr struct {
		WebURL string `json:"web_url"`
	}
	if err := json.Unmarshal(b, &mr); err != nil || mr.WebURL == "" {
		return "", fmt.Errorf("gitlab: the new merge request: %v", err)
	}
	return mr.WebURL, nil
}
