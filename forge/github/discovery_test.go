package github

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/terva-sh/vuoro/forge"
)

type hit struct {
	n       int
	updated string
}

// searching answers the viewer query as login, and each search with the
// pull requests found for its qualifier, a page per slice.
func searching(s *server, login string, found map[string][][]hit, count int) {
	s.answer = func(q string, vars map[string]any) (any, bool) {
		if strings.Contains(q, "viewer") {
			return map[string]any{"data": map[string]any{"viewer": map[string]any{"login": login}}}, true
		}
		if !strings.Contains(q, "search(") {
			return nil, false
		}
		query := vars["q"].(string)
		qualifier := strings.Fields(query)[3]
		name, _, _ := strings.Cut(qualifier, ":")
		pages := found[name]
		page := 0
		if after, ok := vars["after"].(string); ok {
			page = int(after[0] - '0')
		}
		var rows []any
		if page < len(pages) {
			for _, h := range pages[page] {
				rows = append(rows, map[string]any{"number": h.n, "url": "https://" + s.host + "/owner/repo/pull/" + itoa(h.n), "state": "OPEN", "updatedAt": h.updated, "repository": map[string]any{"nameWithOwner": "owner/repo"}})
			}
		}
		more := page+1 < len(pages)
		n := count
		if n == 0 {
			n = len(rows)
		}
		return map[string]any{"data": map[string]any{"search": map[string]any{"issueCount": n, "pageInfo": map[string]any{"hasNextPage": more, "endCursor": string(rune('0' + page + 1))}, "nodes": rows}}}, true
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

func TestDiscoverMergesRolesAcrossSearches(t *testing.T) {
	c, s, _ := serve(t, "13899")
	searching(s, "me", map[string][][]hit{
		"author":           {{{11, "2026-10-01T00:00:00Z"}}},
		"review-requested": {{{12, "2026-10-02T00:00:00Z"}}, {{13, "2026-10-03T00:00:00Z"}}},
		"reviewed-by":      {{{13, "2026-10-03T00:00:00Z"}, {14, "2026-10-04T00:00:00Z"}}},
		"assignee":         {{{11, "2026-10-01T00:00:00Z"}}},
	}, 0)
	got, err := c.Discover(context.Background(), "Me")
	if err != nil {
		t.Fatal(err)
	}
	want := map[int]string{11: "author assignee", 12: "reviewer", 13: "reviewer", 14: "reviewer"}
	if len(got) != len(want) {
		t.Fatalf("%d discovered, want %d: %+v", len(got), len(want), got)
	}
	for _, d := range got {
		var roles []string
		for _, r := range d.Roles {
			roles = append(roles, string(r))
		}
		if strings.Join(roles, " ") != want[d.Ref.Number] || d.Ref.Host != s.host || d.Ref.Project != "owner/repo" || d.Updated.IsZero() {
			t.Errorf("#%d: %v %+v", d.Ref.Number, roles, d)
		}
	}
	for _, q := range s.graphql {
		if strings.Contains(q, "search(") && !strings.Contains(q, "type: ISSUE") {
			t.Errorf("search %s", q)
		}
	}
}

func TestDiscoverRefusesAnotherAccountAndACutShortSearch(t *testing.T) {
	c, s, _ := serve(t, "13899")
	searching(s, "someone-else", nil, 0)
	if _, err := c.Discover(context.Background(), "me"); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("another account: %v", err)
	}
	c, s, _ = serve(t, "13899")
	searching(s, "me", map[string][][]hit{"author": {{{11, "2026-10-01T00:00:00Z"}}}}, maxResults+1)
	if _, err := c.Discover(context.Background(), "me"); err == nil || !strings.Contains(err.Error(), "more pull requests") {
		t.Fatalf("a search GitHub would cut short: %v", err)
	}
}

func TestDiscoverRefusesAPullRequestFromAnotherHost(t *testing.T) {
	c, s, _ := serve(t, "13899")
	searching(s, "me", nil, 0)
	inner := s.answer
	s.answer = func(q string, vars map[string]any) (any, bool) {
		if strings.Contains(q, "search(") {
			return map[string]any{"data": map[string]any{"search": map[string]any{"issueCount": 1, "pageInfo": map[string]any{}, "nodes": []any{
				map[string]any{"number": 1, "url": "https://elsewhere.example/owner/repo/pull/1", "state": "OPEN", "updatedAt": "2026-10-01T00:00:00Z", "repository": map[string]any{"nameWithOwner": "owner/repo"}}}}}}, true
		}
		return inner(q, vars)
	}
	if _, err := c.Discover(context.Background(), "me"); err == nil {
		t.Fatal("a pull request on another host was accepted")
	}
	var _ forge.Discoverer = c
}
