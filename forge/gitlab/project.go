package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/terva-sh/vuoro/forge"
)

var (
	_ forge.ProjectLister    = (*Client)(nil)
	_ forge.DependencyReader = (*Client)(nil)
)

// ParseProjectURL reads a GitLab project URL, which may name a nested group.
func ParseProjectURL(raw string) (forge.Project, error) { return forge.ProjectURL(raw, 0) }

type restProject struct {
	PathWithNamespace string `json:"path_with_namespace"`
	NameWithNamespace string `json:"name_with_namespace"`
	WebURL            string `json:"web_url"`
}

func (c *Client) project(p restProject) (forge.Project, error) {
	got, err := ParseProjectURL(p.WebURL)
	if err != nil || got.Host != c.Host() || !strings.EqualFold(got.Path, p.PathWithNamespace) {
		return forge.Project{}, errors.New("gitlab: invalid project identity")
	}
	return forge.Project{Host: c.Host(), Path: p.PathWithNamespace, Name: p.NameWithNamespace, WebURL: p.WebURL}, nil
}

// SearchProjects finds projects the person is a member of by name, path, or
// group, most recently active first. It returns one page: a search narrows,
// and the person types more to find one further down.
func (c *Client) SearchProjects(ctx context.Context, query string) ([]forge.Project, error) {
	if c.token.Empty() {
		return nil, errors.New("gitlab: searching projects requires an authenticated account")
	}
	q := url.Values{"membership": {"true"}, "simple": {"true"}, "per_page": {"20"}, "order_by": {"last_activity_at"}}
	if query = strings.TrimSpace(query); query != "" {
		q.Set("search", query)
		q.Set("search_namespaces", "true")
	}
	b, _, err := c.do(ctx, "GET", "/api/v4/projects", q, nil)
	if err != nil {
		return nil, err
	}
	var rows []restProject
	if json.Unmarshal(b, &rows) != nil || rows == nil {
		return nil, errors.New("gitlab: invalid project search response")
	}
	out := make([]forge.Project, 0, len(rows))
	for _, row := range rows {
		p, err := c.project(row)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

// Project reads one project by its full path. The answer's path is GitLab's,
// which corrects the case of one typed or pasted.
func (c *Client) Project(ctx context.Context, path string) (forge.Project, error) {
	if c.token.Empty() {
		return forge.Project{}, errors.New("gitlab: project triage requires an authenticated account")
	}
	b, _, err := c.do(ctx, "GET", projectPath(path), nil, nil)
	if err != nil {
		return forge.Project{}, err
	}
	var row restProject
	if json.Unmarshal(b, &row) != nil {
		return forge.Project{}, errors.New("gitlab: invalid project response")
	}
	return c.project(row)
}

// ListOpen returns every open merge request in the project. Like Discover it
// reads the whole list, never a delta, so a change that closed leaves it.
func (c *Client) ListOpen(ctx context.Context, path string) ([]forge.Discovered, error) {
	if c.token.Empty() {
		return nil, errors.New("gitlab: project triage requires an authenticated account")
	}
	var out []forge.Discovered
	seen := map[int]bool{}
	for page := 1; ; page++ {
		if page > 1000 {
			return nil, errors.New("gitlab: project listing exceeds page limit")
		}
		q := url.Values{"state": {"opened"}, "per_page": {"100"}, "page": {strconv.Itoa(page)}, "order_by": {"created_at"}, "sort": {"asc"}}
		b, h, err := c.do(ctx, "GET", projectPath(path)+"/merge_requests", q, nil)
		if err != nil {
			return nil, err
		}
		var rows []struct {
			IID     int       `json:"iid"`
			WebURL  string    `json:"web_url"`
			State   string    `json:"state"`
			Updated time.Time `json:"updated_at"`
		}
		if json.Unmarshal(b, &rows) != nil || rows == nil {
			return nil, errors.New("gitlab: invalid project listing response")
		}
		for _, row := range rows {
			ref, err := ParseURL(row.WebURL)
			if err != nil || ref.Host != c.Host() || !strings.EqualFold(ref.Project, path) || ref.Number != row.IID || row.State != "opened" {
				return nil, errors.New("gitlab: invalid listed identity")
			}
			if seen[ref.Number] {
				// A change that moved between pages could be listed twice
				// and another skipped, so the list is not complete.
				return nil, errors.New("gitlab: project changed while listing; refresh again")
			}
			seen[ref.Number] = true
			out = append(out, forge.Discovered{Ref: ref, Updated: row.Updated})
		}
		if len(rows) == 0 {
			if h.Get("X-Next-Page") != "" {
				return nil, errors.New("gitlab: empty listing page has a successor")
			}
			return out, nil
		}
		if values, present := h["X-Next-Page"]; present {
			if len(values) != 1 {
				return nil, errors.New("gitlab: invalid listing pagination")
			}
			if values[0] == "" {
				return out, nil
			}
			next, err := strconv.Atoi(values[0])
			if err != nil || next != page+1 {
				return nil, errors.New("gitlab: invalid listing pagination")
			}
		}
	}
}

// dependenciesQuery reads a merge request's blocking merge requests, which
// GitLab offers on Premium and Ultimate and still marks experimental.
const dependenciesQuery = `query dependencies($fullPath: ID!, $iid: String!) {
  project(fullPath: $fullPath) {
    mergeRequest(iid: $iid) {
      iid
      blockingMergeRequests { hiddenCount visibleMergeRequests { iid state webUrl } }
    }
  }
}`

// Dependencies returns the open merge requests that must merge before ref.
// A GitLab without the field, or a project whose tier has none, answers
// forge.ErrUnsupported: unknown, never "none". A blocker the person cannot
// see still blocks, and comes back as a Ref with no project.
func (c *Client) Dependencies(ctx context.Context, ref forge.Ref) ([]forge.Ref, error) {
	if c.noDependencies.Load() {
		return nil, forge.ErrUnsupported
	}
	vars := map[string]any{"fullPath": ref.Project, "iid": strconv.Itoa(ref.Number)}
	b, _, err := c.do(ctx, "POST", "/api/graphql", nil, map[string]any{"query": dependenciesQuery, "variables": vars})
	if err != nil {
		return nil, err
	}
	var r struct {
		Data struct {
			Project *struct {
				MergeRequest *struct {
					IID      string `json:"iid"`
					Blocking *struct {
						HiddenCount int `json:"hiddenCount"`
						Visible     []struct {
							IID    string `json:"iid"`
							State  string `json:"state"`
							WebURL string `json:"webUrl"`
						} `json:"visibleMergeRequests"`
					} `json:"blockingMergeRequests"`
				} `json:"mergeRequest"`
			} `json:"project"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if json.Unmarshal(b, &r) != nil {
		return nil, errors.New("gitlab: invalid GraphQL response")
	}
	if len(r.Errors) > 0 {
		if strings.Contains(r.Errors[0].Message, "Field 'blockingMergeRequests' doesn't exist") {
			c.noDependencies.Store(true)
			return nil, forge.ErrUnsupported
		}
		return nil, fmt.Errorf("gitlab: graphql: %s", c.scrub(r.Errors[0].Message))
	}
	if r.Data.Project == nil || r.Data.Project.MergeRequest == nil {
		return nil, fmt.Errorf("%s: %w", ref, ErrNotFound)
	}
	mr := r.Data.Project.MergeRequest
	if mr.IID != strconv.Itoa(ref.Number) {
		return nil, errors.New("gitlab: mismatched merge request identity")
	}
	if mr.Blocking == nil {
		return nil, forge.ErrUnsupported
	}
	out := []forge.Ref{}
	for _, v := range mr.Blocking.Visible {
		if v.State == "merged" {
			continue
		}
		got, err := ParseURL(v.WebURL)
		if err != nil || got.Host != c.Host() || strconv.Itoa(got.Number) != v.IID {
			return nil, errors.New("gitlab: invalid blocking merge request identity")
		}
		out = append(out, got)
	}
	for range mr.Blocking.HiddenCount {
		out = append(out, forge.Ref{Host: c.Host()})
	}
	return out, nil
}

var _ forge.TriageReader = (*Client)(nil)

// codeOwnersPaths are where GitLab looks for CODEOWNERS, in its order.
var codeOwnersPaths = []string{"CODEOWNERS", "docs/CODEOWNERS", ".gitlab/CODEOWNERS"}

// maxCodeOwners bounds the CODEOWNERS text given to an agent.
const maxCodeOwners = 64 << 10

// CodeOwners reads the first CODEOWNERS GitLab would use at ref.
func (c *Client) CodeOwners(ctx context.Context, project, ref string) (string, error) {
	for _, p := range codeOwnersPaths {
		b, err := c.raw(ctx, forge.Ref{Host: c.Host(), Project: project}, p, ref)
		if errors.Is(err, forge.ErrNotFound) {
			continue
		}
		if err != nil {
			return "", err
		}
		if len(b) > maxCodeOwners {
			b = b[:maxCodeOwners]
		}
		return string(b), nil
	}
	return "", nil
}

// RecentAuthors reads up to thirty of the year's commits to path on ref.
func (c *Client) RecentAuthors(ctx context.Context, project, ref, path string) ([]string, error) {
	q := url.Values{"ref_name": {ref}, "path": {path}, "per_page": {"30"}, "since": {time.Now().AddDate(-1, 0, 0).UTC().Format(time.RFC3339)}}
	b, _, err := c.do(ctx, "GET", projectPath(project)+"/repository/commits", q, nil)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		AuthorName string `json:"author_name"`
	}
	if json.Unmarshal(b, &rows) != nil || rows == nil {
		return nil, errors.New("gitlab: invalid commits response")
	}
	out := []string{}
	for _, r := range rows {
		if r.AuthorName != "" && !slices.Contains(out, r.AuthorName) {
			out = append(out, r.AuthorName)
		}
	}
	return out, nil
}
