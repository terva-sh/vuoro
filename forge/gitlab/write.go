package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"

	"github.com/terva-sh/vuoro/forge"
)

var _ forge.Writer = (*Client)(nil)

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

// Submit publishes every draft note of the person's in one batch.
func (c *Client) Submit(ctx context.Context, ref forge.Ref) error {
	_, err := c.write(ctx, "POST", mrPath(ref)+"/draft_notes/bulk_publish", nil)
	return err
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
	return err
}
