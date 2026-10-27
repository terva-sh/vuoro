package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/terva-sh/vuoro/forge"
)

var (
	_ forge.ProjectLister = (*Client)(nil)
	_ forge.Administrator = (*Client)(nil)
	_ forge.TriageReader  = (*Client)(nil)
)

// ParseProjectURL reads a GitHub repository URL, owner and name.
func ParseProjectURL(raw string) (forge.Project, error) { return forge.ProjectURL(raw, 2) }

type gqlRepoID struct {
	NameWithOwner string `json:"nameWithOwner"`
	URL           string `json:"url"`
}

func (c *Client) repoProject(r gqlRepoID) (forge.Project, error) {
	got, err := ParseProjectURL(r.URL)
	if err != nil || got.Host != c.web.Host || !strings.EqualFold(got.Path, r.NameWithOwner) {
		return forge.Project{}, errors.New("github: invalid repository identity")
	}
	return forge.Project{Host: c.web.Host, Path: r.NameWithOwner, Name: r.NameWithOwner, WebURL: r.URL}, nil
}

const repoSearchQuery = `query($q: String!) { search(query: $q, type: REPOSITORY, first: 20) { nodes { ... on Repository { nameWithOwner url } } } }`

const viewerReposQuery = `query { viewer { repositories(first: 20, affiliations: [OWNER, COLLABORATOR, ORGANIZATION_MEMBER], ownerAffiliations: [OWNER, COLLABORATOR, ORGANIZATION_MEMBER], orderBy: {field: PUSHED_AT, direction: DESC}) { nodes { nameWithOwner url } } } }`

