package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/terva-sh/vuoro/forge"
)

func TestAStackedMergeRequestGoesOntoTheSourceBranchAsADraft(t *testing.T) {
	sourceProject := 7
	var opened map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.EscapedPath() == "/api/v4/projects/gitlab-org%2Fcli/merge_requests/3991":
			json.NewEncoder(w).Encode(map[string]any{"source_project_id": sourceProject, "target_project_id": 7, "source_branch": "fix/total", "sha": "abc123"})
		case r.Method == "POST" && r.URL.EscapedPath() == "/api/v4/projects/gitlab-org%2Fcli/merge_requests":
			if r.Header.Get("Authorization") != "Bearer "+secret {
				http.Error(w, `{"message":"401 Unauthorized"}`, 401)
				return
			}
			b, _ := io.ReadAll(r.Body)
			json.Unmarshal(b, &opened)
			w.Write([]byte(`{"iid": 4000, "web_url": "https://gitlab.example/gitlab-org/cli/-/merge_requests/4000"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c, ref := client(t, srv, secret)
	ctx := context.Background()

	got, err := c.StackTarget(ctx, ref)
	if err != nil || got != (forge.StackTarget{Branch: "fix/total", Head: "abc123"}) {
		t.Fatalf("target %+v %v", got, err)
	}
	url, err := c.OpenStacked(ctx, ref, forge.NewStacked{Branch: "vuoro/fix-3991-abc", Target: "fix/total", Title: "Fix review findings", Description: "d"})
	if err != nil || url != "https://gitlab.example/gitlab-org/cli/-/merge_requests/4000" {
		t.Fatalf("opened %q %v", url, err)
	}
	if opened["title"] != "Draft: Fix review findings" || opened["source_branch"] != "vuoro/fix-3991-abc" || opened["target_branch"] != "fix/total" || opened["remove_source_branch"] != true {
		t.Fatalf("sent %v", opened)
	}

	sourceProject = 8
	if _, err := c.StackTarget(ctx, ref); !errors.Is(err, forge.ErrFork) {
		t.Fatalf("a fork: %v", err)
	}
}
