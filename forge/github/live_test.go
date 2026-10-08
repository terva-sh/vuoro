package github

import (
	"context"
	"errors"
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

// TestLive opens and checks out a real pull request without posting or
// starting an agent. The token is read from VUORO_LIVE_GITHUB_TOKEN, as a
// configured token_env is, and never printed.
//
// VUORO_LIVE_GITHUB=1 VUORO_LIVE_GITHUB_PR=https://github.com/OWNER/REPO/pull/N
// VUORO_LIVE_GITHUB_TOKEN=... go test -run TestLive -v ./forge/github
func TestLive(t *testing.T) {
	if os.Getenv("VUORO_LIVE_GITHUB") != "1" {
		t.Skip("set VUORO_LIVE_GITHUB=1, VUORO_LIVE_GITHUB_PR and VUORO_LIVE_GITHUB_TOKEN to open and fetch a pull request")
	}
	ref, err := ParseURL(os.Getenv("VUORO_LIVE_GITHUB_PR"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	token, err := config.Forge{TokenEnv: "VUORO_LIVE_GITHUB_TOKEN"}.Token(ctx)
	if err != nil {
		t.Fatal(err)
	}
	c, err := New("https://"+ref.Host, token)
	if err != nil {
		t.Fatal(err)
	}
	inner := http.DefaultTransport
	c.http.Transport = roundTrip(func(req *http.Request) (*http.Response, error) {
		if req.Method != "GET" && !(req.Method == "POST" && req.URL.Path == "/graphql") {
			return nil, errors.New("live check refuses writes")
		}
		return inner.RoundTrip(req)
	})
	ch, err := c.Open(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if ch.Kind != forge.GitHub || ch.Diff.Head == "" || ch.Diff.Base == "" || len(ch.Files) == 0 {
		t.Fatal("live pull request is missing review metadata")
	}
	facts, err := c.ReadFacts(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if facts.Diff.Head != ch.Diff.Head || len(facts.Files) != 0 || len(facts.Reviews) != len(ch.Reviews) {
		t.Fatal("facts disagree with the opened change")
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
	run("--git-dir="+repo, "fetch", "--quiet", "--filter=blob:none", "--no-tags", ch.Checkout.HTTPS, "+"+ch.Checkout.HeadRef+":refs/vuoro/head", "+refs/heads/"+ch.TargetBranch+":refs/vuoro/target")
	run("--git-dir="+repo, "cat-file", "-e", ch.Diff.Base+"^{commit}")
	run("--git-dir="+repo, "worktree", "add", "--quiet", "--detach", worktree, ch.Diff.Head)
	if head := run("-C", worktree, "rev-parse", "HEAD"); head != ch.Diff.Head {
		t.Fatal("worktree is not the reviewed head")
	}
	resolved := 0
	for _, th := range ch.Threads {
		if th.Resolved {
			resolved++
		}
	}
	t.Logf("%s: %d files, %d threads (%d resolved), %d reviews, %d reviewers, %d requests, %d pushes, mergeability %s",
		ref.Display(forge.GitHub), len(ch.Files), len(ch.Threads), resolved, len(ch.Reviews), len(ch.Reviewers), len(ch.Requests), len(ch.Pushes), ch.Mergeability)
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
