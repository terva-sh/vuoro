package review

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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
	left, err := r.leftOut(ctx, gitDir, from, src.Head)
	if err != nil {
		return "", err
	}
	// The patch leaves out lockfiles and generated files, which are named
	// below with their line counts, so a dependency bump is not refused for
	// its lockfile. What is left is still refused whole past the cap.
	args := []string{"diff", "--no-ext-diff", "--no-textconv", "--no-color", "--binary", from, src.Head, "--"}
	if len(left) > 0 {
		args = append(args, ".")
		for _, f := range left {
			args = append(args, ":(exclude,literal)"+f.path)
		}
	}
	patch, err := r.summaryGit(ctx, gitDir, maxSummaryPatch, args...)
	if err != nil {
		return "", err
	}
	out := summaryFence("Commits in range (untrusted):", commits) + "\n\n" + summaryFence("Exact net diff (untrusted; an empty diff means no net file changes):", patch)
	if len(left) > 0 {
		var b strings.Builder
		for _, f := range left {
			fmt.Fprintf(&b, "%s +%s -%s (%s)\n", f.path, f.added, f.deleted, f.why)
		}
		out += "\n\n" + summaryFence("Files left out of the diff above, with their added and deleted lines (untrusted names; their contents were not sent):", b.String())
	}
	return out, nil
}

// lockfiles are the dependency lockfiles a summary leaves out of its patch.
var lockfiles = map[string]bool{
	"package-lock.json": true, "npm-shrinkwrap.json": true, "yarn.lock": true, "pnpm-lock.yaml": true, "bun.lockb": true, "bun.lock": true,
	"uv.lock": true, "poetry.lock": true, "Pipfile.lock": true, "pdm.lock": true, "conda-lock.yml": true,
	"Cargo.lock": true, "go.sum": true, "Gemfile.lock": true, "composer.lock": true, "mix.lock": true, "pubspec.lock": true,
	"Podfile.lock": true, "packages.lock.json": true, "flake.lock": true, "gradle.lockfile": true, "Package.resolved": true,
}

type leftOutFile struct{ path, added, deleted, why string }

// leftOut lists the changed files a summary leaves out of its patch: known
// lockfiles, and files the base commit's .gitattributes marks
// linguist-generated or -diff. The attributes are read at the base, so the
// change cannot hide its own code by marking it generated. A git too old to
// read attributes from a commit leaves out the lockfiles alone.
func (r *Repos) leftOut(ctx context.Context, gitDir, from, head string) ([]leftOutFile, error) {
	stat, err := r.summaryGit(ctx, gitDir, maxSummaryPatch, "diff", "--numstat", "-z", "--no-renames", "--no-ext-diff", "--no-textconv", from, head, "--")
	if err != nil {
		return nil, err
	}
	type counts struct{ added, deleted string }
	changed := map[string]counts{}
	var paths []string
	for _, rec := range strings.Split(stat, "\x00") {
		added, rest, ok := strings.Cut(rec, "\t")
		if !ok {
			continue
		}
		deleted, path, ok := strings.Cut(rest, "\t")
		if !ok || path == "" {
			continue
		}
		changed[path] = counts{added, deleted}
		paths = append(paths, path)
	}
	why := map[string]string{}
	for _, p := range paths {
		if lockfiles[filepath.Base(filepath.FromSlash(p))] {
			why[p] = "lockfile"
		}
	}
	if len(paths) > 0 {
		attrs, err := r.summaryGit(ctx, gitDir, maxSummaryPatch, append([]string{"check-attr", "--source=" + from, "-z", "linguist-generated", "diff", "--"}, paths...)...)
		if err == nil {
			f := strings.Split(attrs, "\x00")
			for i := 0; i+2 < len(f); i += 3 {
				path, attr, value := f[i], f[i+1], f[i+2]
				switch {
				case why[path] != "":
				case attr == "linguist-generated" && (value == "set" || value == "true"):
					why[path] = "generated"
				case attr == "diff" && value == "unset":
					why[path] = "marked -diff"
				}
			}
		}
	}
	var out []leftOutFile
	for _, p := range paths {
		if why[p] != "" {
			c := changed[p]
			out = append(out, leftOutFile{path: p, added: c.added, deleted: c.deleted, why: why[p]})
		}
	}
	return out, nil
}
