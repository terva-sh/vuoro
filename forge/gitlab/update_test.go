package gitlab

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/terva-sh/vuoro/forge"
)

// GitLab rebases a merge request in the background, so the rebase is read
// until it finishes, and its failure is the person's to read. A merge, which
// GitLab does not do, falls back to vuoro's push.
func TestGitLabRebasesThroughTheForgeAgainstTheHead(t *testing.T) {
	defer func(d time.Duration) { rebasePoll = d }(rebasePoll)
	rebasePoll = time.Millisecond
	var rebased, reads int
	mergeError := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.EscapedPath()
		switch {
		case r.Method == "GET" && path == "/api/v4/projects/gitlab-org%2Fcli/merge_requests/3991":
			if r.URL.Query().Get("include_rebase_in_progress") != "true" {
				t.Error("the rebase's progress was not asked for")
			}
			reads++
			inProgress := rebased > 0 && reads < 4
			w.Write([]byte(`{"sha": "abc123", "rebase_in_progress": ` + map[bool]string{true: "true", false: "false"}[inProgress] + `, "merge_error": "` + mergeError + `"}`))
		case r.Method == "PUT" && path == "/api/v4/projects/gitlab-org%2Fcli/merge_requests/3991/rebase":
			if r.Header.Get("Authorization") != "Bearer "+secret {
				http.Error(w, `{"message":"401 Unauthorized"}`, 401)
				return
			}
			rebased++
			w.WriteHeader(202)
			w.Write([]byte(`{"rebase_in_progress": true}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c, ref := client(t, srv, secret)
	ctx := context.Background()
	if err := c.UpdateBranch(ctx, ref, forge.UpdateMerge, "abc123"); !errors.Is(err, forge.ErrUnsupported) || rebased != 0 {
		t.Fatalf("a merge: %v", err)
	}
	if err := c.UpdateBranch(ctx, ref, forge.UpdateRebase, "def456"); !errors.Is(err, forge.ErrHeadMoved) || rebased != 0 {
		t.Fatalf("a moved head was rebased: %v", err)
	}
	if err := c.UpdateBranch(ctx, ref, forge.UpdateRebase, "abc123"); err != nil || rebased != 1 || reads < 4 {
		t.Fatalf("rebase %v, rebased %d, read %d times", err, rebased, reads)
	}
	reads, mergeError = 0, "Rebase failed: conflict"
	if err := c.UpdateBranch(ctx, ref, forge.UpdateRebase, "abc123"); err == nil {
		t.Fatal("a failed rebase was reported done")
	}
}
