package review

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
)

// The git work of an update (docs/plan.md section 6.10, D66). It runs in
// the run's own worktree and in vuoro's clone, never in the editor's
// repository.

// updateRef is where an update keeps a commit it made or read, so the clone
// holds it until the update is done.
func updateRef(runID, name string) string { return "refs/vuoro/update/" + runID + "/" + name }

// gitIn runs one git command in the worktree at dir, with env added. It
// takes the same care as git: no hook, no prompt, no detached maintenance.
func (r *Repos) gitIn(ctx context.Context, dir string, env []string, args ...string) (string, error) {
	bin := r.Git
	if bin == "" {
		bin = "git"
	}
	ctx, cancel := context.WithTimeout(ctx, gitTimeout)
	defer cancel()
	full := []string{"-C", dir, "-c", "core.hooksPath=" + os.DevNull, "-c", "core.fsmonitor=false", "-c", "advice.detachedHead=false",
		"-c", "maintenance.autoDetach=false", "-c", "gc.autoDetach=false", "-c", "merge.conflictStyle=diff3", "-c", "rerere.enabled=false"}
	cmd := exec.CommandContext(ctx, bin, append(full, args...)...)
	cmd.Env = append(append(os.Environ(), "GIT_TERMINAL_PROMPT=0"), env...)
	if os.Getenv("GIT_SSH_COMMAND") == "" {
		cmd.Env = append(cmd.Env, "GIT_SSH_COMMAND=ssh -o BatchMode=yes")
	}
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return out.String(), fmt.Errorf("review: git %s: %s: %w", args[0], lastLine(errb.String(), err), err)
	}
	return out.String(), nil
}

// identityEnv makes who the author and committer of a commit vuoro writes.
func identityEnv(who Identity) []string {
	return []string{"GIT_AUTHOR_NAME=" + who.Name, "GIT_AUTHOR_EMAIL=" + who.Email, "GIT_COMMITTER_NAME=" + who.Name, "GIT_COMMITTER_EMAIL=" + who.Email}
}

// step is one attempt to bring changes into the worktree: a merge of the
// target, or the replay of one commit.
type step struct {
	// result is the commit written when nothing conflicted.
	result string
	// seed is the commit holding the conflicted tree, markers included, and
	// conflicted the paths git could not merge, when something did.
	seed       string
	conflicted []string
}

// UpdateTarget is the commit the target branch's ref holds after the run's
// fetch.
func (r *Repos) UpdateTarget(ctx context.Context, key, branch string) (string, error) {
	gitDir, err := r.gitDir(key)
	if err != nil {
		return "", err
	}
	out, err := r.git(ctx, gitDir, "rev-parse", "--verify", "refs/vuoro/target/"+branch+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("review: the target branch %s did not arrive with the fetch", branch)
	}
	return strings.TrimSpace(out), nil
}

// isAncestor says a is an ancestor of b, or b itself, in key's clone.
func (r *Repos) isAncestor(ctx context.Context, key, a, b string) (bool, error) {
	gitDir, err := r.gitDir(key)
	if err != nil {
		return false, err
	}
	_, err = r.git(ctx, gitDir, "merge-base", "--is-ancestor", a, b)
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return false, nil
	}
	return err == nil, err
}

// mergeStep merges target into onto in the worktree at dir. Nothing
// conflicting, it writes the merge commit, with onto and target as parents
// and who as author and committer. Something conflicting, it writes the
// seed: the conflicted tree as one commit on onto. Either way the worktree
// is put back at onto.
func (r *Repos) mergeStep(ctx context.Context, key, runID, dir string, who Identity, onto, target, message string) (step, error) {
	unlock, err := r.lockKey(ctx, key)
	if err != nil {
		return step{}, err
	}
	defer unlock()
	defer r.restore(ctx, dir, onto)
	if err := r.restore(ctx, dir, onto); err != nil {
		return step{}, err
	}
	_, err = r.gitIn(ctx, dir, nil, "merge", "--no-commit", "--no-ff", "--no-edit", target)
	return r.finish(ctx, key, runID, dir, who, err, []string{onto, target}, message, "merge")
}

