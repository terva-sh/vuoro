package review

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

const maxSummaryPatch = 256 << 10

// maxListing bounds what vuoro reads for itself and does not send whole: a
// file list, a commit list it trims to fit, or a patch it leaves files out of
// (D70).
const maxListing = 64 << 20
const maxSummaryCommits = 32 << 10

var errSummaryLarge = errors.New("review: this summary range is too large; no partial range was sent")

// errRangeHuge refuses an ask whose range is past what vuoro reads for
// itself, and errRangeSplit one whose patch vuoro could not match to its
// files; neither sends a partial range.
var (
	errRangeHuge  = errors.New("review: this range is past the 64 MiB vuoro reads for a conversation; ask about a narrower range")
	errRangeSplit = errors.New("review: vuoro could not match this range's patch to its files, so nothing was sent")
)

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
	return r.summaryGitIn(ctx, gitDir, limit, "", args...)
}

// summaryGitIn is summaryGit with stdin, for paths too many to pass as
// arguments.
func (r *Repos) summaryGitIn(ctx context.Context, gitDir string, limit int, stdin string, args ...string) (string, error) {
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
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
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

// summaryInput reads the commits and the net patch from from to src.Head,
// and answers the files it left out of the patch. anyFile is for an agent
// that can read the history itself (an ask run, D70): its commit list keeps
// the newest commits that fit, and its patch may leave out any file. Without
// it a commit list past its cap, or what is left of a patch past its cap, is
// refused.
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
	logArgs := []string{"log", "--no-show-signature", "--format=%H %s", from + ".." + src.Head}
	commits, err := r.summaryGit(ctx, gitDir, maxSummaryCommits, logArgs...)
	if errors.Is(err, errSummaryLarge) && anyFile {
		// An ask is given the newest commits that fit, whole lines only, and
		// told how many older ones it can read with git_log.
		commits, err = r.summaryGit(ctx, gitDir, maxListing, logArgs...)
		if err == nil {
			commits = fitLines(commits, maxSummaryCommits, "older commits not listed; read them with git_log")
		} else if errors.Is(err, errSummaryLarge) {
			err = errRangeHuge
		}
	}
	if err != nil {
		return "", nil, err
	}
	// The whole patch is sent when it fits, lockfiles included, since a
	// lockfile edit can be the change that matters. Past the cap, lockfiles
	// the forge base already holds and generated files are left out,
	// largest first and only until the rest fits, and named below with
	// their line counts. So a dependency bump is not refused for its
	// lockfile, and a large lockfile cannot carry a small edit to another
	// out of the patch. With anyFile, the other files follow them (fitPatch).
	args := []string{"diff", "--no-ext-diff", "--no-textconv", "--no-color", "--binary", from, src.Head, "--"}
	patch, err := r.summaryGit(ctx, gitDir, maxSummaryPatch, args...)
	var left []LeftOut
	if errors.Is(err, errSummaryLarge) {
		patch, left, err = r.fitPatch(ctx, gitDir, src, from, anyFile)
	}
	if err != nil {
		return "", nil, err
	}
	out := summaryFence("Commits in range (untrusted):", commits) + "\n\n" + summaryFence("Exact net diff (untrusted; an empty diff means no net file changes):", patch)
	if len(left) > 0 {
		var b strings.Builder
		for _, f := range left {
			// A path holding a control character is quoted, so it cannot
			// write lines of its own into the list.
			path := f.Path
			if strings.ContainsFunc(path, unicode.IsControl) {
				path = strconv.Quote(path)
			}
			fmt.Fprintf(&b, "%s +%s -%s (%s)\n", path, f.Added, f.Deleted, f.Why)
		}
		list := fitLines(b.String(), maxSummaryCommits, "more files left out, not listed")
		out += "\n\n" + summaryFence("Files left out of the diff above, with their added and deleted lines (untrusted names; their contents were not sent):", list)
	}
	return out, left, nil
}

// fitPatch is the patch from from to src.Head with files left out until it
// fits under the cap: lockfiles and generated files first, then, with
// anyFile, any other file, each largest first. It reads the whole patch once
// and drops files' sections, so leaving out thousands of files costs no more
// than one. Without anyFile, what is left past the cap is refused.
func (r *Repos) fitPatch(ctx context.Context, gitDir string, src Source, from string, anyFile bool) (string, []LeftOut, error) {
	// Attributes come from the change's forge base, never from a commit on
	// its own branch, such as the head a since-review range starts at.
	huge := func(err error) error {
		if errors.Is(err, errSummaryLarge) {
			if anyFile {
				return errRangeHuge
			}
			return errSummaryLarge
		}
		return err
	}
	changed, err := r.changedFiles(ctx, gitDir, src.Base, from, src.Head)
	if err != nil {
		return "", nil, huge(err)
	}
	// The prefixes are pinned, since sectionsMatch reads them and the
	// person's diff.noprefix or diff.srcPrefix would change them.
	full, err := r.summaryGit(ctx, gitDir, maxListing, "diff", "--no-ext-diff", "--no-textconv", "--no-color", "--binary", "--no-renames", "--src-prefix=a/", "--dst-prefix=b/", from, src.Head, "--")
	if err != nil {
		return "", nil, huge(err)
	}
	sections := splitPatch(full)
	if !sectionsMatch(sections, changed) {
		// numstat and the patch list the same files in the same order; when
		// they disagree, nothing is guessed, since a dropped section must be
		// the file the list names.
		return "", nil, errRangeSplit
	}
	order := make([]int, 0, len(changed))
	total := 0
	for i := range changed {
		total += len(sections[i])
		if changed[i].Why == "" && anyFile {
			changed[i].Why = "large"
		}
		if changed[i].Why != "" {
			order = append(order, i)
		}
	}
	// Lockfiles and generated files go first, then the large, each largest
	// first.
	tier := func(i int) int {
		if changed[i].Why == "large" {
			return 1
		}
		return 0
	}
	slices.SortStableFunc(order, func(a, b int) int {
		return cmp.Or(cmp.Compare(tier(a), tier(b)), cmp.Compare(len(sections[b]), len(sections[a])))
	})
	out := map[int]bool{}
	for _, i := range order {
		if total <= maxSummaryPatch {
			break
		}
		out[i], total = true, total-len(sections[i])
	}
	if total > maxSummaryPatch {
		return "", nil, errSummaryLarge
	}
	var b strings.Builder
	var left []LeftOut
	for i := range changed {
		if out[i] {
			left = append(left, changed[i])
		} else {
			b.WriteString(sections[i])
		}
	}
	return b.String(), left, nil
}

