package forgejo

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/terva-sh/vuoro/forge"
)

var _ forge.Discoverer = (*Client)(nil)
var (
	_ forge.FactsReader = (*Client)(nil)
	_ forge.Account     = (*Client)(nil)
)

type apiIssue struct {
	Number      int       `json:"number"`
	HTMLURL     string    `json:"html_url"`
	State       string    `json:"state"`
	PullRequest *struct{} `json:"pull_request"`
	Repository  struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
	Updated time.Time `json:"updated_at"`
}

// Discover returns complete open membership for this authenticated person.
// All search filters are scoped to the signed-in user by Forgejo.
func (c *Client) Discover(ctx context.Context, username string) ([]forge.Discovered, error) {
	if c.token.Empty() || username == "" {
		return nil, errors.New("forgejo: inbox requires an authenticated account")
	}
	person, err := read[apiUser](ctx, c, "/user")
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(person.Login, username) {
		return nil, errors.New("forgejo: signed-in account does not match configured username")
	}
	queries := []struct {
		filter string
		role   forge.Role
	}{
		{"created", forge.AuthorRole}, {"review_requested", forge.ReviewerRole}, {"assigned", forge.AssigneeRole}, {"reviewed", forge.ReviewerRole},
	}
	members := map[string]forge.Discovered{}
	for _, query := range queries {
		for page := 1; ; page++ {
			if page > 1000 {
				return nil, errors.New("forgejo: discovery exceeds page limit")
			}
			q := url.Values{"state": {"open"}, "type": {"pulls"}, query.filter: {"true"}, "limit": {"100"}, "page": {strconv.Itoa(page)}}
			b, err := c.get(ctx, "/repos/issues/search", q)
			if err != nil {
				return nil, err
			}
			var rows []apiIssue
			if json.Unmarshal(b, &rows) != nil || rows == nil {
				return nil, errors.New("forgejo: invalid discovery response")
			}
			if len(rows) == 0 {
				break
			}
			for _, row := range rows {
				ref, err := ParseURL(row.HTMLURL)
				if err != nil || ref.Host != c.base.Host || ref.Number != row.Number || ref.Project != row.Repository.FullName || row.PullRequest == nil || row.State != "open" {
					return nil, errors.New("forgejo: invalid discovered identity")
				}
				key := ref.String()
				m, exists := members[key]
				if query.filter == "reviewed" && !slices.Contains(m.Roles, forge.ReviewerRole) {
					ch, err := c.ReadFacts(ctx, ref)
					if err != nil {
						return nil, err
					}
					if ch.State != forge.Open || !slices.ContainsFunc(ch.Reviewers, func(r forge.Reviewer) bool { return strings.EqualFold(r.Username, username) }) {
						continue
					}
				}
				if !exists {
					m = forge.Discovered{Ref: ref, Updated: row.Updated}
				}
				if exists && !m.Updated.Equal(row.Updated) {
					m.Updated = time.Time{}
				}
				if !slices.Contains(m.Roles, query.role) {
					m.Roles = append(m.Roles, query.role)
				}
				members[key] = m
			}
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
