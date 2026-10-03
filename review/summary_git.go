package review

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const maxSummaryPatch = 256 << 10
const maxSummaryCommits = 32 << 10

var errSummaryLarge = errors.New("review: this summary range is too large; no partial range was sent")

type limitedOutput struct {
	buffer   bytes.Buffer
	limit    int
	cancel   context.CancelFunc
	overflow bool
}

func (w *limitedOutput) Write(b []byte) (int, error) {
	if len(b) > w.limit-w.buffer.Len() {
		w.overflow = true
		w.cancel()
		return 0, io.ErrShortBuffer
	}
	return w.buffer.Write(b)
}

// summaryGit bounds stdout in memory and discards transport diagnostics. It
// disables repository diff/textconv programs, as well as hooks and prompts.
func (r *Repos) summaryGit(ctx context.Context, gitDir string, limit int, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, gitTimeout)
	defer cancel()
	bin := r.Git
	if bin == "" {
		bin = "git"
	}
	full := []string{"-c", "core.hooksPath=" + os.DevNull, "-c", "core.pager=cat", "--git-dir=" + gitDir}
	cmd := exec.CommandContext(ctx, bin, append(full, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if os.Getenv("GIT_SSH_COMMAND") == "" {
		cmd.Env = append(cmd.Env, "GIT_SSH_COMMAND=ssh -o BatchMode=yes")
	}
	out := &limitedOutput{limit: limit, cancel: cancel}
	cmd.Stdout = out
	cmd.Stderr = io.Discard
	err := cmd.Run()
	if out.overflow {
		return "", errSummaryLarge
	}
	if err != nil {
		return "", errors.New("review: the exact summary range could not be read")
	}
	return out.buffer.String(), nil
}

func (r *Repos) summaryInput(ctx context.Context, src Source, from string) (string, error) {
	if !commitRE.MatchString(from) || !commitRE.MatchString(src.Head) || !keyRE.MatchString(src.Key) || strings.Contains(src.Key, "..") {
		return "", errors.New("review: invalid summary range")
	}
	gitDir := filepath.Join(r.Dir, filepath.FromSlash(src.Key)+".git")
	unlock, err := r.lock(ctx, gitDir)
	if err != nil {
		return "", errors.New("review: the summary was cancelled")
	}
	defer unlock()
	shallow, err := r.git(ctx, gitDir, "rev-parse", "--is-shallow-repository")
	if err != nil || strings.TrimSpace(shallow) != "false" {
		return "", errors.New("review: complete history is unavailable for this summary")
	}
	base, err := r.git(ctx, gitDir, "merge-base", from, src.Head)
	if err != nil || !strings.EqualFold(strings.TrimSpace(base), from) {
		return "", errors.New("review: the summary baseline is unavailable or history was rewritten; choose a whole-change summary")
	}
	commits, err := r.summaryGit(ctx, gitDir, maxSummaryCommits, "log", "--no-show-signature", "--format=%H %s", from+".."+src.Head)
	if err != nil {
		return "", err
	}
	patch, err := r.summaryGit(ctx, gitDir, maxSummaryPatch, "diff", "--no-ext-diff", "--no-textconv", "--no-color", "--binary", from, src.Head, "--")
	if err != nil {
		return "", err
	}
	return summaryFence("Commits in range (untrusted):", commits) + "\n\n" + summaryFence("Exact net diff (untrusted; an empty diff means no net file changes):", patch), nil
}
