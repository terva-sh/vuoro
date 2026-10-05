package review

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/terva-sh/vuoro/forge/patch"
)

// localRepo is a checkout with main, and a feature branch two commits ahead
// that edits, adds, renames, deletes, and adds a binary file.
func localRepo(t *testing.T) (dir string, git func(...string) string) {
	t.Helper()
	dir = t.TempDir()
	git = func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q", "-b", "main")
	write("a.go", "package a\n\nfunc A() int { return 1 }\n")
	write("old name.txt", strings.Repeat("a line that stays the same\n", 8))
	write("gone.txt", "bye\n")
	git("add", ".")
	git("commit", "-q", "-m", "base")
	git("switch", "-q", "-c", "feature")
	write("a.go", "package a\n\nfunc A() int { return 2 }\n")
	write("new.go", "package a\n\nvar New = true\n")
	git("add", ".")
	git("commit", "-q", "-m", "Change A", "-m", "It returns two now.")
	git("mv", "old name.txt", "new name.txt")
	git("rm", "-q", "gone.txt")
	write("blob.bin", "\x00\x01\x02binary")
	git("add", ".")
	git("commit", "-q", "-m", "Rename, delete, and add a binary")
	return dir, git
}

func TestLocalChangeReadsARange(t *testing.T) {
	dir, git := localRepo(t)
	lt, err := LocalChange(context.Background(), "", dir, "main..feature")
	if err != nil {
		t.Fatal(err)
	}
	ch := lt.Change
	if ch.Kind != Local || ch.Ref.Host != LocalHost || !strings.HasPrefix(ch.Ref.Project, nameUnsafe.ReplaceAllString(filepath.Base(dir), "-")+"-") {
		t.Fatalf("ref %+v kind %s", ch.Ref, ch.Kind)
	}
	if ch.Diff.Head != git("rev-parse", "feature") || ch.Diff.Base != git("rev-parse", "main") {
		t.Fatalf("diff refs %+v", ch.Diff)
	}
	if ch.Title != "Rename, delete, and add a binary" || !strings.Contains(ch.Description, "It returns two now.") {
		t.Fatalf("title %q description %q", ch.Title, ch.Description)
	}
	got := map[string]string{}
	for _, f := range ch.Files {
		switch {
		case f.Omitted:
			got[f.NewPath] = "omitted"
		case f.Renamed:
			got[f.NewPath] = "renamed from " + f.OldPath
		case f.New:
			got[f.NewPath] = "new"
		case f.Deleted:
			got[f.NewPath] = "deleted"
		default:
			got[f.NewPath] = "changed"
		}
	}
	want := map[string]string{"a.go": "changed", "new.go": "new", "new name.txt": "renamed from old name.txt", "gone.txt": "deleted", "blob.bin": "omitted"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: %q, want %q (all %v)", k, got[k], v, got)
		}
	}
	for _, f := range ch.Files {
		if f.NewPath == "a.go" {
			old, cur, ok := patch.Line(f.Patch, true, 3)
			if !ok || old != 0 || cur != 3 {
				t.Errorf("a.go line 3 is not an added line in %q", f.Patch)
			}
			if !strings.HasPrefix(f.Patch, "@@ ") {
				t.Errorf("patch keeps its header: %q", f.Patch)
			}
		}
	}
	if lt.Source.Remote != git("rev-parse", "--show-toplevel") || lt.Source.Key != LocalHost+"/"+ch.Ref.Project || lt.Source.Head != ch.Diff.Head {
		t.Fatalf("source %+v", lt.Source)
	}
	if lt.Dirty {
		t.Fatal("a clean checkout reads as dirty")
	}
}

func TestLocalChangeRangeForms(t *testing.T) {
	dir, git := localRepo(t)
	main, feature := git("rev-parse", "main"), git("rev-parse", "feature")
	// Main moves on after the branch, so the merge base is not main's head.
	git("switch", "-q", "main")
	os.WriteFile(filepath.Join(dir, "later.txt"), []byte("later\n"), 0o644)
	git("add", ".")
	git("commit", "-q", "-m", "later on main")
	git("switch", "-q", "feature")
	for _, target := range []string{"main..feature", "main...feature", "main", "main..", "main...HEAD"} {
		lt, err := LocalChange(context.Background(), "", dir, target)
		if err != nil {
			t.Fatalf("%s: %v", target, err)
		}
		if lt.Change.Diff.Base != main || lt.Change.Diff.Head != feature {
			t.Errorf("%s: base %s head %s", target, lt.Change.Diff.Base, lt.Change.Diff.Head)
		}
		for _, f := range lt.Change.Files {
			if f.NewPath == "later.txt" {
				t.Errorf("%s reviews main's own later commit", target)
			}
		}
	}

	// With no target, the upstream is the base.
	git("branch", "-q", "--set-upstream-to=main")
	lt, err := LocalChange(context.Background(), "", dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if lt.Change.Diff.Base != main || lt.Change.TargetBranch != "main" {
		t.Fatalf("default range: base %s from %q", lt.Change.Diff.Base, lt.Change.TargetBranch)
	}
}

func TestLocalChangeRefusals(t *testing.T) {
	dir, _ := localRepo(t)
	for target, want := range map[string]string{
		"--output=/tmp/x":   "is not a revision or a range",
		"main..--exec=evil": "is not a revision or a range",
		"nope..feature":     "is not a commit",
		"main..a..b":        "is not a revision or a range",
	} {
		if _, err := LocalChange(context.Background(), "", dir, target); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: %v, want %q", target, err, want)
		}
	}
	if _, err := LocalChange(context.Background(), "", dir, "feature..main"); !errors.Is(err, ErrNothingToReview) {
		t.Errorf("an empty range: %v", err)
	}
	if _, err := LocalChange(context.Background(), "", t.TempDir(), "main"); err == nil || !strings.Contains(err.Error(), "not a git checkout") {
		t.Errorf("outside a checkout: %v", err)
	}
	// No upstream and no origin: the person is asked to name a range.
	if _, err := LocalChange(context.Background(), "", dir, ""); err == nil || !strings.Contains(err.Error(), "name a range") {
		t.Errorf("no upstream: %v", err)
	}

	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a // edited\n"), 0o644)
	lt, err := LocalChange(context.Background(), "", dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	if !lt.Dirty {
		t.Fatal("an uncommitted edit is not reported")
	}
}

func TestLocalChangeChecksOutThroughRepos(t *testing.T) {
	dir, _ := localRepo(t)
	lt, err := LocalChange(context.Background(), "", dir, "main..feature")
	if err != nil {
		t.Fatal(err)
	}
	state := t.TempDir()
	r := &Repos{Dir: filepath.Join(state, "repos")}
	wt := filepath.Join(state, "worktrees", "r1")
	if err := r.Checkout(context.Background(), lt.Source, wt); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(wt, "new name.txt")); err != nil || !strings.Contains(string(b), "stays the same") {
		t.Fatalf("the worktree is not at the head: %v", err)
	}
}
