package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/terva-sh/vuoro/forge"
)

var _ forge.Stacker = (*Client)(nil)

const stackQuery = `query($owner: String!, $name: String!, $number: Int!) { repository(owner: $owner, name: $name) { pullRequest(number: $number) {
	headRefName headRefOid isCrossRepository headRepository { nameWithOwner } } } }`

// StackTarget reads the pull request's head branch and commit, and refuses
// one whose head is in a fork, or in a fork since deleted.
func (c *Client) StackTarget(ctx context.Context, ref forge.Ref) (forge.StackTarget, error) {
	owner, name := ownerName(ref)
	var out struct {
		Repository *struct {
			PullRequest *struct {
				HeadRefName       string `json:"headRefName"`
				HeadRefOID        string `json:"headRefOid"`
				IsCrossRepository bool   `json:"isCrossRepository"`
				HeadRepository    *struct {
					NameWithOwner string `json:"nameWithOwner"`
				} `json:"headRepository"`
			} `json:"pullRequest"`
		} `json:"repository"`
	}
	if err := c.query(ctx, stackQuery, map[string]any{"owner": owner, "name": name, "number": ref.Number}, &out); err != nil {
		return forge.StackTarget{}, err
	}
	if out.Repository == nil || out.Repository.PullRequest == nil {
		return forge.StackTarget{}, forge.ErrNotFound
	}
	p := out.Repository.PullRequest
	if p.IsCrossRepository || p.HeadRepository == nil || !strings.EqualFold(p.HeadRepository.NameWithOwner, ref.Project) {
		return forge.StackTarget{}, forge.ErrFork
	}
	if !forge.ValidBranch(p.HeadRefName) {
		return forge.StackTarget{}, errors.New("github: the pull request's head branch has an unsafe name")
	}
	return forge.StackTarget{Branch: p.HeadRefName, Head: p.HeadRefOID}, nil
}

// OpenStacked opens a draft pull request in the repository. GitHub refuses a
// draft where the plan has none, such as a private repository on a free
// plan; the pull request then opens as an ordinary one, which is returned
// with forge.ErrNotDraft for the person to mark.
func (c *Client) OpenStacked(ctx context.Context, ref forge.Ref, s forge.NewStacked) (string, error) {
	path := repoPath(ref) + "/pulls"
	body := map[string]any{"head": s.Branch, "base": s.Target, "title": s.Title, "body": s.Description, "draft": true}
	b, err := c.do(ctx, http.MethodPost, c.rest+path, path, body)
	draft := true
	var api *forge.APIError
	if errors.As(err, &api) && api.Status == http.StatusUnprocessableEntity && strings.Contains(strings.ToLower(api.Message), "draft") {
		draft = false
		body["draft"] = false
		b, err = c.do(ctx, http.MethodPost, c.rest+path, path, body)
	}
	if err != nil {
		return "", err
	}
	var pr struct {
		HTMLURL string `json:"html_url"`
		Draft   bool   `json:"draft"`
	}
	if err := json.Unmarshal(b, &pr); err != nil || pr.HTMLURL == "" {
		return "", fmt.Errorf("github: the new pull request: %v", err)
	}
	if !draft || !pr.Draft {
		return pr.HTMLURL, fmt.Errorf("github: the pull request %w: GitHub does not offer drafts in this repository", forge.ErrNotDraft)
	}
	return pr.HTMLURL, nil
}