// splitPatch cuts a patch into each file's section. A section opens with a
// line starting "diff --git "; inside a section every line starts with a
// space, +, -, @, \, a header word, or base85, so no other line can. A path
// whose type changes, such as a file becoming a symlink, gets two sections
// under one header, a deletion and a creation; with renames off only that
// can repeat a header, so the two are kept as one, as numstat counts them.
func splitPatch(patch string) []string {
	var out []string
	var header string
	start := 0
	for i := 0; i < len(patch); {
		end := strings.IndexByte(patch[i:], '\n')
		next := len(patch)
		if end >= 0 {
			next = i + end + 1
		}
		if line := patch[i:next]; strings.HasPrefix(line, "diff --git ") {
			if i > start && line != header {
				out = append(out, patch[start:i])
				start = i
			}
			header = line
		}
		i = next
	}
	if start < len(patch) {
		out = append(out, patch[start:])
	}
	return out
}

// sectionsMatch says whether each section is the file changed lists at its
// place. A header git quotes, for a path with unusual bytes, is matched by
// place alone.
func sectionsMatch(sections []string, changed []LeftOut) bool {
	if len(sections) != len(changed) {
		return false
	}
	for i, sec := range sections {
		header, _, _ := strings.Cut(sec, "\n")
		if strings.HasPrefix(header, `diff --git "`) {
			continue
		}
		if header != "diff --git a/"+changed[i].Path+" b/"+changed[i].Path {
			return false
		}
	}
	return true
}

// fitLines keeps the first whole lines of text that fit in limit bytes, and
// says how many it dropped and what of.
func fitLines(text string, limit int, dropped string) string {
	if len(text) <= limit {
		return text
	}
	lines := strings.SplitAfter(text, "\n")
	n, size := 0, 0
	for n < len(lines) && size+len(lines[n]) <= limit {
		size += len(lines[n])
		n++
	}
	rest := 0
	for _, l := range lines[n:] {
		if l != "" {
			rest++
		}
	}
	return strings.Join(lines[:n], "") + fmt.Sprintf("… %d %s\n", rest, dropped)
}

// lockfiles are the names of dependency lockfiles. A file with one of these
// names may be left out of a patch past the cap, and only when the forge base
// already holds its path (changedFiles).
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

// changedFiles lists the files changed between from and head with their
// line counts, giving a Why to those a patch too large to send leaves out
// first: known lockfiles that attrs, the change's forge base, already holds,
// and files its .gitattributes marks linguist-generated or -diff. Reading
// both there means the change cannot hide its own code by naming it a
// lockfile or marking it generated. A git too old to read attributes from a
// commit marks the lockfiles alone.
func (r *Repos) changedFiles(ctx context.Context, gitDir, attrs, from, head string) ([]LeftOut, error) {
	stat, err := r.summaryGit(ctx, gitDir, maxListing, "diff", "--numstat", "-z", "--no-renames", "--no-ext-diff", "--no-textconv", from, head, "--")
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
	// Paths go on stdin, however many there are. A path holding a newline
	// cannot be asked about one per line, and is not counted a lockfile.
	named = slices.DeleteFunc(named, func(p string) bool { return strings.ContainsAny(p, "\n\r") })
	if len(named) > 0 {
		var in strings.Builder
		for _, p := range named {
			in.WriteString(attrs + ":" + p + "\n")
		}
		have, err := r.summaryGitIn(ctx, gitDir, maxListing, in.String(), "cat-file", "--batch-check=%(objecttype)")
		if err != nil {
			return nil, err
		}
		answers := strings.Split(strings.TrimSuffix(have, "\n"), "\n")
		if len(answers) != len(named) {
			return nil, errors.New("review: the exact summary range could not be read")
		}
		for i, p := range named {
			if answers[i] == "blob" {
				why[p] = "lockfile"
			}
		}
	}
	if len(paths) > 0 {
		marks, err := r.summaryGitIn(ctx, gitDir, maxListing, strings.Join(paths, "\x00")+"\x00", "check-attr", "--source="+attrs, "--stdin", "-z", "linguist-generated", "diff")
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
