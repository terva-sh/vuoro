package review

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
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

// summaryInput reads the commits and the net patch from from to src.Head.
// anyFile lets a patch past the cap leave out any file, largest first after
// the lockfiles and generated files, which an agent that can read the
// history itself (an ask run, D70) may be given; without it what is left past
// the cap is refused. It answers the files left out as well.
func (r *Repos) summaryInput(ctx context.Context, src Source, from string, anyFile bool) (string, []LeftOut, error) {
	if !commitRE.MatchString(from) || !commitRE.MatchString(src.Head) || !keyRE.MatchString(src.Key) || strings.Contains(src.Key, "..") {
		return "", nil, errors.New("review: invalid summary range")
	}
	gitDir := filepath.Join(r.Dir, filepath.FromSlash(src.Key)+".git")
	unlock, err := r.lock(ctx, gitDir)
	if err != nil {
		return "", nil, errors.New("review: the summary was cancelled")
	}
	defer unlock()
	shallow, err := r.git(ctx, gitDir, "rev-parse", "--is-shallow-repository")
	if err != nil || strings.TrimSpace(shallow) != "false" {
		return "", nil, errors.New("review: complete history is unavailable for this summary")
	}
	base, err := r.git(ctx, gitDir, "merge-base", from, src.Head)
	if err != nil || !strings.EqualFold(strings.TrimSpace(base), from) {
		return "", nil, errors.New("review: the summary baseline is unavailable or history was rewritten; choose a whole-change summary")
	}
	commits, err := r.summaryGit(ctx, gitDir, maxSummaryCommits, "log", "--no-show-signature", "--format=%H %s", from+".."+src.Head)
	if err != nil {
		return "", nil, err
	}
	// The whole patch is sent when it fits, lockfiles included, since a
	// lockfile edit can be the change that matters. Past the cap, lockfiles
	// the forge base already holds and generated files are left out,
	// largest first and only until the rest fits, and named below with
	// their line counts. So a dependency bump is not refused for its
	// lockfile, and a large lockfile cannot carry a small edit to another
	// out of the patch. With anyFile, the other files follow them, largest
	// first, so nothing is refused for its size; without it what is left is
	// still refused whole past the cap.
	args := []string{"diff", "--no-ext-diff", "--no-textconv", "--no-color", "--binary", from, src.Head, "--"}
	patch, err := r.summaryGit(ctx, gitDir, maxSummaryPatch, args...)
	var left []LeftOut
	if errors.Is(err, errSummaryLarge) {
		// Attributes come from the change's forge base, never from a commit
		// on its own branch, such as the head a since-review range starts at.
		var changed []LeftOut
		if changed, err = r.changedFiles(ctx, gitDir, src.Base, from, src.Head); err != nil {
			return "", nil, err
		}
		var can, rest []LeftOut
		for _, f := range changed {
			if f.Why != "" {
				can = append(can, f)
			} else if anyFile {
				f.Why = "large"
				rest = append(rest, f)
			}
		}
		bySize := func(a, b LeftOut) int { return cmp.Compare(b.size(), a.size()) }
		slices.SortStableFunc(can, bySize)
		slices.SortStableFunc(rest, bySize)
		can = append(can, rest...)
		without := func(n int) (string, error) {
			a := append(slices.Clip(args), ".")
			for _, f := range can[:n] {
				a = append(a, ":(exclude,literal)"+f.Path)
			}
			return r.summaryGit(ctx, gitDir, maxSummaryPatch, a...)
		}
		// Leaving out more only shrinks the patch, so search for the fewest.
		lo, hi := 1, len(can)
		patch, err = "", errSummaryLarge
		for lo <= hi {
			n := (lo + hi) / 2
			p, e := without(n)
			if errors.Is(e, errSummaryLarge) {
				lo = n + 1
				continue
			}
			if e != nil {
				return "", nil, e
			}
			patch, err, left, hi = p, nil, can[:n], n-1
		}
	}
	if err != nil {
		return "", nil, err
	}
	out := summaryFence("Commits in range (untrusted):", commits) + "\n\n" + summaryFence("Exact net diff (untrusted; an empty diff means no net file changes):", patch)
	if len(left) > 0 {
		var b strings.Builder
		for _, f := range left {
			fmt.Fprintf(&b, "%s +%s -%s (%s)\n", f.Path, f.Added, f.Deleted, f.Why)
		}
		out += "\n\n" + summaryFence("Files left out of the diff above, with their added and deleted lines (untrusted names; their contents were not sent):", b.String())
	}
	return out, left, nil
}