// pickStep replays commit onto onto in the worktree at dir, keeping its
// author and message, with who as committer.
func (r *Repos) pickStep(ctx context.Context, key, runID, dir string, who Identity, onto, commit string) (step, error) {
	unlock, err := r.lockKey(ctx, key)
	if err != nil {
		return step{}, err
	}
	defer unlock()
	defer r.restore(ctx, dir, onto)
	if err := r.restore(ctx, dir, onto); err != nil {
		return step{}, err
	}
	message, env, err := r.authorship(ctx, key, commit)
	if err != nil {
		return step{}, err
	}
	_, err = r.gitIn(ctx, dir, nil, "cherry-pick", "--no-commit", commit)
	return r.finishWith(ctx, key, runID, dir, append(env, committerEnv(who)...), err, []string{onto}, message, "pick")
}

// finish turns the worktree's state after a merge or a replay into a step.
func (r *Repos) finish(ctx context.Context, key, runID, dir string, who Identity, mergeErr error, parents []string, message, what string) (step, error) {
	return r.finishWith(ctx, key, runID, dir, identityEnv(who), mergeErr, parents, message, what)
}

func (r *Repos) finishWith(ctx context.Context, key, runID, dir string, env []string, mergeErr error, parents []string, message, what string) (step, error) {
	out, err := r.gitIn(ctx, dir, nil, "diff", "--name-only", "-z", "--diff-filter=U")
	if err != nil {
		return step{}, err
	}
	var conflicted []string
	for _, p := range strings.Split(out, "\x00") {
		if p != "" {
			conflicted = append(conflicted, p)
		}
	}
	if mergeErr != nil && len(conflicted) == 0 {
		return step{}, mergeErr
	}
	if len(conflicted) > 0 {
		// The conflicted result, markers and all, becomes the seed the
		// editor starts from. It has one parent, so the editor sees what
		// came in as a diff, and it is never pushed.
		if _, err := r.gitIn(ctx, dir, nil, "add", "-A"); err != nil {
			return step{}, err
		}
		parents = parents[:1]
		message = "vuoro: " + what + " with conflicts to resolve\n\nThis commit holds conflict markers. It is the editor's starting point and is never pushed."
	}
	tree, err := r.gitIn(ctx, dir, nil, "write-tree")
	if err != nil {
		return step{}, err
	}
	if what == "pick" && len(conflicted) == 0 {
		// A replayed commit the target already holds changes nothing, and
		// is dropped, as git rebase drops it.
		if was, err := r.gitIn(ctx, dir, nil, "rev-parse", parents[0]+"^{tree}"); err == nil && strings.TrimSpace(was) == strings.TrimSpace(tree) {
			return step{result: parents[0]}, nil
		}
	}
	sha, err := r.commitTree(ctx, dir, env, strings.TrimSpace(tree), parents, message)
	if err != nil {
		return step{}, err
	}
	name := "result-" + sha[:12]
	if len(conflicted) > 0 {
		name = "seed-" + sha[:12]
	}
	if _, err := r.gitIn(ctx, dir, nil, "update-ref", updateRef(runID, name), sha); err != nil {
		return step{}, err
	}
	if len(conflicted) > 0 {
		return step{seed: sha, conflicted: conflicted}, nil
	}
	return step{result: sha}, nil
}

