package github

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/terva-sh/vuoro/forge"
)

var (
	_ forge.Discoverer  = (*Client)(nil)
	_ forge.FactsReader = (*Client)(nil)
	_ forge.Account     = (*Client)(nil)
)

const searchQuery = `query($q: String!, $after: String) { search(query: $q, type: ISSUE, first: 100, after: $after) {
	issueCount pageInfo { hasNextPage endCursor }
	nodes { ... on PullRequest { number url state updatedAt repository { nameWithOwner } } } } }`

// maxResults is as many results as GitHub's search returns for one query.
const maxResults = 1000

// Discover returns complete open membership for the signed-in person. A
// search GitHub would cut short is refused rather than read in part.
func (c *Client) Discover(ctx context.Context, username string) ([]forge.Discovered, error) {
	if c.token.Empty() || username == "" {
		return nil, errors.New("github: inbox requires an authenticated account")
	}
	me, err := c.Me(ctx)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(me, username) {
		return nil, errors.New("github: signed-in account does not match configured username")
	}
	// reviewed-by keeps a pull request the person has reviewed, whose request
	// GitHub dropped when they answered it.
	queries := []struct {
		qualifier string
		role      forge.Role
	}{
		{"author", forge.AuthorRole}, {"review-requested", forge.ReviewerRole}, {"reviewed-by", forge.ReviewerRole}, {"assignee", forge.AssigneeRole},
	}
	members := map[string]forge.Discovered{}
	for _, query := range queries {
		q := fmt.Sprintf("is:pr is:open archived:false %s:%s", query.qualifier, me)
		var after any
		for {
			var out struct {
				Search struct {
					IssueCount int      `json:"issueCount"`
					PageInfo   pageInfo `json:"pageInfo"`
					Nodes      []struct {
						Number     int       `json:"number"`
						URL        string    `json:"url"`
						State      string    `json:"state"`
						UpdatedAt  time.Time `json:"updatedAt"`
						Repository struct {
							NameWithOwner string `json:"nameWithOwner"`
						} `json:"repository"`
					} `json:"nodes"`
				} `json:"search"`
			}
			if err := c.query(ctx, searchQuery, map[string]any{"q": q, "after": after}, &out); err != nil {
				return nil, err
			}
			if out.Search.IssueCount > maxResults {
				return nil, fmt.Errorf("github: %s:%s finds more pull requests than GitHub's search returns", query.qualifier, me)
			}
			for _, row := range out.Search.Nodes {
				ref, err := ParseURL(row.URL)
				if err != nil || ref.Host != c.web.Host || ref.Number != row.Number || !strings.EqualFold(ref.Project, row.Repository.NameWithOwner) || row.State != "OPEN" {
					return nil, errors.New("github: invalid discovered identity")
				}
				key := ref.String()
				m, exists := members[key]
				if !exists {
					m = forge.Discovered{Ref: ref, Updated: row.UpdatedAt}
				}
				if exists && !m.Updated.Equal(row.UpdatedAt) {
					m.Updated = time.Time{}
				}
				if !slices.Contains(m.Roles, query.role) {
					m.Roles = append(m.Roles, query.role)
				}
				members[key] = m
			}
			if !out.Search.PageInfo.HasNextPage {
				break
			}
			after = out.Search.PageInfo.EndCursor
		}
	}
	keys := make([]string, 0, len(members))
	for key := range members {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	out := make([]forge.Discovered, 0, len(keys))
	for _, key := range keys {
		out = append(out, members[key])
	}
	return out, nil
}
