package review

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/terva-sh/vuoro/forge"
)

// Local is the kind of a change that is a range of commits in a checkout,
// with no forge behind it (docs/plan.md section 6.7).
const Local = "local"

// LocalHost is the Ref host of a local change.
const LocalHost = "local"

const (
	// maxLocalPatch bounds one file's patch; a larger file is listed as
	// omitted, as a forge does with a diff it will not show.
	maxLocalPatch = 512 << 10
	// maxLocalLog bounds the commit messages that become the description.
	maxLocalLog = 32 << 10
	// maxLocalFiles bounds how many files a local range may touch.
	maxLocalFiles = 2000
)

// LocalTarget is a local range resolved for review.
type LocalTarget struct {
	Change *forge.Change
	Source Source
	// Dirty says the checkout has uncommitted changes to tracked files,
	// which a review of commits does not see.
	Dirty bool
}

// ErrNothingToReview is a range whose head is its merge base.
var ErrNothingToReview = errors.New("review: the range has no commits to review")

var nameUnsafe = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// LocalChange resolves target in the checkout at dir into a change and the
// source that fetches it (docs/plan.md section 6.7). target is "A..B" or
// "A...B", both meaning B against its merge base with A; a single revision A,
// meaning A...HEAD; or empty, meaning the upstream against HEAD. git runs with
// hooks, fsmonitor, external diff programs, and textconv disabled, because
// the checkout's configuration is not the person's choice to run code.
func LocalChange(ctx context.Context, gitBin, dir, target string) (*LocalTarget, error) {
	g := localGit{bin: gitBin, dir: dir}
	top, err := g.run(ctx, 4<<10, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("review: %s is not a git checkout", dir)
	}
	top = strings.TrimSpace(top)
	g.dir = top

	baseRev, headRev, err := splitRange(target)
	if err != nil {
		return nil, err
	}
	if baseRev == "" {
		if baseRev, err = g.upstream(ctx); err != nil {
			return nil, err
		}
	}
	head, err := g.commit(ctx, headRev)
	if err != nil {
		return nil, err
	}
	other, err := g.commit(ctx, baseRev)
	if err != nil {
		return nil, err
	}
	base, err := g.run(ctx, 4<<10, "merge-base", other, head)
	if err != nil {
		return nil, fmt.Errorf("review: %s and %s have no common history", baseRev, headRev)
	}
	base = strings.TrimSpace(base)
	if base == head {
		return nil, ErrNothingToReview
	}

	files, err := g.files(ctx, base, head)
	if err != nil {
		return nil, err
	}
	title, _ := g.run(ctx, 4<<10, "log", "-1", "--format=%s", head)
	log, _ := g.run(ctx, maxLocalLog, "log", "--format=%h %s%n%n%b", base+".."+head)
	dirty, _ := g.run(ctx, 64<<10, "status", "--porcelain", "--untracked-files=no")

	name := nameUnsafe.ReplaceAllString(filepath.Base(top), "-")
	if name == "" || name == "." || name == ".." {
		name = "checkout"
	}
	sum := sha256.Sum256([]byte(top))
	project := name + "-" + hex.EncodeToString(sum[:4])
	ch := &forge.Change{
		Kind:         Local,
		Ref:          forge.Ref{Host: LocalHost, Project: project},
		Title:        strings.TrimSpace(title),
		Description:  strings.TrimSpace(log),
		SourceBranch: headRev,
		TargetBranch: baseRev,
		Diff:         forge.DiffRefs{Base: base, Start: base, Head: head},
		State:        forge.Open,
		Created:      time.Now().UTC(),
		Files:        files,
	}
	src := Source{Remote: top, Key: LocalHost + "/" + project, Refspecs: []string{"+" + head + ":refs/vuoro/head"}, Head: head, Base: base}
	return &LocalTarget{Change: ch, Source: src, Dirty: strings.TrimSpace(dirty) != ""}, nil
}

// splitRange returns the two sides of target; an empty base means the
// upstream.
func splitRange(target string) (base, head string, err error) {
	if strings.HasPrefix(target, "-") || strings.ContainsAny(target, "\x00\n") {
		return "", "", fmt.Errorf("review: %q is not a revision or a range", target)
	}
	switch {
	case target == "":
		return "", "HEAD", nil
	case strings.Contains(target, "..."):
		base, head, _ = strings.Cut(target, "...")
	case strings.Contains(target, ".."):
		base, head, _ = strings.Cut(target, "..")
	default:
		return target, "HEAD", nil
	}
	if base == "" {
		base = "HEAD"
	}
	if head == "" {
		head = "HEAD"
	}
	if strings.HasPrefix(head, "-") || strings.Contains(head, "..") {
		return "", "", fmt.Errorf("review: %q is not a revision or a range", target)
	}
	return base, head, nil
}

