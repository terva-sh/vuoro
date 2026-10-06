package forgejo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
)

// TestLiveStack hands an editor's work back to a real pull request both ways:
// pushed to its own head branch, and as a draft pull request stacked onto that
// branch. It reads a fork's pull request to see it refused.
//
// tea owns the API's authentication, as in TestLiveWrite, and git pushes with
// the person's own transport over the pull request's SSH clone URL, as
// review.Repos.PushEditor does. The test pushes a commit to the pull
// request's head branch, opens a pull request and closes it, and deletes the
// branch it pushed for it. Use a sandbox.
//
//	VUORO_LIVE_FORGEJO_STACK_PR=https://HOST/OWNER/REPO/pulls/N \
//	VUORO_LIVE_FORGEJO_FORK_PR=https://HOST/OWNER/REPO/pulls/M \
//	VUORO_LIVE_FORGEJO_STACKER=<tea login> \
//	go test -run TestLiveStack -v ./forge/forgejo
func TestLiveStack(t *testing.T) {
	raw, fork, login := os.Getenv("VUORO_LIVE_FORGEJO_STACK_PR"), os.Getenv("VUORO_LIVE_FORGEJO_FORK_PR"), os.Getenv("VUORO_LIVE_FORGEJO_STACKER")
	if raw == "" || fork == "" || login == "" {
		t.Skip("set VUORO_LIVE_FORGEJO_STACK_PR, VUORO_LIVE_FORGEJO_FORK_PR and VUORO_LIVE_FORGEJO_STACKER to push to a sandbox pull request")
	}
	ref, err := ParseURL(raw)
	if err != nil {
		t.Fatal(err)
	}
	forkRef, err := ParseURL(fork)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	// The secret only marks the client as signed in; tea sends its own.
	c, err := New("https://"+ref.Host, config.SecretForTest("held-by-tea"))
	if err != nil {
		t.Fatal(err)
	}
	c.http.Transport = teaWriter(login)
	nonce := time.Now().UTC().Format("20060102T150405Z")

	if _, err := c.StackTarget(ctx, forkRef); !errors.Is(err, forge.ErrFork) {
		t.Fatalf("a fork's pull request: %v", err)
	}
	ch, err := c.Open(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	remote := ch.Checkout.SSH
	target, err := c.StackTarget(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}

	// The editor's work: one commit on the head the forge reported.
	dir := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir, "-c", "user.name=vuoro live test", "-c", "user.email=vuoro-live@invalid", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "--quiet")
	git("fetch", "--quiet", remote, "refs/heads/"+target.Branch)
	if got := git("rev-parse", "FETCH_HEAD"); got != target.Head {
		t.Fatalf("%s is at %s, but the forge reported %s", target.Branch, got, target.Head)
	}
	git("checkout", "--quiet", "--detach", "FETCH_HEAD")
	if err := os.WriteFile(filepath.Join(dir, "vuoro-live-stack.txt"), []byte(nonce+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "vuoro-live-stack.txt")
	git("commit", "--quiet", "-m", "vuoro live stack "+nonce)
	sha := git("rev-parse", "HEAD")

	// Somebody else's change (D39): the commit goes to a branch of its own,
	// and a draft pull request onto the head branch.
	branch := "vuoro/live-stack-" + nonce
	git("push", "--quiet", "--no-verify", remote, sha+":refs/heads/"+branch)
	t.Cleanup(func() {
		cmd := exec.Command("git", "-C", dir, "push", "--quiet", "--delete", remote, branch)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("deleting %s: %v\n%s", branch, err, out)
		}
	})
	url, openErr := c.OpenStacked(ctx, ref, forge.NewStacked{Branch: branch, Target: target.Branch, Title: "Fix review findings (vuoro live test " + nonce + ")", Description: "Opened and closed by TestLiveStack."})
	// A pull request that opened is closed, even one that is not a draft.
	n, err := strconv.Atoi(path.Base(url))
	if url != "" && err != nil {
		t.Fatalf("the new pull request's URL %q", url)
	}
	opened := fmt.Sprintf("%s/pulls/%d", repoPath(ref), n)
	if url != "" {
		t.Cleanup(func() {
			if _, err := c.write(context.Background(), "PATCH", opened, map[string]any{"state": "closed"}); err != nil {
				t.Errorf("closing %s: %v", url, err)
			}
		})
	}
	if openErr != nil {
		t.Fatalf("open: %v", openErr)
	}
	pr, err := read[apiPR](ctx, c, opened)
	if err != nil {
		t.Fatal(err)
	}
	if !pr.Draft || pr.Head.Ref != branch || pr.Base.Ref != target.Branch || pr.Head.SHA != sha {
		t.Fatalf("the stacked pull request: draft %v, %s at %s onto %s", pr.Draft, pr.Head.Ref, pr.Head.SHA, pr.Base.Ref)
	}
	t.Logf("opened %s, a draft from %s onto %s", url, branch, target.Branch)

	// The person's own change (D41): the same commit fast-forwards its head
	// branch, and the forge reports the new head.
	git("push", "--quiet", "--no-verify", remote, sha+":refs/heads/"+target.Branch)
	moved, err := c.StackTarget(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if moved.Head != sha {
		t.Fatalf("%s is at %s after the push, want %s", target.Branch, moved.Head, sha)
	}
	t.Logf("pushed %s to %s, the pull request's own head branch", sha[:8], target.Branch)
}
