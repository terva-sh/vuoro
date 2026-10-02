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
	locks map[string]*sync.Mutex
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
	unlock := r.lock(gitDir)
	defer unlock()

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
	_, err := r.git(ctx, gitDir, "worktree", "add", "--quiet", "--detach", dir, src.Head)
	return err
}

// Remove deletes the worktree at dir, which came from src's clone.
func (r *Repos) Remove(ctx context.Context, key, dir string) error {
	gitDir := filepath.Join(r.Dir, filepath.FromSlash(key)+".git")
	unlock := r.lock(gitDir)
	defer unlock()
	_, err := r.git(ctx, gitDir, "worktree", "remove", "--force", dir)
	if rmErr := os.RemoveAll(dir); err == nil {
		err = rmErr
	}
	_, _ = r.git(ctx, gitDir, "worktree", "prune")
	return err
}

func (r *Repos) lock(gitDir string) func() {
	r.mu.Lock()
	if r.locks == nil {
		r.locks = map[string]*sync.Mutex{}
	}
	m := r.locks[gitDir]
	if m == nil {
		m = &sync.Mutex{}
		r.locks[gitDir] = m
	}
	r.mu.Unlock()
	m.Lock()
	return m.Unlock
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
		return "", fmt.Errorf("review: git %s: %s", args[0], lastLine(errb.String(), err))
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
