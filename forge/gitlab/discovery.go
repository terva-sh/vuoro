package gitlab

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

// Discover returns complete open membership for the configured signed-in person.
// It never follows pagination URLs or uses a delta as a membership snapshot.
func (c *Client) Discover(ctx context.Context, username string) ([]forge.Discovered, error) {
	if c.token.Empty() || username == "" {
		return nil, errors.New("gitlab: inbox requires an authenticated account")
	}
	b, _, err := c.do(ctx, "GET", "/api/v4/user", nil, nil)
	if err != nil {
		return nil, err
	}
	var person struct {
		Username string `json:"username"`
	}
	if json.Unmarshal(b, &person) != nil || !strings.EqualFold(person.Username, username) {
		return nil, errors.New("gitlab: signed-in account does not match configured username")
	}
	queries := []struct {
		filter string
		role   forge.Role
	}{
		{"author_username", forge.AuthorRole},
		{"reviewer_username", forge.ReviewerRole},
		{"assignee_username[]", forge.AssigneeRole},
	}
	members := map[string]forge.Discovered{}
	for _, query := range queries {
		for page := 1; ; page++ {
			if page > 1000 {
				return nil, errors.New("gitlab: discovery exceeds page limit")
			}
			q := url.Values{"scope": {"all"}, "state": {"opened"}, "per_page": {"100"}, "page": {strconv.Itoa(page)}, query.filter: {username}, "order_by": {"created_at"}, "sort": {"asc"}}
			b, h, err := c.do(ctx, "GET", "/api/v4/merge_requests", q, nil)
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
				return nil, errors.New("gitlab: invalid discovery response")
			}
			for _, row := range rows {
				ref, err := ParseURL(row.WebURL)
				if err != nil || ref.Host != c.Host() || ref.Number != row.IID || row.State != "opened" {
					return nil, errors.New("gitlab: invalid discovered identity")
				}
				key := ref.String()
				m, exists := members[key]
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
			// GitLab may omit paging headers on large searches. Continue until
			// an empty page in that case; a server-clamped page size is not EOF.
			if len(rows) == 0 {
				if h.Get("X-Next-Page") != "" {
					return nil, errors.New("gitlab: empty discovery page has a successor")
				}
				break
			}
			if values, present := h["X-Next-Page"]; present {
				if len(values) != 1 {
					return nil, errors.New("gitlab: invalid discovery pagination")
				}
				if values[0] == "" {
					break
				}
				next, err := strconv.Atoi(values[0])
				if err != nil || next != page+1 {
					return nil, errors.New("gitlab: invalid discovery pagination")
				}
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
