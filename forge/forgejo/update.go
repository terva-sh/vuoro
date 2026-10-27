package forgejo

import (
	"context"
	"net/url"

	"github.com/terva-sh/vuoro/forge"
)

var _ forge.BranchUpdater = (*Client)(nil)

// UpdateBranch has Forgejo merge the base into the pull request's head
// branch, or rebase the branch onto it. Forgejo's route takes no expected
// head, so the head is read first: a push landing between the two is the
// one it cannot refuse.
func (c *Client) UpdateBranch(ctx context.Context, ref forge.Ref, strategy, head string) error {
	if c.token.Empty() {
		return errNoToken
	}
	if strategy != forge.UpdateMerge && strategy != forge.UpdateRebase {
		return forge.ErrUnsupported
	}
	pr, err := read[apiPR](ctx, c, prPath(ref))
	if err != nil {
		return err
	}
	if pr.Head.SHA != head {
		return forge.ErrHeadMoved
	}
	_, err = c.do(ctx, "POST", prPath(ref)+"/update", url.Values{"style": {strategy}}, nil)
	return err
}