// lockfiles are the names of dependency lockfiles. A file with one of these
// names may be left out of a patch past the cap, and only when the forge base
// already holds its path (leftOut).
var lockfiles = map[string]bool{
	"package-lock.json": true, "npm-shrinkwrap.json": true, "yarn.lock": true, "pnpm-lock.yaml": true, "bun.lockb": true, "bun.lock": true,
	"uv.lock": true, "poetry.lock": true, "Pipfile.lock": true, "pdm.lock": true, "conda-lock.yml": true,
	"Cargo.lock": true, "go.sum": true, "Gemfile.lock": true, "composer.lock": true, "mix.lock": true, "pubspec.lock": true,
	"Podfile.lock": true, "packages.lock.json": true, "flake.lock": true, "gradle.lockfile": true, "Package.resolved": true,
}

// LeftOut is a changed file whose patch was not sent to the agent, with its
// added and deleted lines as numstat counts them and why it was left out:
// lockfile, generated, marked -diff, or large.
type LeftOut struct {
	Path    string `json:"path"`
	Added   string `json:"added"`
	Deleted string `json:"deleted"`
	Why     string `json:"why"`
}

// size is the file's changed lines; a binary file, which numstat counts as
// -, sorts first.
func (f LeftOut) size() int {
	a, errA := strconv.Atoi(f.Added)
	d, errD := strconv.Atoi(f.Deleted)
	if errA != nil || errD != nil {
		return math.MaxInt
	}
	return a + d
}

// changedFiles lists the files changed between from and head with their
// line counts, giving a Why to those a patch too large to send leaves out
// first: known lockfiles that attrs, the change's forge base, already holds,
// and files its .gitattributes marks linguist-generated or -diff. Reading
// both there means the change cannot hide its own code by naming it a
// lockfile or marking it generated. A git too old to read attributes from a
// commit marks the lockfiles alone.
func (r *Repos) changedFiles(ctx context.Context, gitDir, attrs, from, head string) ([]LeftOut, error) {
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
	// A lockfile counts only where the forge base already has one, so a
	// change cannot leave out a new file by giving it a lockfile's name.
	var named []string
	for _, p := range paths {
		if lockfiles[filepath.Base(filepath.FromSlash(p))] {
			named = append(named, p)
		}
	}
	why := map[string]string{}
	if len(named) > 0 {
		have, err := r.summaryGit(ctx, gitDir, maxSummaryPatch, append([]string{"ls-tree", "-z", "--name-only", "--full-tree", attrs, "--"}, named...)...)
		if err != nil {
			return nil, err
		}
		for _, p := range strings.Split(have, "\x00") {
			if slices.Contains(named, p) {
				why[p] = "lockfile"
			}
		}
	}
	if len(paths) > 0 {
		marks, err := r.summaryGit(ctx, gitDir, maxSummaryPatch, append([]string{"check-attr", "--source=" + attrs, "-z", "linguist-generated", "diff", "--"}, paths...)...)
		if err == nil {
			f := strings.Split(marks, "\x00")
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
	var out []LeftOut
	for _, p := range paths {
		c := changed[p]
		out = append(out, LeftOut{Path: p, Added: c.added, Deleted: c.deleted, Why: why[p]})
	}
	return out, nil
}
