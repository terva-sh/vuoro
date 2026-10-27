package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/terva-sh/vuoro/forge"
)

var _ forge.BranchUpdater = (*Client)(nil)

// rebasePoll is how often a rebase GitLab runs in the background is read.
var rebasePoll = time.Second

// UpdateBranch has GitLab rebase the merge request's source branch onto its
// target, and waits for it to finish. GitLab has no merge of the target into
// the source branch, and its rebase takes no expected head, so the head is
// read first: a push landing between the two is the one it cannot refuse.
func (c *Client) UpdateBranch(ctx context.Context, ref forge.Ref, strategy, head string) error {
	if strategy != forge.UpdateRebase {
		return forge.ErrUnsupported
	}
	if c.token.Empty() {
		return errNoToken
	}
	mr, err := c.rebaseState(ctx, ref)
	if err != nil {
		return err
	}
	if mr.SHA != head {
		return forge.ErrHeadMoved
	}
	if _, err := c.write(ctx, "PUT", mrPath(ref)+"/rebase", nil); err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("gitlab: the rebase is still running: %w", ctx.Err())
		case <-time.After(rebasePoll):
		}
		mr, err := c.rebaseState(ctx, ref)
		switch {
		case err != nil:
			return err
		case mr.InProgress:
			continue
		case mr.Error != "":
			return fmt.Errorf("gitlab: the rebase failed: %s", mr.Error)
		}
		return nil
	}
}

type rebaseState struct {
	SHA        string `json:"sha"`
	InProgress bool   `json:"rebase_in_progress"`
	Error      string `json:"merge_error"`
}

func (c *Client) rebaseState(ctx context.Context, ref forge.Ref) (rebaseState, error) {
	var mr rebaseState
	b, _, err := c.do(ctx, "GET", mrPath(ref), url.Values{"include_rebase_in_progress": {"true"}}, nil)
	if err != nil {
		return mr, err
	}
	if err := json.Unmarshal(b, &mr); err != nil {
		return mr, errors.New("gitlab: invalid merge request")
	}
	return mr, nil
}
