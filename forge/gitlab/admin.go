package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/terva-sh/vuoro/forge"
)

var _ forge.Administrator = (*Client)(nil)

// setPeopleMutation adds or removes reviewers or assignees. The operation
// mode is always named: GitLab's default is REPLACE, which would drop a
// colleague's edit made since the person looked (D65).
const setPeopleMutation = `mutation set($projectPath: ID!, $iid: String!, $usernames: [String!]!, $mode: MutationOperationMode!) {
  %s(input: {projectPath: $projectPath, iid: $iid, %s: $usernames, operationMode: $mode}) { errors }
}`

const setDraftMutation = `mutation set($projectPath: ID!, $iid: String!, $draft: Boolean!) {
  mergeRequestSetDraft(input: {projectPath: $projectPath, iid: $iid, draft: $draft}) { errors }
}`

// Administer makes the person's administrative edits, one forge call each,
// in a fixed order. A failure names the edits already made, since GitLab has
// no transaction across them.
func (c *Client) Administer(ctx context.Context, ref forge.Ref, ch forge.Changes) error {
	if c.token.Empty() {
		return forge.ErrUnsupported
	}
	if ref.Host != c.base.Host {
		return fmt.Errorf("gitlab: %s is on %s, and this client talks to %s", ref, ref.Host, c.base.Host)
	}
	type step struct {
		what string
		run  func() error
	}
	people := func(mutation, field, mode string, names []string) func() error {
		return func() error {
			return c.mutate(ctx, mutation, fmt.Sprintf(setPeopleMutation, mutation, field),
				map[string]any{"projectPath": ref.Project, "iid": strconv.Itoa(ref.Number), "usernames": names, "mode": mode})
		}
	}
	var steps []step
	if len(ch.AddReviewers) > 0 {
		steps = append(steps, step{"added reviewers", people("mergeRequestSetReviewers", "reviewerUsernames", "APPEND", ch.AddReviewers)})
	}
	if len(ch.RemoveReviewers) > 0 {
		steps = append(steps, step{"removed reviewers", people("mergeRequestSetReviewers", "reviewerUsernames", "REMOVE", ch.RemoveReviewers)})
	}
	if len(ch.AddAssignees) > 0 {
		steps = append(steps, step{"added assignees", people("mergeRequestSetAssignees", "assigneeUsernames", "APPEND", ch.AddAssignees)})
	}
	if len(ch.RemoveAssignees) > 0 {
		steps = append(steps, step{"removed assignees", people("mergeRequestSetAssignees", "assigneeUsernames", "REMOVE", ch.RemoveAssignees)})
	}
	if len(ch.AddLabels)+len(ch.RemoveLabels) > 0 {
		steps = append(steps, step{"changed labels", func() error { return c.setLabels(ctx, ref, ch.AddLabels, ch.RemoveLabels) }})
	}
	if ch.Draft != nil {
		draft := *ch.Draft
		steps = append(steps, step{"set the draft state", func() error {
			return c.mutate(ctx, "mergeRequestSetDraft", setDraftMutation,
				map[string]any{"projectPath": ref.Project, "iid": strconv.Itoa(ref.Number), "draft": draft})
		}})
	}
	var done []string
	for _, s := range steps {
		if err := s.run(); err != nil {
			if len(done) > 0 {
				return fmt.Errorf("%w (already %s)", err, strings.Join(done, ", "))
			}
			return err
		}
		done = append(done, s.what)
	}
	return nil
}

