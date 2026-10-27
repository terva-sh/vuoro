package forgejo

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
)

// Forgejo merges or rebases by its style parameter, once the head is still
// the one the person saw.
func TestForgejoUpdatesABranchByItsStyle(t *testing.T) {
	var styles []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/v1/repos/terva-sh/vuoro/pulls/2":
			w.Write([]byte(`{"head": {"ref": "fix/total", "sha": "abc123", "repo": {"full_name": "terva-sh/vuoro"}}, "base": {"ref": "main", "repo": {"full_name": "terva-sh/vuoro"}}}`))
		case r.Method == "POST" && r.URL.Path == "/api/v1/repos/terva-sh/vuoro/pulls/2/update":
			if r.Header.Get("Authorization") != "token synthetic-forgejo-token" {
				http.Error(w, `{"message":"token is required"}`, 401)
				return
			}
			styles = append(styles, r.URL.Query().Get("style"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c, err := New(srv.URL, config.SecretForTest("synthetic-forgejo-token"))
	if err != nil {
		t.Fatal(err)
	}
	ref := forge.Ref{Host: c.base.Host, Project: "terva-sh/vuoro", Number: 2}
	ctx := context.Background()
	for _, s := range []string{forge.UpdateMerge, forge.UpdateRebase} {
		if err := c.UpdateBranch(ctx, ref, s, "abc123"); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
	if err := c.UpdateBranch(ctx, ref, forge.UpdateMerge, "def456"); !errors.Is(err, forge.ErrHeadMoved) {
		t.Fatalf("a moved head was updated: %v", err)
	}
	if err := c.UpdateBranch(ctx, ref, "squash", "abc123"); !errors.Is(err, forge.ErrUnsupported) {
		t.Fatalf("an unknown strategy: %v", err)
	}
	if len(styles) != 2 || styles[0] != "merge" || styles[1] != "rebase" {
		t.Fatalf("styles %q", styles)
	}
}
