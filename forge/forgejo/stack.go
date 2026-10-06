package forgejo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/terva-sh/vuoro/forge"
)

var _ forge.Stacker = (*Client)(nil)

// StackTarget reads the pull request's head branch and commit, and refuses
// one whose head is in a fork, or in a fork since deleted.
func (c *Client) StackTarget(ctx context.Context, ref forge.Ref) (forge.StackTarget, error) {
	pr, err := read[apiPR](ctx, c, prPath(ref))
	if err != nil {
		return forge.StackTarget{}, err
	}
	if pr.Head.Repo == nil || pr.Base.Repo == nil || !strings.EqualFold(pr.Head.Repo.FullName, pr.Base.Repo.FullName) {
		return forge.StackTarget{}, forge.ErrFork
	}
	if !forge.ValidBranch(pr.Head.Ref) {
		return forge.StackTarget{}, errors.New("forgejo: the pull request's head branch has an unsafe name")
	}
	return forge.StackTarget{Branch: pr.Head.Ref, Head: pr.Head.SHA}, nil
}

// OpenStacked opens a draft pull request in the repository. Forgejo marks one
// draft by a title prefix, WIP: or [WIP] unless the instance configures
// others, and its API neither names them nor sets a draft another way. With
// others it opens as an ordinary pull request, which is returned with
// forge.ErrNotDraft for the person to mark.
func (c *Client) OpenStacked(ctx context.Context, ref forge.Ref, s forge.NewStacked) (string, error) {
	title := s.Title
	if l := strings.ToLower(title); !strings.HasPrefix(l, "wip:") && !strings.HasPrefix(l, "[wip]") {
		title = "WIP: " + title
	}
	b, err := c.write(ctx, "POST", repoPath(ref)+"/pulls", map[string]any{
		"head": s.Branch, "base": s.Target, "title": title, "body": s.Description,
	})
	if err != nil {
		return "", err
	}
	var pr struct {
		HTMLURL string `json:"html_url"`
		Draft   bool   `json:"draft"`
	}
	if err := json.Unmarshal(b, &pr); err != nil || pr.HTMLURL == "" {
		return "", fmt.Errorf("forgejo: the new pull request: %v", err)
	}
	if !pr.Draft {
		return pr.HTMLURL, fmt.Errorf("forgejo: the pull request %w: this instance marks drafts by other title prefixes", forge.ErrNotDraft)
	}
	return pr.HTMLURL, nil
}
