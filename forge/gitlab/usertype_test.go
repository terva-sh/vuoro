package gitlab

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
)

// userTypes is a GitLab whose merge request has a thread by GitLab's ghost
// user and a reply by a person. With old set, it refuses UserCore.type as a
// GitLab before that field does.
func userTypes(t *testing.T, old bool) (*httptest.Server, *[]string) {
	t.Helper()
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query string `json:"query"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		queries = append(queries, req.Query)
		withType := strings.Contains(req.Query, "username bot type")
		if old && withType {
			w.Write([]byte(`{"errors":[{"message":"Field 'type' doesn't exist on type 'UserCore'"}]}`))
			return
		}
		u := func(name, kind string) map[string]any {
			m := map[string]any{"username": name, "bot": false}
			if withType {
				m["type"] = kind
			}
			return m
		}
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"project": map[string]any{"mergeRequest": map[string]any{
			"iid": "5", "title": "Old change", "state": "opened", "updatedAt": "2026-02-01T00:00:00Z", "createdAt": "2026-01-01T00:00:00Z",
			"author": u("me", "HUMAN"), "diffRefs": map[string]any{"baseSha": "b", "headSha": "h", "startSha": "s"},
			"mergeRequestDiffs": map[string]any{"pageInfo": map[string]any{}, "nodes": []any{}},
			"participants":      map[string]any{"pageInfo": map[string]any{}, "nodes": []any{u("ghost1", "GHOST"), u("helper", "PROJECT_BOT"), u("alice", "HUMAN")}},
			"discussions": map[string]any{"pageInfo": map[string]any{"hasNextPage": false}, "nodes": []any{map[string]any{
				"id": "d1", "resolvable": true, "resolved": false,
				"notes": map[string]any{"pageInfo": map[string]any{}, "nodes": []any{map[string]any{"id": "n1", "author": u("ghost1", "GHOST"), "createdAt": "2026-01-02T00:00:00Z", "body": "?"}}},
			}}},
		}}}})
	}))
	t.Cleanup(srv.Close)
	return srv, &queries
}

func TestGitLabsGhostUserIsNotAPerson(t *testing.T) {
	srv, _ := userTypes(t, false)
	c, _ := New(srv.URL, config.Secret{})
	ch, err := c.ReadFacts(context.Background(), forge.Ref{Host: c.Host(), Project: "group/project", Number: 5})
	if err != nil {
		t.Fatal(err)
	}
	if a := ch.Threads[0].Notes[0].Author; a.Username != "ghost1" || !a.Bot {
		t.Fatalf("ghost note author %+v", a)
	}
	bots := map[string]bool{}
	for _, p := range ch.Participants {
		bots[p.Username] = p.Bot
	}
	if !bots["ghost1"] || !bots["helper"] || bots["alice"] {
		t.Fatalf("participants %+v", ch.Participants)
	}
}

func TestAGitLabWithoutUserTypeStillReads(t *testing.T) {
	srv, queries := userTypes(t, true)
	c, _ := New(srv.URL, config.Secret{})
	ref := forge.Ref{Host: c.Host(), Project: "group/project", Number: 5}
	ch, err := c.ReadFacts(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	if ch.Title != "Old change" || ch.Threads[0].Notes[0].Author.Bot {
		t.Fatalf("read %+v", ch.Threads[0].Notes[0].Author)
	}
	if _, err := c.ReadFacts(context.Background(), ref); err != nil {
		t.Fatal(err)
	}
	// One refusal, then every query without the field.
	if len(*queries) != 3 || !strings.Contains((*queries)[0], "username bot type") || strings.Contains((*queries)[1], "username bot type") || strings.Contains((*queries)[2], "username bot type") {
		t.Fatalf("%d queries", len(*queries))
	}
}
