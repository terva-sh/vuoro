package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/terva-sh/vuoro/forge"
)

var _ forge.BranchUpdater = (*Client)(nil)

const headQuery = `query($owner: String!, $name: String!, $number: Int!) { repository(owner: $owner, name: $name) { pullRequest(number: $number) { id headRefOid } } }`

const rebaseMutation = `mutation($id: ID!, $head: GitObjectID!) { updatePullRequestBranch(input: {pullRequestId: $id, expectedHeadOid: $head, updateMethod: REBASE}) { pullRequest { headRefOid } } }`

// UpdateBranch has GitHub update the pull request's head branch from its
// base. A merge goes through REST, which takes the expected head; a rebase
// through GraphQL, which REST does not offer, also against the expected head.
func (c *Client) UpdateBranch(ctx context.Context, ref forge.Ref, strategy, head string) error {
	if c.token.Empty() {
		return forge.ErrUnsupported
	}
	switch strategy {
	case forge.UpdateMerge:
		path := fmt.Sprintf("%s/pulls/%d/update-branch", repoPath(ref), ref.Number)
		_, err := c.do(ctx, http.MethodPut, c.rest+path, path, map[string]any{"expected_head_sha": head})
		var api *forge.APIError
		if errors.As(err, &api) && api.Status == http.StatusUnprocessableEntity && strings.Contains(strings.ToLower(api.Message), "expected head sha") {
			return forge.ErrHeadMoved
		}
		return err
	case forge.UpdateRebase:
		owner, name := ownerName(ref)
		var pr struct {
			Repository *struct {
				PullRequest *struct {
					ID         string `json:"id"`
					HeadRefOID string `json:"headRefOid"`
				} `json:"pullRequest"`
			} `json:"repository"`
		}
		if err := c.query(ctx, headQuery, map[string]any{"owner": owner, "name": name, "number": ref.Number}, &pr); err != nil {
			return err
		}
		if pr.Repository == nil || pr.Repository.PullRequest == nil {
			return forge.ErrNotFound
		}
		if pr.Repository.PullRequest.HeadRefOID != head {
			return forge.ErrHeadMoved
		}
		var out struct{}
		return c.query(ctx, rebaseMutation, map[string]any{"id": pr.Repository.PullRequest.ID, "head": head}, &out)
	}
	return forge.ErrUnsupported
}
