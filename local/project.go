package local

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// maxBranches bounds the branches read from one project.
const maxBranches = 200

// FixPrefix names the branches vuoro's editor writes into the person's
// repository (D42). They are listed under the branch they fix.
const FixPrefix = "vuoro/"

// Worktree is one checkout of a project: the main one or a linked worktree.
type Worktree struct {
	Path   string `json:"path"`
	Branch string `json:"branch,omitempty"` // empty when detached
	Head   string `json:"head"`
	// Dirty says it has uncommitted changes to tracked files, which a
	// review of commits does not see.
	Dirty bool `json:"dirty"`
}

// Branch is a local branch with commits beyond the project's base.
type Branch struct {
	Name      string    `json:"name"`
	Tip       string    `json:"tip"`
	Ahead     int       `json:"ahead"`
	Committed time.Time `json:"committed"`
	Worktrees []string  `json:"worktrees"` // the paths it is checked out in
	// UpstreamRemote and UpstreamBranch are the branch's upstream, as a remote
	// and a branch name on it, both empty when it has none.
	UpstreamRemote string `json:"upstream_remote,omitempty"`
	UpstreamBranch string `json:"upstream_branch,omitempty"`
}

// Project is one repository, however many checkouts it has.
type Project struct {
	// Key is the absolute git common directory, the same for a main
	// checkout and its worktrees.
	Key       string     `json:"key"`
	Name      string     `json:"name"`
	Path      string     `json:"path"` // the main checkout
	Base      string     `json:"base,omitempty"`
	Worktrees []Worktree `json:"worktrees"`
	Branches  []Branch   `json:"branches"`
	// Remotes are the remotes that name a forge project.
	Remotes []Remote `json:"remotes"`
	// Fixes are vuoro's own branches in the repository, by name, with
	// their tips.
	Fixes map[string]string `json:"-"`
	// Error is why the project or its base could not be read.
	Error string `json:"error,omitempty"`

	git Git
}

// Inspect reads the checkout at path. base, when set, is the person's
// choice of base; otherwise it is origin/HEAD, then main, then master. An
// error means path is not a checkout at all. A project whose base cannot be
// found is returned with Error set and no branches.
func Inspect(ctx context.Context, gitBin, path, base string) (*Project, error) {
	g := Git{Bin: gitBin, Dir: path}
	out, err := g.Run(ctx, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return nil, fmt.Errorf("local: %s is not a git checkout", path)
	}
	p := &Project{Key: filepath.Clean(strings.TrimSpace(out)), Worktrees: []Worktree{}, Branches: []Branch{}, Remotes: []Remote{}, Fixes: map[string]string{}}
	if out, err = g.Run(ctx, "worktree", "list", "--porcelain"); err != nil {
		return nil, err
	}
	p.Worktrees = parseWorktrees(out)
	if len(p.Worktrees) > 0 {
		p.Path = p.Worktrees[0].Path
	} else {
		p.Path = path // a bare repository, which has no checkout of its own
	}
	p.git = Git{Bin: gitBin, Dir: p.Path}
	p.Name = filepath.Base(p.Path)
	p.readRemotes(ctx)
	for i := range p.Worktrees {
		w := &p.Worktrees[i]
		st, err := Git{Bin: gitBin, Dir: w.Path}.Run(ctx, "status", "--porcelain", "--untracked-files=no")
		w.Dirty = err == nil && strings.TrimSpace(st) != ""
	}

	if p.Base, err = p.findBase(ctx, base); err != nil {
		p.Error = err.Error()
		return p, nil
	}
	if err := p.readBranches(ctx); err != nil {
		p.Error = err.Error()
	}
	return p, nil
}

// parseWorktrees reads `git worktree list --porcelain`. A bare entry, the
// common directory of a bare repository, is not a checkout and is left out.
func parseWorktrees(out string) []Worktree {
	var all []Worktree
	var w *Worktree
	bare := false
	flush := func() {
		if w != nil && !bare {
			all = append(all, *w)
		}
		w, bare = nil, false
	}
	for _, l := range strings.Split(out, "\n") {
		l = strings.TrimRight(l, "\r")
		switch {
		case strings.HasPrefix(l, "worktree "):
			flush()
			w = &Worktree{Path: filepath.Clean(strings.TrimPrefix(l, "worktree "))}
		case w == nil:
		case strings.HasPrefix(l, "HEAD "):
			w.Head = strings.TrimPrefix(l, "HEAD ")
		case strings.HasPrefix(l, "branch "):
			w.Branch = strings.TrimPrefix(strings.TrimPrefix(l, "branch "), "refs/heads/")
		case l == "bare":
			bare = true
		}
	}
	flush()
	if all == nil {
		all = []Worktree{}
	}
	return all
}

