package github

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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

// TestLiveStack checks both of the editor's routes on a sandbox pull request:
// a push to the pull request's own head branch, and a draft pull request
// stacked onto it. git pushes with the person's own transport credentials,
// as review.Repos.PushEditor does. The test pushes a commit to the head
// branch, opens a pull request and closes it, and deletes the branch it
// pushed for it. Use a sandbox.
//
// VUORO_LIVE_GITHUB_STACK_PR=https://github.com/OWNER/SANDBOX/pull/N
// VUORO_LIVE_GITHUB_TOKEN=... go test -run TestLiveStack -v ./forge/github
func TestLiveStack(t *testing.T) {
	raw := os.Getenv("VUORO_LIVE_GITHUB_STACK_PR")
	if raw == "" {
		t.Skip("set VUORO_LIVE_GITHUB_STACK_PR and VUORO_LIVE_GITHUB_TOKEN to push to a sandbox pull request")
	}
	ref, err := ParseURL(raw)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	token, err := config.Forge{TokenEnv: "VUORO_LIVE_GITHUB_TOKEN"}.Token(ctx)
	if err != nil {
		t.Fatal(err)
	}
	c, err := New("https://"+ref.Host, token)
	if err != nil {
		t.Fatal(err)
	}
	ch, err := c.ReadFacts(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	target, err := c.StackTarget(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if target.Head != ch.Diff.Head || target.Branch != ch.SourceBranch {
		t.Fatalf("target %+v, change at %s on %s", target, ch.Diff.Head, ch.SourceBranch)
	}
	remote := ch.Checkout.SSH
	dir := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir, "-c", "user.name=vuoro live test", "-c", "user.email=vuoro-live@invalid", "-c", "commit.gpgsign=false"}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "--quiet")
	git("fetch", "--quiet", remote, "refs/heads/"+target.Branch)
	git("checkout", "--quiet", "--detach", "FETCH_HEAD")
	b := make([]byte, 4)
	rand.Read(b)
	nonce := hex.EncodeToString(b)
	commit := func(line string) string {
		t.Helper()
		f, err := os.OpenFile(filepath.Join(dir, "vuoro-live-stack.txt"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		f.WriteString(line + "\n")
		f.Close()
		git("add", "vuoro-live-stack.txt")
		git("commit", "--quiet", "-m", "vuoro live stack "+line)
		return git("rev-parse", "HEAD")
	}

	// Own: a commit pushed to the pull request's head branch moves it. GitHub
	// updates the pull request's head a few seconds after the push lands.
	sha := commit(nonce + " own")
	git("push", "--quiet", "--no-verify", remote, sha+":refs/heads/"+target.Branch)
	var moved forge.StackTarget
	for range 30 {
		if moved, err = c.StackTarget(ctx, ref); err != nil {
			t.Fatal(err)
		}
		if moved.Head == sha {
			break
		}
		time.Sleep(2 * time.Second)
	}
	if moved.Head != sha {
		t.Fatalf("%s is at %s after the push, want %s", target.Branch, moved.Head, sha)
	}
	t.Logf("pushed %s to %s, the pull request's own head branch", sha[:8], target.Branch)

	// Stacked: a later commit on a branch of its own, and a draft pull request
	// onto the head. It must not be the commit the head branch already holds,
	// or GitHub marks the new pull request merged at once.
	stacked := commit(nonce + " stacked")
	branch := "vuoro/live-stack-" + nonce
	git("push", "--quiet", "--no-verify", remote, stacked+":refs/heads/"+branch)
	t.Cleanup(func() {
		cmd := exec.Command("git", "-C", dir, "push", "--quiet", "--delete", remote, branch)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Logf("could not delete %s: %v %s", branch, err, out)
		}
	})
	url, openErr := c.OpenStacked(ctx, ref, forge.NewStacked{Branch: branch, Target: moved.Branch, Title: "Fix review findings (vuoro live test " + nonce + ")", Description: "Opened and closed by TestLiveStack."})
	if url != "" {
		t.Cleanup(func() {
			n := url[strings.LastIndex(url, "/")+1:]
			path := repoPath(ref) + "/pulls/" + n
			if _, err := c.do(context.Background(), http.MethodPatch, c.rest+path, path, map[string]any{"state": "closed"}); err != nil {
				t.Logf("could not close %s: %v", url, err)
			}
		})
	}
	if openErr != nil && !errors.Is(openErr, forge.ErrNotDraft) {
		t.Fatal(openErr)
	}
	t.Logf("opened %s (draft: %v)", url, openErr == nil)
}
