package forgejo

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
)

// TestLive opens and checks out a real PR without posting or starting an agent.
// tea owns authentication; this test never reads its auth file or token.
//
// VUORO_LIVE_FORGEJO=1 VUORO_LIVE_FORGEJO_PR=https://HOST/OWNER/REPO/pulls/N
// go test -run TestLive -v ./forge/forgejo
func TestLive(t *testing.T) {
	if os.Getenv("VUORO_LIVE_FORGEJO") != "1" {
		t.Skip("set VUORO_LIVE_FORGEJO=1 and VUORO_LIVE_FORGEJO_PR to open and fetch a PR with tea and git")
	}
	ref, err := ParseURL(os.Getenv("VUORO_LIVE_FORGEJO_PR"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := New("https://"+ref.Host, config.Secret{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	c.http.Transport = roundTrip(func(req *http.Request) (*http.Response, error) {
		if req.Method != "GET" {
			return nil, errors.New("live check refuses writes")
		}
		cmd := exec.CommandContext(req.Context(), "tea", "api", req.URL.String())
		cmd.Stderr = io.Discard
		b, err := cmd.Output()
		if err != nil {
			return nil, errors.New("tea could not read the live endpoint")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(b)))}, nil
	})
	ch, err := c.Open(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if ch.Kind != forge.Forgejo || ch.Diff.Head == "" || ch.Diff.Base == "" || len(ch.Files) == 0 {
		t.Fatal("live PR is missing review metadata")
	}
	if !forge.ValidBranch(ch.TargetBranch) {
		t.Fatal("unsafe target branch")
	}
	root := t.TempDir()
	repo := filepath.Join(root, "repo.git")
	worktree := filepath.Join(root, "worktree")
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.CommandContext(ctx, "git", args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git live check failed: %v\n%s", err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "--bare", "--quiet", repo)
	remote := ch.Checkout.SSH
	if remote == "" {
		remote = ch.Checkout.HTTPS
	}
	run("--git-dir="+repo, "fetch", "--quiet", "--filter=blob:none", "--no-tags", remote, "+"+ch.Checkout.HeadRef+":refs/vuoro/head", "+refs/heads/"+ch.TargetBranch+":refs/vuoro/target")
	run("--git-dir="+repo, "cat-file", "-e", ch.Diff.Base+"^{commit}")
	run("--git-dir="+repo, "worktree", "add", "--quiet", "--detach", worktree, ch.Diff.Head)
	if head := run("-C", worktree, "rev-parse", "HEAD"); head != ch.Diff.Head {
		t.Fatal("worktree is not the reviewed head")
	}
	shown := 0
	for _, f := range ch.Files {
		if !f.Omitted && f.Patch != "" {
			shown++
		}
	}
	if shown == 0 {
		t.Fatal("all live patches were omitted")
	}
	t.Logf("%s: %d files (%d patches), %d threads, %d pushes; head %.12s checked out over %s", ref.Display(forge.Forgejo), len(ch.Files), shown, len(ch.Threads), len(ch.Pushes), ch.Diff.Head, remote)
}
