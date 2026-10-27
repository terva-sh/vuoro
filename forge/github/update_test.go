package github

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
)

// GitHub merges through REST and rebases through GraphQL, each against the
// head the person saw.
func TestGitHubUpdatesABranchAgainstTheExpectedHead(t *testing.T) {
	var merged []map[string]any
	var mutation map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var req map[string]any
		json.Unmarshal(b, &req)
		switch {
		case r.URL.Path == "/api/graphql" && strings.HasPrefix(req["query"].(string), "mutation"):
			mutation = req["variables"].(map[string]any)
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"updatePullRequestBranch": map[string]any{"pullRequest": map[string]any{"headRefOid": "f00d"}}}})
		case r.URL.Path == "/api/graphql":
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"repository": map[string]any{"pullRequest": map[string]any{"id": "PR_1", "headRefOid": head}}}})
		case r.Method == "PUT" && r.URL.Path == "/api/v3/repos/owner/repo/pulls/1/update-branch":
			merged = append(merged, req)
			if req["expected_head_sha"] != head {
				w.WriteHeader(422)
				w.Write([]byte(`{"message":"expected head sha didn't match current head ref."}`))
				return
			}
			w.WriteHeader(202)
			w.Write([]byte(`{"message":"Updating pull request branch."}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	c, err := New(srv.URL, config.SecretForTest("sentinel-token"))
	if err != nil {
		t.Fatal(err)
	}
	ref := forge.Ref{Host: strings.TrimPrefix(srv.URL, "http://"), Project: "owner/repo", Number: 1}
	ctx := context.Background()
	if err := c.UpdateBranch(ctx, ref, forge.UpdateMerge, head); err != nil || len(merged) != 1 {
		t.Fatalf("merge %v %v", err, merged)
	}
	if err := c.UpdateBranch(ctx, ref, forge.UpdateMerge, "0000"); !errors.Is(err, forge.ErrHeadMoved) {
		t.Fatalf("a moved head was merged: %v", err)
	}
	if err := c.UpdateBranch(ctx, ref, forge.UpdateRebase, "0000"); !errors.Is(err, forge.ErrHeadMoved) || mutation != nil {
		t.Fatalf("a moved head was rebased: %v", err)
	}
	if err := c.UpdateBranch(ctx, ref, forge.UpdateRebase, head); err != nil || mutation["id"] != "PR_1" || mutation["head"] != head {
		t.Fatalf("rebase %v %v", err, mutation)
	}
	if err := c.UpdateBranch(ctx, ref, "squash", head); !errors.Is(err, forge.ErrUnsupported) {
		t.Fatalf("an unknown strategy: %v", err)
	}
}
