package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
)

// diffPages is a GitLab whose merge request has 230 diff versions, served a
// hundred at a time. With moved set, the merge request's updatedAt changes
// between pages.
func diffPages(t *testing.T, moved bool) (*httptest.Server, *int) {
	t.Helper()
	const total = 230
	stamp := func(i int) string { return time.Date(2026, 1, 1, 0, 0, i, 0, time.UTC).Format(time.RFC3339) }
	page := func(after string) (nodes []map[string]any, info map[string]any) {
		start := 0
		fmt.Sscan(after, &start)
		end := min(start+100, total)
		// GitLab lists the newest version first.
		for i := total - 1 - start; i >= total-end; i-- {
			nodes = append(nodes, map[string]any{"createdAt": stamp(i)})
		}
		return nodes, map[string]any{"hasNextPage": end < total, "endCursor": fmt.Sprint(end)}
	}
	diffQueries := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		updated := "2026-02-01T00:00:00Z"
		if strings.Contains(req.Query, "query mergeRequestDiffs") {
			diffQueries++
			if moved {
				updated = "2026-02-02T00:00:00Z"
			}
			nodes, info := page(req.Variables["after"].(string))
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"project": map[string]any{"mergeRequest": map[string]any{
				"iid": "5", "updatedAt": updated, "mergeRequestDiffs": map[string]any{"pageInfo": info, "nodes": nodes}}}}})
			return
		}
		nodes, info := page("")
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"project": map[string]any{"mergeRequest": map[string]any{
			"iid": "5", "title": "Pushed often", "state": "opened", "updatedAt": updated, "createdAt": "2026-01-01T00:00:00Z",
			"author":            map[string]any{"username": "me"},
			"diffRefs":          map[string]any{"baseSha": "b", "headSha": "h", "startSha": "s"},
			"mergeRequestDiffs": map[string]any{"pageInfo": info, "nodes": nodes},
			"discussions":       map[string]any{"pageInfo": map[string]any{"hasNextPage": false}, "nodes": []any{}},
		}}}})
	}))
	t.Cleanup(srv.Close)
	return srv, &diffQueries
}

func pagedClient(t *testing.T, srv *httptest.Server) (*Client, forge.Ref) {
	t.Helper()
	c, err := New(srv.URL, config.Secret{})
	if err != nil {
		t.Fatal(err)
	}
	return c, forge.Ref{Host: c.Host(), Project: "group/project", Number: 5}
}

func TestEveryDiffVersionIsReadPastTheFirstPage(t *testing.T) {
	srv, queries := diffPages(t, false)
	c, ref := pagedClient(t, srv)
	ch, err := c.ReadFacts(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	if *queries != 2 {
		t.Errorf("%d diff version queries, want 2", *queries)
	}
	if len(ch.Pushes) != 230 {
		t.Fatalf("%d pushes, want 230", len(ch.Pushes))
	}
	for i := 1; i < len(ch.Pushes); i++ {
		if !ch.Pushes[i-1].At.Before(ch.Pushes[i].At) {
			t.Fatalf("pushes out of order at %d", i)
		}
	}
	if first := ch.Pushes[0].At; !first.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("the oldest push is %s", first)
	}
	if last := ch.Pushes[len(ch.Pushes)-1]; last.Head != "h" {
		t.Errorf("the newest push names head %q", last.Head)
	}
}

func TestAMergeRequestThatMovesBetweenDiffPagesIsRefused(t *testing.T) {
	srv, _ := diffPages(t, true)
	c, ref := pagedClient(t, srv)
	if _, err := c.ReadFacts(context.Background(), ref); err == nil || !strings.Contains(err.Error(), "changed while reading pages") {
		t.Fatalf("err %v", err)
	}
}