// SearchProjects finds repositories the account can see by name. With no
// query it lists the account's own, most recently pushed first. It returns
// one page: a search narrows, and the person types more.
func (c *Client) SearchProjects(ctx context.Context, query string) ([]forge.Project, error) {
	var rows []gqlRepoID
	if query = strings.TrimSpace(query); query == "" {
		var out struct {
			Viewer struct {
				Repositories struct {
					Nodes []gqlRepoID `json:"nodes"`
				} `json:"repositories"`
			} `json:"viewer"`
		}
		if err := c.query(ctx, viewerReposQuery, nil, &out); err != nil {
			return nil, err
		}
		rows = out.Viewer.Repositories.Nodes
	} else {
		var out struct {
			Search struct {
				Nodes []gqlRepoID `json:"nodes"`
			} `json:"search"`
		}
		if err := c.query(ctx, repoSearchQuery, map[string]any{"q": query + " in:name fork:true archived:false"}, &out); err != nil {
			return nil, err
		}
		rows = out.Search.Nodes
	}
	out := make([]forge.Project, 0, len(rows))
	for _, r := range rows {
		p, err := c.repoProject(r)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

// Project reads one repository by owner/name. The answer's path is GitHub's,
// which corrects the case of one typed or pasted.
func (c *Client) Project(ctx context.Context, path string) (forge.Project, error) {
	owner, name, ok := strings.Cut(path, "/")
	if !ok || strings.Contains(name, "/") {
		return forge.Project{}, forge.ErrNotAProject
	}
	var out struct {
		Repository *gqlRepoID `json:"repository"`
	}
	if err := c.query(ctx, `query($owner: String!, $name: String!) { repository(owner: $owner, name: $name) { nameWithOwner url } }`,
		map[string]any{"owner": owner, "name": name}, &out); err != nil {
		return forge.Project{}, err
	}
	if out.Repository == nil {
		return forge.Project{}, forge.ErrNotFound
	}
	return c.repoProject(*out.Repository)
}

const openPRsQuery = `query($owner: String!, $name: String!, $after: String) { repository(owner: $owner, name: $name) {
	pullRequests(states: OPEN, first: 100, after: $after, orderBy: {field: CREATED_AT, direction: ASC}) {
		pageInfo { hasNextPage endCursor } nodes { number url state updatedAt } } } }`

// ListOpen returns every open pull request in the repository. Like Discover
// it reads the whole list, so one that closed leaves it.
func (c *Client) ListOpen(ctx context.Context, path string) ([]forge.Discovered, error) {
	owner, name, _ := strings.Cut(path, "/")
	var out []forge.Discovered
	seen := map[int]bool{}
	var after any
	for page := 1; ; page++ {
		if page > maxPages {
			return nil, errors.New("github: repository listing exceeds page limit")
		}
		var res struct {
			Repository *struct {
				PullRequests struct {
					PageInfo pageInfo `json:"pageInfo"`
					Nodes    []struct {
						Number    int       `json:"number"`
						URL       string    `json:"url"`
						State     string    `json:"state"`
						UpdatedAt time.Time `json:"updatedAt"`
					} `json:"nodes"`
				} `json:"pullRequests"`
			} `json:"repository"`
		}
		if err := c.query(ctx, openPRsQuery, map[string]any{"owner": owner, "name": name, "after": after}, &res); err != nil {
			return nil, err
		}
		if res.Repository == nil {
			return nil, forge.ErrNotFound
		}
		prs := res.Repository.PullRequests
		for _, row := range prs.Nodes {
			ref, err := ParseURL(row.URL)
			if err != nil || ref.Host != c.web.Host || !strings.EqualFold(ref.Project, path) || ref.Number != row.Number || row.State != "OPEN" {
				return nil, errors.New("github: invalid listed identity")
			}
			if seen[ref.Number] {
				return nil, errors.New("github: repository changed while listing; refresh again")
			}
			seen[ref.Number] = true
			out = append(out, forge.Discovered{Ref: ref, Updated: row.UpdatedAt})
		}
		if !prs.PageInfo.HasNextPage {
			return out, nil
		}
		after = prs.PageInfo.EndCursor
	}
}

// Administer makes the person's administrative edits, one call each, in a
// fixed order. GitHub's REST endpoints for these add and remove, never
// replace, so a colleague's edit since the person looked survives (D65). A
// failure names the edits already made.
func (c *Client) Administer(ctx context.Context, ref forge.Ref, ch forge.Changes) error {
	if c.token.Empty() {
		return forge.ErrUnsupported
	}
	if !strings.EqualFold(ref.Host, c.web.Host) || len(strings.Split(ref.Project, "/")) != 2 || ref.Number < 1 {
		return fmt.Errorf("github: %s does not belong to this client", ref)
	}
	pull := fmt.Sprintf("%s/pulls/%d", repoPath(ref), ref.Number)
	issue := fmt.Sprintf("%s/issues/%d", repoPath(ref), ref.Number)
	type step struct {
		what string
		run  func() error
	}
	var steps []step
	if len(ch.AddReviewers) > 0 {
		steps = append(steps, step{"added reviewers", func() error {
			return c.write(ctx, http.MethodPost, pull+"/requested_reviewers", map[string]any{"reviewers": ch.AddReviewers})
		}})
	}
	if len(ch.RemoveReviewers) > 0 {
		steps = append(steps, step{"removed reviewers", func() error {
			return c.write(ctx, http.MethodDelete, pull+"/requested_reviewers", map[string]any{"reviewers": ch.RemoveReviewers})
		}})
	}
	if len(ch.AddAssignees) > 0 {
		steps = append(steps, step{"added assignees", func() error { return c.addAssignees(ctx, issue, ch.AddAssignees) }})
	}
	if len(ch.RemoveAssignees) > 0 {
		steps = append(steps, step{"removed assignees", func() error {
			return c.write(ctx, http.MethodDelete, issue+"/assignees", map[string]any{"assignees": ch.RemoveAssignees})
		}})
	}
	if len(ch.AddLabels) > 0 {
		steps = append(steps, step{"added labels", func() error {
			return c.write(ctx, http.MethodPost, issue+"/labels", map[string]any{"labels": ch.AddLabels})
		}})
	}
	for _, l := range ch.RemoveLabels {
		steps = append(steps, step{"removed label " + l, func() error {
			err := c.write(ctx, http.MethodDelete, issue+"/labels/"+url.PathEscape(l), nil)
			if errors.Is(err, forge.ErrNotFound) {
				return nil // already gone
			}
			return err
		}})
	}
	if ch.Draft != nil {
		draft := *ch.Draft
		steps = append(steps, step{"set the draft state", func() error { return c.setDraft(ctx, ref, draft) }})
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

// write sends one REST write to a path under the API.
func (c *Client) write(ctx context.Context, method, path string, body any) error {
	_, err := c.do(ctx, method, c.rest+path, path, body)
	return err
}

// addAssignees adds assignees. GitHub answers success and drops, unsaid, an
// account that cannot be assigned, so the answer is checked for each.
func (c *Client) addAssignees(ctx context.Context, issue string, names []string) error {
	path := issue + "/assignees"
	b, err := c.do(ctx, http.MethodPost, c.rest+path, path, map[string]any{"assignees": names})
	if err != nil {
		return err
	}
	var got struct {
		Assignees []struct {
			Login string `json:"login"`
		} `json:"assignees"`
	}
	if json.Unmarshal(b, &got) != nil {
		return errors.New("github: invalid assignees response")
	}
	for _, n := range names {
		if !slices.ContainsFunc(got.Assignees, func(a struct {
			Login string `json:"login"`
		}) bool {
			return strings.EqualFold(a.Login, n)
		}) {
			return fmt.Errorf("github: %s cannot be assigned to this pull request", n)
		}
	}
	return nil
}

// setDraft converts a pull request to a draft or marks it ready for review,
// which only GraphQL can do.
func (c *Client) setDraft(ctx context.Context, ref forge.Ref, draft bool) error {
	owner, name := ownerName(ref)
	var id struct {
		Repository *struct {
			PullRequest *struct {
				ID string `json:"id"`
			} `json:"pullRequest"`
		} `json:"repository"`
	}
	if err := c.query(ctx, `query($owner: String!, $name: String!, $number: Int!) { repository(owner: $owner, name: $name) { pullRequest(number: $number) { id } } }`,
		map[string]any{"owner": owner, "name": name, "number": ref.Number}, &id); err != nil {
		return err
	}
	if id.Repository == nil || id.Repository.PullRequest == nil || id.Repository.PullRequest.ID == "" {
		return fmt.Errorf("%s: %w", ref, forge.ErrNotFound)
	}
	mutation := `mutation($id: ID!) { markPullRequestReadyForReview(input: {pullRequestId: $id}) { pullRequest { isDraft } } }`
	if draft {
		mutation = `mutation($id: ID!) { convertPullRequestToDraft(input: {pullRequestId: $id}) { pullRequest { isDraft } } }`
	}
	var out map[string]*struct {
		PullRequest struct {
			IsDraft bool `json:"isDraft"`
		} `json:"pullRequest"`
	}
	if err := c.query(ctx, mutation, map[string]any{"id": id.Repository.PullRequest.ID}, &out); err != nil {
		return err
	}
	for _, v := range out {
		if v == nil || v.PullRequest.IsDraft != draft {
			return errors.New("github: the draft state did not change")
		}
	}
	if len(out) == 0 {
		return errors.New("github: the draft mutation answered nothing")
	}
	return nil
}

// maxCandidatePages bounds a members or labels listing.
const maxCandidatePages = 10

// Members lists the accounts that can be assigned to the repository's pull
// requests, which are the accounts that can be asked to review, matching
// query.
func (c *Client) Members(ctx context.Context, project, query string) ([]forge.Member, error) {
	if c.token.Empty() {
		return nil, forge.ErrUnsupported
	}
	owner, name, _ := strings.Cut(project, "/")
	out := []forge.Member{}
	var after any
	for range maxCandidatePages {
		var res struct {
			Repository *struct {
				AssignableUsers struct {
					PageInfo pageInfo `json:"pageInfo"`
					Nodes    []struct {
						Login string `json:"login"`
						Name  string `json:"name"`
					} `json:"nodes"`
				} `json:"assignableUsers"`
			} `json:"repository"`
		}
		q := `query($owner: String!, $name: String!, $query: String, $after: String) { repository(owner: $owner, name: $name) {
			assignableUsers(first: 100, query: $query, after: $after) { pageInfo { hasNextPage endCursor } nodes { login name } } } }`
		if err := c.query(ctx, q, map[string]any{"owner": owner, "name": name, "query": strings.TrimSpace(query), "after": after}, &res); err != nil {
			return nil, err
		}
		if res.Repository == nil {
			return nil, forge.ErrNotFound
		}
		for _, u := range res.Repository.AssignableUsers.Nodes {
			out = append(out, forge.Member{Username: u.Login, Name: u.Name})
		}
		if !res.Repository.AssignableUsers.PageInfo.HasNextPage {
			break
		}
		after = res.Repository.AssignableUsers.PageInfo.EndCursor
	}
	return out, nil
}

// Labels lists the repository's labels.
func (c *Client) Labels(ctx context.Context, project string) ([]string, error) {
	if c.token.Empty() {
		return nil, forge.ErrUnsupported
	}
	owner, name, _ := strings.Cut(project, "/")
	out := []string{}
	var after any
	for range maxCandidatePages {
		var res struct {
			Repository *struct {
				Labels struct {
					PageInfo pageInfo `json:"pageInfo"`
					Nodes    []struct {
						Name string `json:"name"`
					} `json:"nodes"`
				} `json:"labels"`
			} `json:"repository"`
		}
		q := `query($owner: String!, $name: String!, $after: String) { repository(owner: $owner, name: $name) {
			labels(first: 100, after: $after) { pageInfo { hasNextPage endCursor } nodes { name } } } }`
		if err := c.query(ctx, q, map[string]any{"owner": owner, "name": name, "after": after}, &res); err != nil {
			return nil, err
		}
		if res.Repository == nil {
			return nil, forge.ErrNotFound
		}
		for _, l := range res.Repository.Labels.Nodes {
			out = append(out, l.Name)
		}
		if !res.Repository.Labels.PageInfo.HasNextPage {
			break
		}
		after = res.Repository.Labels.PageInfo.EndCursor
	}
	return out, nil
}

// codeOwnersPaths are where GitHub looks for CODEOWNERS, in its order.
var codeOwnersPaths = []string{".github/CODEOWNERS", "CODEOWNERS", "docs/CODEOWNERS"}

// maxCodeOwners bounds the CODEOWNERS text given to an agent.
const maxCodeOwners = 64 << 10

// CodeOwners reads the first CODEOWNERS GitHub would use at ref.
func (c *Client) CodeOwners(ctx context.Context, project, ref string) (string, error) {
	for _, p := range codeOwnersPaths {
		b, err := c.get(ctx, repoPath(forge.Ref{Project: project})+"/contents/"+p, url.Values{"ref": {ref}})
		if errors.Is(err, forge.ErrNotFound) {
			continue
		}
		if err != nil {
			return "", err
		}
		var file struct {
			Encoding string `json:"encoding"`
			Content  string `json:"content"`
		}
		if json.Unmarshal(b, &file) != nil || file.Encoding != "base64" {
			return "", errors.New("github: invalid contents response")
		}
		text, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(file.Content, "\n", ""))
		if err != nil {
			return "", errors.New("github: invalid contents encoding")
		}
		if len(text) > maxCodeOwners {
			text = text[:maxCodeOwners]
		}
		return string(text), nil
	}
	return "", nil
}

// RecentAuthors reads up to thirty of the year's commits to path on ref, and
// returns their authors' logins, or names where GitHub links no account.
func (c *Client) RecentAuthors(ctx context.Context, project, ref, path string) ([]string, error) {
	q := url.Values{"sha": {ref}, "path": {path}, "per_page": {"30"}, "since": {time.Now().AddDate(-1, 0, 0).UTC().Format(time.RFC3339)}}
	b, err := c.get(ctx, repoPath(forge.Ref{Project: project})+"/commits", q)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Author *struct {
			Login string `json:"login"`
		} `json:"author"`
		Commit struct {
			Author struct {
				Name string `json:"name"`
			} `json:"author"`
		} `json:"commit"`
	}
	if json.Unmarshal(b, &rows) != nil || rows == nil {
		return nil, errors.New("github: invalid commits response")
	}
	out := []string{}
	for _, r := range rows {
		who := r.Commit.Author.Name
		if r.Author != nil && r.Author.Login != "" {
			who = r.Author.Login
		}
		if who != "" && !slices.Contains(out, who) {
			out = append(out, who)
		}
	}
	return out, nil
}