type localGit struct {
	bin string
	dir string
}

// run runs one git command in the checkout and bounds its output.
func (g localGit) run(ctx context.Context, limit int, args ...string) (string, error) {
	bin := g.bin
	if bin == "" {
		bin = "git"
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	full := []string{"-C", g.dir, "--literal-pathspecs",
		"-c", "core.hooksPath=" + os.DevNull, "-c", "core.fsmonitor=false", "-c", "core.pager=cat",
		"-c", "diff.external=", "-c", "core.quotePath=false"}
	cmd := exec.CommandContext(ctx, bin, append(full, args...)...)
	// No index refresh is written, and git never prompts.
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0")
	out := &limitedOutput{limit: limit, cancel: cancel}
	var errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = out, &errb
	err := cmd.Run()
	if out.overflow {
		return "", errTooLarge
	}
	if err != nil {
		return "", fmt.Errorf("review: git %s: %s", args[0], lastLine(errb.String(), err))
	}
	return out.buffer.String(), nil
}

var errTooLarge = errors.New("review: git printed more than vuoro reads")

// commit resolves rev to a full commit ID.
func (g localGit) commit(ctx context.Context, rev string) (string, error) {
	out, err := g.run(ctx, 4<<10, "rev-parse", "--verify", "--quiet", "--end-of-options", rev+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("review: %q is not a commit in this checkout", rev)
	}
	return strings.TrimSpace(out), nil
}

// upstream is the revision a range with no base is reviewed against: the
// branch's upstream, then origin/HEAD, then origin/main.
func (g localGit) upstream(ctx context.Context) (string, error) {
	if out, err := g.run(ctx, 4<<10, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"); err == nil {
		if u := strings.TrimSpace(out); u != "" {
			return u, nil
		}
	}
	for _, rev := range []string{"origin/HEAD", "origin/main"} {
		if _, err := g.commit(ctx, rev); err == nil {
			return rev, nil
		}
	}
	return "", errors.New("review: this branch has no upstream, origin/HEAD or origin/main; name a range such as main..HEAD")
}

// files lists the range's files with a patch each, as a forge would send
// them.
func (g localGit) files(ctx context.Context, base, head string) ([]forge.FileDiff, error) {
	raw, err := g.run(ctx, 4<<20, "diff", "--raw", "-z", "-M", "--no-abbrev", base, head)
	if err != nil {
		return nil, err
	}
	inv, err := parseRaw(raw)
	if err != nil {
		return nil, err
	}
	if len(inv) > maxLocalFiles {
		return nil, fmt.Errorf("review: the range touches %d files; vuoro reviews at most %d", len(inv), maxLocalFiles)
	}
	for i := range inv {
		f := &inv[i]
		paths := []string{f.NewPath}
		if f.OldPath != f.NewPath {
			paths = []string{f.OldPath, f.NewPath}
		}
		args := append([]string{"diff", "--no-ext-diff", "--no-textconv", "--no-color", "-M", "-U3", base, head, "--"}, paths...)
		out, err := g.run(ctx, maxLocalPatch, args...)
		if errors.Is(err, errTooLarge) {
			f.Omitted = true
			continue
		}
		if err != nil {
			return nil, err
		}
		if strings.Contains(out, "\nBinary files ") || strings.HasPrefix(out, "Binary files ") || strings.Contains(out, "\nGIT binary patch") {
			f.Omitted = true
			continue
		}
		if at := strings.Index(out, "\n@@ "); at >= 0 {
			f.Patch = out[at+1:]
		}
	}
	return inv, nil
}

// parseRaw reads `git diff --raw -z`: a status record, then one path, or two
// for a rename or a copy.
func parseRaw(raw string) ([]forge.FileDiff, error) {
	fields := strings.Split(strings.TrimSuffix(raw, "\x00"), "\x00")
	var out []forge.FileDiff
	for i := 0; i < len(fields); {
		meta := fields[i]
		if meta == "" {
			i++
			continue
		}
		parts := strings.Fields(meta)
		if !strings.HasPrefix(meta, ":") || len(parts) < 5 || i+1 >= len(fields) {
			return nil, errors.New("review: git diff --raw printed what vuoro cannot read")
		}
		status := parts[4]
		f := forge.FileDiff{OldPath: fields[i+1], NewPath: fields[i+1]}
		i += 2
		switch status[0] {
		case 'R', 'C':
			if i >= len(fields) {
				return nil, errors.New("review: git diff --raw printed what vuoro cannot read")
			}
			f.NewPath = fields[i]
			f.Renamed = status[0] == 'R'
			f.New = status[0] == 'C'
			i++
		case 'A':
			f.New = true
		case 'D':
			f.Deleted = true
		}
		out = append(out, f)
	}
	return out, nil
}
