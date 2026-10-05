package review

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/terva-sh/vuoro/state"
)

// Repos keeps a bare, partial clone of each reviewed project under Dir, and
// checks out worktrees from them (docs/plan.md section 8). It fetches over
// the person's own git transport, with their own keys or credential helper,
// and vuoro never holds a git credential.
type Repos struct {
	Dir string // repos/ under the state directory
	Git string // the git binary; empty for git on PATH

	mu    sync.Mutex
	locks map[string]chan struct{}
}

// Source is what to fetch for one change.
type Source struct {
	Remote   string   // the clone URL
	Key      string   // where the clone lives under Dir, such as gitlab.com/group/project
	Refspecs []string // the refs that bring in Head and Base
	Head     string   // the SHA to check out
	Base     string   // a SHA that must arrive too
}

// gitTimeout bounds one git command. A first fetch of a large project is the
// slow one.
const gitTimeout = 10 * time.Minute

var keyRE = regexp.MustCompile(`^[A-Za-z0-9._-]+(/[A-Za-z0-9._-]+)+$`)

// Checkout fetches src into its clone and checks out src.Head, detached, at
// dir. dir must not exist yet.
func (r *Repos) Checkout(ctx context.Context, src Source, dir string) error {
	if !keyRE.MatchString(src.Key) || strings.Contains(src.Key, "..") {
		return fmt.Errorf("review: bad repository key %q", src.Key)
	}
	gitDir := filepath.Join(r.Dir, filepath.FromSlash(src.Key)+".git")
	unlock, err := r.lock(ctx, gitDir)
	if err != nil {
		return err
	}
	defer unlock()

	if err := r.fetch(ctx, gitDir, src); err != nil {
		return err
	}
	for _, sha := range []string{src.Head, src.Base} {
		if sha == "" {
			continue
		}
		if _, err := r.git(ctx, gitDir, "cat-file", "-e", sha+"^{commit}"); err != nil {
			return fmt.Errorf("review: %s did not arrive with the fetch", short(sha))
		}
	}
	if err := state.MkdirAll(filepath.Dir(dir)); err != nil {
		return err
	}
	_, err = r.git(ctx, gitDir, "worktree", "add", "--quiet", "--detach", dir, src.Head)
	return err
}

// fetch runs while the repository lock is held. It never changes a worktree.
func (r *Repos) fetch(ctx context.Context, gitDir string, src Source) error {
	if _, err := os.Stat(filepath.Join(gitDir, "HEAD")); errors.Is(err, os.ErrNotExist) {
		if err := state.MkdirAll(filepath.Dir(gitDir)); err != nil {
			return err
		}
		if _, err := r.git(ctx, "", "init", "--quiet", "--bare", gitDir); err != nil {
			return err
		}
		if _, err := r.git(ctx, gitDir, "remote", "add", "origin", src.Remote); err != nil {
			return err
		}
	} else if _, err := r.git(ctx, gitDir, "remote", "set-url", "origin", src.Remote); err != nil {
		return err
	}
	// Blobs arrive when the worktree is checked out, so a large project's
	// history costs commits and trees only.
	fetch := append([]string{"fetch", "--quiet", "--filter=blob:none", "--no-tags", "origin"}, src.Refspecs...)
	if _, err := r.git(ctx, gitDir, fetch...); err != nil {
		return err
	}
	return nil
}

// Remove deletes the worktree at dir, which came from src's clone.
func (r *Repos) Remove(ctx context.Context, key, dir string) error {
	gitDir := filepath.Join(r.Dir, filepath.FromSlash(key)+".git")
	unlock, err := r.lock(ctx, gitDir)
	if err != nil {
		return err
	}
	defer unlock()
	_, err = r.git(ctx, gitDir, "worktree", "remove", "--force", dir)
	if rmErr := os.RemoveAll(dir); err == nil {
		err = rmErr
	}
	_, _ = r.git(ctx, gitDir, "worktree", "prune")
	return err
}

