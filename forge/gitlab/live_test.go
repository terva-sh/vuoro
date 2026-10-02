package gitlab

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
)

// TestLive opens a public merge request on gitlab.com anonymously. It is the
// one test here that reaches the network, so it runs only when asked:
//
//	VUORO_LIVE_GITLAB=1 go test -run TestLive ./forge/gitlab
//
// VUORO_LIVE_GITLAB_MR names another merge request URL.
func TestLive(t *testing.T) {
	if os.Getenv("VUORO_LIVE_GITLAB") != "1" {
		t.Skip("set VUORO_LIVE_GITLAB=1 to open a merge request on gitlab.com")
	}
	raw := os.Getenv("VUORO_LIVE_GITLAB_MR")
	if raw == "" {
		raw = "https://gitlab.com/gitlab-org/cli/-/merge_requests/3991"
	}
	ref, err := forge.ParseGitLabURL(raw)
	if err != nil {
		t.Fatal(err)
	}
	c, err := New("https://"+ref.Host, config.Secret{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	ch, err := c.Open(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if ch.Diff.Head == "" || len(ch.Files) == 0 || len(ch.Threads) == 0 {
		t.Fatalf("head %q, %d files, %d threads", ch.Diff.Head, len(ch.Files), len(ch.Threads))
	}
	for _, f := range ch.Files {
		if f.Patch == "" && !f.Omitted && !f.Renamed && !f.New && !f.Deleted {
			t.Errorf("%s has no patch and is not marked omitted", f.NewPath)
		}
	}
	t.Logf("%s: %q, %d files, %d threads, %d reviewers, %d pushes", ref, ch.Title, len(ch.Files), len(ch.Threads), len(ch.Reviewers), len(ch.Pushes))
}