// mutate runs one mutation, and fails on any error GitLab reports, at the top
// level or in the mutation's own errors, such as a username that matches no
// visible user.
func (c *Client) mutate(ctx context.Context, name, query string, vars map[string]any) error {
	b, _, err := c.do(ctx, "POST", "/api/graphql", nil, map[string]any{"query": query, "variables": vars})
	if err != nil {
		return err
	}
	var r struct {
		Data   map[string]*struct{ Errors []string } `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if json.Unmarshal(b, &r) != nil {
		return errors.New("gitlab: invalid GraphQL response")
	}
	if len(r.Errors) > 0 {
		return fmt.Errorf("gitlab: graphql: %s", c.scrub(r.Errors[0].Message))
	}
	got := r.Data[name]
	if got == nil {
		return fmt.Errorf("gitlab: %s answered nothing", name)
	}
	if len(got.Errors) > 0 {
		return fmt.Errorf("gitlab: %s", c.scrub(strings.Join(got.Errors, "; ")))
	}
	return nil
}

// setLabels adds and removes labels by name. REST takes both as deltas, and
// needs no lookup of label IDs, which GraphQL's mutation does.
func (c *Client) setLabels(ctx context.Context, ref forge.Ref, add, remove []string) error {
	for _, l := range append(append([]string{}, add...), remove...) {
		if l == "" || strings.Contains(l, ",") {
			return fmt.Errorf("gitlab: %q cannot be sent as a label name", l)
		}
	}
	body := map[string]string{}
	if len(add) > 0 {
		body["add_labels"] = strings.Join(add, ",")
	}
	if len(remove) > 0 {
		body["remove_labels"] = strings.Join(remove, ",")
	}
	_, _, err := c.do(ctx, "PUT", fmt.Sprintf("%s/merge_requests/%d", projectPath(ref.Project), ref.Number), nil, body)
	return err
}

// maxCandidatePages bounds a members or labels listing: a picker narrows by
// query rather than reading a huge group whole.
const maxCandidatePages = 10

// Members lists the project's members, inherited ones included, that match
// query. Blocked and pending accounts are left out, since they cannot review.
func (c *Client) Members(ctx context.Context, project, query string) ([]forge.Member, error) {
	if c.token.Empty() {
		return nil, forge.ErrUnsupported
	}
	out := []forge.Member{}
	err := c.pages(ctx, projectPath(project)+"/members/all", url.Values{"query": {strings.TrimSpace(query)}}, func(b []byte) (int, error) {
		var rows []struct {
			Username string `json:"username"`
			Name     string `json:"name"`
			State    string `json:"state"`
		}
		if json.Unmarshal(b, &rows) != nil || rows == nil {
			return 0, errors.New("gitlab: invalid members response")
		}
		for _, r := range rows {
			if r.State == "" || r.State == "active" {
				out = append(out, forge.Member{Username: r.Username, Name: r.Name})
			}
		}
		return len(rows), nil
	})
	return out, err
}

// Labels lists the labels the project's merge requests can carry, its
// ancestor groups' included.
func (c *Client) Labels(ctx context.Context, project string) ([]string, error) {
	if c.token.Empty() {
		return nil, forge.ErrUnsupported
	}
	out := []string{}
	err := c.pages(ctx, projectPath(project)+"/labels", url.Values{"include_ancestor_groups": {"true"}}, func(b []byte) (int, error) {
		var rows []struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(b, &rows) != nil || rows == nil {
			return 0, errors.New("gitlab: invalid labels response")
		}
		for _, r := range rows {
			out = append(out, r.Name)
		}
		return len(rows), nil
	})
	return out, err
}

// pages reads a REST listing a hundred at a time, up to maxCandidatePages.
func (c *Client) pages(ctx context.Context, path string, q url.Values, each func([]byte) (int, error)) error {
	for page := 1; page <= maxCandidatePages; page++ {
		q.Set("per_page", "100")
		q.Set("page", strconv.Itoa(page))
		b, h, err := c.do(ctx, "GET", path, q, nil)
		if err != nil {
			return err
		}
		n, err := each(b)
		if err != nil {
			return err
		}
		// A short page is the last; GitLab may also say so with an empty
		// X-Next-Page.
		if next := h.Get("X-Next-Page"); n < 100 || (next == "" && h.Get("X-Page") != "") {
			return nil
		}
	}
	return nil
}