func (r *Repos) lock(ctx context.Context, gitDir string) (func(), error) {
	r.mu.Lock()
	if r.locks == nil {
		r.locks = map[string]chan struct{}{}
	}
	m := r.locks[gitDir]
	if m == nil {
		m = make(chan struct{}, 1)
		r.locks[gitDir] = m
	}
	r.mu.Unlock()
	select {
	case m <- struct{}{}:
		return func() { <-m }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// git runs one git command. No hook runs, git never prompts, and an error
// names the subcommand and git's last line of output, which carries no
// credential: git keeps those in its helper.
func (r *Repos) git(ctx context.Context, gitDir string, args ...string) (string, error) {
	bin := r.Git
	if bin == "" {
		bin = "git"
	}
	ctx, cancel := context.WithTimeout(ctx, gitTimeout)
	defer cancel()
	full := []string{"-c", "core.hooksPath=" + os.DevNull, "-c", "advice.detachedHead=false"}
	if gitDir != "" {
		full = append(full, "--git-dir="+gitDir)
	}
	full = append(full, args...)
	cmd := exec.CommandContext(ctx, bin, full...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if os.Getenv("GIT_SSH_COMMAND") == "" {
		cmd.Env = append(cmd.Env, "GIT_SSH_COMMAND=ssh -o BatchMode=yes")
	}
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("review: git %s: %s: %w", args[0], lastLine(errb.String(), err), err)
	}
	return out.String(), nil
}

func lastLine(s string, err error) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if l := strings.TrimSpace(lines[len(lines)-1]); l != "" {
		return l
	}
	return err.Error()
}

func short(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

// gitDir is the clone of a repository key.
func (r *Repos) gitDir(key string) (string, error) {
	if !keyRE.MatchString(key) || strings.Contains(key, "..") {
		return "", fmt.Errorf("review: bad repository key %q", key)
	}
	return filepath.Join(r.Dir, filepath.FromSlash(key)+".git"), nil
}

// editorRef is where an editor's commits are kept in the clone, once fetched.
func editorRef(runID string) string { return "refs/vuoro/editor/" + runID }

// SeedEditor makes dir a new repository holding head, from key's clone, on a
// new branch. It shares nothing with the clone, so the editor writes only its
// own repository (D40). The run's checkout has brought head's files into the
// clone, which a shallow fetch needs. dir must not exist yet.
func (r *Repos) SeedEditor(ctx context.Context, key, runID, head, branch, dir string) error {
	gitDir, err := r.gitDir(key)
	if err != nil {
		return err
	}
	unlock, err := r.lock(ctx, gitDir)
	if err != nil {
		return err
	}
	defer unlock()
	base := "refs/vuoro/editor-base/" + runID
	if _, err := r.git(ctx, gitDir, "update-ref", base, head); err != nil {
		return err
	}
	if err := state.MkdirAll(dir); err != nil {
		return err
	}
	if _, err := r.git(ctx, "", "init", "--quiet", dir); err != nil {
		return err
	}
	work := "--work-tree=" + dir
	ed := filepath.Join(dir, ".git")
	if _, err := r.git(ctx, ed, "fetch", "--quiet", "--depth=1", "--no-tags", gitDir, base); err != nil {
		return err
	}
	if _, err := r.git(ctx, ed, work, "checkout", "--quiet", "-b", branch, "FETCH_HEAD"); err != nil {
		return err
	}
	got, err := r.git(ctx, ed, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if strings.TrimSpace(got) != head {
		return fmt.Errorf("review: the editor's checkout is at %s, not %s", short(got), short(head))
	}
	return nil
}

// FetchEditor brings branch's commits from the editor's repository at dir
// into key's clone, and returns the ref that holds them. vuoro never runs git
// inside the editor's repository, whose configuration the agent could have
// written. Serving a fetch runs no command that configuration names, and the
// upload-pack it starts is told to ignore fsmonitor and hooks too.
func (r *Repos) FetchEditor(ctx context.Context, key, runID, branch, dir string) (string, error) {
	gitDir, err := r.gitDir(key)
	if err != nil {
		return "", err
	}
	unlock, err := r.lock(ctx, gitDir)
	if err != nil {
		return "", err
	}
	defer unlock()
	ref := editorRef(runID)
	pack := "--upload-pack=git -c core.fsmonitor=false -c core.hooksPath=" + os.DevNull + " upload-pack"
	if _, err := r.git(ctx, gitDir, "fetch", "--quiet", "--no-tags", "--no-write-fetch-head", pack, dir, "+refs/heads/"+branch+":"+ref); err != nil {
		return "", err
	}
	return ref, nil
}

// DropEditor removes an editor's refs from key's clone.
func (r *Repos) DropEditor(ctx context.Context, key, runID string) error {
	gitDir, err := r.gitDir(key)
	if err != nil {
		return err
	}
	unlock, err := r.lock(ctx, gitDir)
	if err != nil {
		return err
	}
	defer unlock()
	for _, ref := range []string{editorRef(runID), "refs/vuoro/editor-base/" + runID} {
		_, _ = r.git(ctx, gitDir, "update-ref", "-d", ref)
	}
	return nil
}