// findBase resolves the project's base to a revision git can read.
func (p *Project) findBase(ctx context.Context, chosen string) (string, error) {
	if chosen != "" {
		if strings.HasPrefix(chosen, "-") || !p.isCommit(ctx, chosen) {
			return "", fmt.Errorf("the base %q is not a commit in this repository", chosen)
		}
		return chosen, nil
	}
	if out, err := p.git.Run(ctx, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD"); err == nil {
		if b := strings.TrimSpace(out); b != "" {
			return b, nil
		}
	}
	for _, b := range []string{"main", "master"} {
		if p.isCommit(ctx, b) {
			return b, nil
		}
	}
	return "", errors.New("no origin/HEAD, main, or master to compare branches with; choose a base")
}

func (p *Project) isCommit(ctx context.Context, rev string) bool {
	_, err := p.git.Run(ctx, "rev-parse", "--verify", "--quiet", "--end-of-options", rev+"^{commit}")
	return err == nil
}

// readBranches lists the local branches not merged into the base, and
// vuoro's own fix branches whether merged or not.
func (p *Project) readBranches(ctx context.Context) error {
	format := "--format=%(refname:short)%00%(objectname)%00%(committerdate:unix)"
	out, err := p.git.Run(ctx, "for-each-ref", format, "refs/heads/"+FixPrefix)
	if err != nil {
		return err
	}
	for _, l := range lines(out) {
		if f := strings.Split(l, "\x00"); len(f) == 3 {
			p.Fixes[f[0]] = f[1]
		}
	}
	// git 2.41 and later count every branch's commits beyond the base in the
	// same call. An older git counts them one branch at a time below.
	format += "%00%(upstream:remotename)%00%(upstream:remoteref)"
	counted := true
	out, err = p.git.Run(ctx, "for-each-ref", "--no-merged="+p.Base, format+"%00%(ahead-behind:"+p.Base+")", "refs/heads/")
	if err != nil {
		counted = false
		if out, err = p.git.Run(ctx, "for-each-ref", "--no-merged="+p.Base, format, "refs/heads/"); err != nil {
			return err
		}
	}
	checked := map[string][]string{}
	for _, w := range p.Worktrees {
		if w.Branch != "" {
			checked[w.Branch] = append(checked[w.Branch], w.Path)
		}
	}
	for _, l := range lines(out) {
		f := strings.Split(l, "\x00")
		if len(f) < 5 || strings.HasPrefix(f[0], FixPrefix) {
			continue
		}
		sec, _ := strconv.ParseInt(f[2], 10, 64)
		wt := checked[f[0]]
		if wt == nil {
			wt = []string{}
		}
		b := Branch{Name: f[0], Tip: f[1], Committed: time.Unix(sec, 0).UTC(), Worktrees: wt, Ahead: -1}
		// A branch whose upstream is another local branch has the remote ".".
		if f[3] != "" && f[3] != "." {
			b.UpstreamRemote, b.UpstreamBranch = f[3], strings.TrimPrefix(f[4], "refs/heads/")
		}
		if counted && len(f) == 6 {
			if ab := strings.Fields(f[5]); len(ab) == 2 {
				b.Ahead, _ = strconv.Atoi(ab[0])
			}
		}
		p.Branches = append(p.Branches, b)
	}
	sort.SliceStable(p.Branches, func(i, j int) bool { return p.Branches[i].Committed.After(p.Branches[j].Committed) })
	if len(p.Branches) > maxBranches {
		p.Branches = p.Branches[:maxBranches]
	}
	for i := range p.Branches {
		b := &p.Branches[i]
		if b.Ahead >= 0 {
			continue
		}
		n, err := p.git.Run(ctx, "rev-list", "--count", p.Base+".."+b.Tip)
		if err != nil {
			return err
		}
		b.Ahead, _ = strconv.Atoi(strings.TrimSpace(n))
	}
	return nil
}

// Containing are the project's branches, among names, whose history holds
// sha, and whether the base's does too.
func (p *Project) Containing(ctx context.Context, sha string, names []string) (branches []string, inBase bool, err error) {
	if p.Base == "" || strings.HasPrefix(sha, "-") {
		return nil, false, nil
	}
	if _, err := p.git.Run(ctx, "merge-base", "--is-ancestor", sha, p.Base); err == nil {
		return nil, true, nil
	}
	refs := make([]string, 0, len(names))
	for _, n := range names {
		refs = append(refs, "refs/heads/"+n)
	}
	if len(refs) == 0 {
		return nil, false, nil
	}
	out, err := p.git.Run(ctx, append([]string{"for-each-ref", "--contains=" + sha, "--format=%(refname:short)"}, refs...)...)
	if err != nil {
		// A commit the repository no longer has is in no branch.
		return nil, false, nil
	}
	return lines(out), false, nil
}

// Merged says whether branch from's history is all in branch into.
func (p *Project) Merged(ctx context.Context, from, into string) bool {
	_, err := p.git.Run(ctx, "merge-base", "--is-ancestor", "refs/heads/"+from, "refs/heads/"+into)
	return err == nil
}