// commitTree writes a commit of tree with parents and message, with env
// naming its author and committer.
func (r *Repos) commitTree(ctx context.Context, dir string, env []string, tree string, parents []string, message string) (string, error) {
	args := []string{"commit-tree", tree, "-m", message}
	for _, p := range parents {
		args = append(args, "-p", p)
	}
	out, err := r.gitIn(ctx, dir, env, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// restore puts the worktree back at onto, clean, after a merge or replay.
func (r *Repos) restore(ctx context.Context, dir, onto string) error {
	_, _ = r.gitIn(ctx, dir, nil, "merge", "--abort")
	_, _ = r.gitIn(ctx, dir, nil, "cherry-pick", "--abort")
	if _, err := r.gitIn(ctx, dir, nil, "reset", "--quiet", "--hard", onto); err != nil {
		return err
	}
	_, err := r.gitIn(ctx, dir, nil, "clean", "-fdq")
	return err
}

// authorship is commit's message, and the environment that keeps its author
// and author date on a replay.
func (r *Repos) authorship(ctx context.Context, key, commit string) (string, []string, error) {
	gitDir, err := r.gitDir(key)
	if err != nil {
		return "", nil, err
	}
	out, err := r.git(ctx, gitDir, "log", "-1", "--format=%an%x00%ae%x00%aI%x00%B", commit)
	if err != nil {
		return "", nil, err
	}
	f := strings.SplitN(out, "\x00", 4)
	if len(f) != 4 {
		return "", nil, fmt.Errorf("review: could not read %s", short(commit))
	}
	return strings.TrimRight(f[3], "\n"), []string{"GIT_AUTHOR_NAME=" + f[0], "GIT_AUTHOR_EMAIL=" + f[1], "GIT_AUTHOR_DATE=" + f[2]}, nil
}

// committerEnv makes who the committer of a replayed commit.
func committerEnv(who Identity) []string {
	return []string{"GIT_COMMITTER_NAME=" + who.Name, "GIT_COMMITTER_EMAIL=" + who.Email}
}

// lockKey takes the clone's lock: an update's worktree shares the clone's
// objects and refs with every run's fetch.
func (r *Repos) lockKey(ctx context.Context, key string) (func(), error) {
	gitDir, err := r.gitDir(key)
	if err != nil {
		return nil, err
	}
	return r.lock(ctx, gitDir)
}

// markerLine says a line is one git writes around a conflict.
func markerLine(line string) bool {
	for _, m := range []string{"<<<<<<<", "|||||||", ">>>>>>>"} {
		if line == m || strings.HasPrefix(line, m+" ") {
			return true
		}
	}
	return false
}

// maxResolvedFile bounds a conflicted file read back to look for markers.
const maxResolvedFile = 16 << 20

// resolved checks the editor's work on a stop, as FetchEditor brought it
// into the clone at tip: it stands on the seed, and no file that conflicted
// still holds a marker line. It returns tip's tree, and the files the editor
// changed beyond those that conflicted.
func (r *Repos) resolved(ctx context.Context, key, runID, seed, tip string, conflicted []string) (string, []string, error) {
	gitDir, err := r.gitDir(key)
	if err != nil {
		return "", nil, err
	}
	unlock, err := r.lock(ctx, gitDir)
	if err != nil {
		return "", nil, err
	}
	defer unlock()
	if _, err := r.git(ctx, gitDir, "merge-base", "--is-ancestor", seed, tip); err != nil {
		return "", nil, errors.New("review: the editor's branch no longer stands on the conflicted merge it was given; it must not rewrite history")
	}
	var marked []string
	for _, p := range conflicted {
		if _, err := r.git(ctx, gitDir, "cat-file", "-e", tip+":"+p); err != nil {
			continue // resolved by removing the file
		}
		size, err := r.git(ctx, gitDir, "cat-file", "-s", tip+":"+p)
		if err != nil {
			return "", nil, err
		}
		if n, err := strconv.Atoi(strings.TrimSpace(size)); err != nil || n > maxResolvedFile {
			return "", nil, fmt.Errorf("review: %s is too large to check for conflict markers", p)
		}
		body, err := r.git(ctx, gitDir, "cat-file", "blob", tip+":"+p)
		if err != nil {
			return "", nil, err
		}
		sc := bufio.NewScanner(strings.NewReader(body))
		sc.Buffer(make([]byte, 64<<10), maxResolvedFile)
		for sc.Scan() {
			if markerLine(strings.TrimRight(sc.Text(), "\r")) {
				marked = append(marked, p)
				break
			}
		}
	}
	if len(marked) > 0 {
		return "", nil, fmt.Errorf("review: conflict markers are still in %s; ask the editor to finish", strings.Join(marked, ", "))
	}
	out, err := r.git(ctx, gitDir, "diff", "--name-only", "-z", seed, tip)
	if err != nil {
		return "", nil, err
	}
	outside := []string{}
	for _, p := range strings.Split(out, "\x00") {
		if p != "" && !slices.Contains(conflicted, p) {
			outside = append(outside, p)
		}
	}
	tree, err := r.git(ctx, gitDir, "rev-parse", tip+"^{tree}")
	if err != nil {
		return "", nil, err
	}
	if _, err := r.git(ctx, gitDir, "update-ref", updateRef(runID, "tip-"+tip[:12]), tip); err != nil {
		return "", nil, err
	}
	return strings.TrimSpace(tree), outside, nil
}

// commitIn writes a commit in key's clone.
func (r *Repos) commitIn(ctx context.Context, key, runID string, env []string, tree string, parents []string, message string) (string, error) {
	gitDir, err := r.gitDir(key)
	if err != nil {
		return "", err
	}
	unlock, err := r.lock(ctx, gitDir)
	if err != nil {
		return "", err
	}
	defer unlock()
	sha, err := r.commitTree(ctx, gitDir, env, tree, parents, message)
	if err != nil {
		return "", err
	}
	if _, err := r.git(ctx, gitDir, "update-ref", updateRef(runID, "result-"+sha[:12]), sha); err != nil {
		return "", err
	}
	return sha, nil
}

// subjects are the subjects of the commits in rng, newest first, at most n.
func (r *Repos) subjects(ctx context.Context, key, rng string, n int) ([]string, int, error) {
	gitDir, err := r.gitDir(key)
	if err != nil {
		return nil, 0, err
	}
	count, err := r.git(ctx, gitDir, "rev-list", "--count", rng)
	if err != nil {
		return nil, 0, err
	}
	out, err := r.git(ctx, gitDir, "log", "--format=%s", fmt.Sprintf("-n%d", n), rng)
	if err != nil {
		return nil, 0, err
	}
	var total int
	fmt.Sscan(strings.TrimSpace(count), &total)
	lines := []string{}
	for _, l := range strings.Split(strings.TrimSpace(out), "\n") {
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines, total, nil
}

// DropUpdate removes an update's refs from key's clone.
func (r *Repos) DropUpdate(ctx context.Context, key, runID string) error {
	gitDir, err := r.gitDir(key)
	if err != nil {
		return err
	}
	unlock, err := r.lock(ctx, gitDir)
	if err != nil {
		return err
	}
	defer unlock()
	out, err := r.git(ctx, gitDir, "for-each-ref", "--format=%(refname)", "refs/vuoro/update/"+runID+"/")
	if err != nil {
		return err
	}
	for _, ref := range strings.Fields(out) {
		_, _ = r.git(ctx, gitDir, "update-ref", "-d", ref)
	}
	return nil
}

// PushUpdate pushes an update's result, sha, to branch at remote, over the
// person's own git transport. With lease empty it never forces, so a branch
// that moved is refused. With lease set, for an accepted rebase (D67), it
// forces with a lease against that commit, the head the person saw, so a
// push that landed since is refused rather than overwritten.
func (r *Repos) PushUpdate(ctx context.Context, key, remote, sha, branch, lease string) error {
	gitDir, err := r.gitDir(key)
	if err != nil {
		return err
	}
	unlock, err := r.lock(ctx, gitDir)
	if err != nil {
		return err
	}
	defer unlock()
	args := []string{"push", "--quiet", "--no-verify"}
	if lease != "" {
		args = append(args, "--force-with-lease=refs/heads/"+branch+":"+lease)
	}
	_, err = r.git(ctx, gitDir, append(args, remote, sha+":refs/heads/"+branch)...)
	return err
}
