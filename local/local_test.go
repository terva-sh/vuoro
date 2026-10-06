package local

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// repo is a checkout at dir on main with one commit, and a git helper.
func repo(t *testing.T, dir string) func(...string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "-b", "main")
	git("commit", "-q", "--allow-empty", "-m", "base")
	return git
}

// same says whether two paths name one folder, however each is spelled:
// git reports a resolved path, and on Windows a long one.
func same(a, b string) bool {
	x, err1 := os.Stat(a)
	y, err2 := os.Stat(b)
	return err1 == nil && err2 == nil && os.SameFile(x, y)
}

func TestAStoreKeepsItsListsOnceAndReadsBackEmptyAsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "local.json")
	s, err := LoadStore(path)
	if err != nil || s.Projects == nil || s.Roots == nil || s.Hidden == nil || s.Bases == nil {
		t.Fatalf("a missing file should be an empty store with no nil lists: %+v %v", s, err)
	}
	s.AddProject("/a")
	s.AddProject("/a")
	s.AddRoot("/w")
	s.Hide("/a/.git")
	s.SetBase("/a/.git", "develop")
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	back, _ := LoadStore(path)
	if !slices.Equal(back.Projects, []string{"/a"}) || !slices.Equal(back.Roots, []string{"/w"}) || back.Bases["/a/.git"] != "develop" {
		t.Fatalf("read back %+v", back)
	}
	back.RemoveProject("/a")
	back.Unhide("/a/.git")
	back.SetBase("/a/.git", "")
	if len(back.Projects) != 0 || len(back.Hidden) != 0 || len(back.Bases) != 0 {
		t.Fatalf("after removing: %+v", back)
	}
}

func TestCleanPathTakesOnlyAnAbsoluteFolder(t *testing.T) {
	dir := t.TempDir()
	if p, err := CleanPath(dir + string(filepath.Separator) + "."); err != nil || p != filepath.Clean(dir) {
		t.Fatalf("CleanPath(%s/.) = %q, %v", dir, p, err)
	}
	file := filepath.Join(dir, "f")
	os.WriteFile(file, nil, 0o600)
	for _, bad := range []string{"", "relative/path", file, filepath.Join(dir, "missing")} {
		if _, err := CleanPath(bad); err == nil {
			t.Errorf("CleanPath(%q) was accepted", bad)
		}
	}
}

func TestAScanFindsCheckoutsWithinItsBounds(t *testing.T) {
	root := t.TempDir()
	repo(t, filepath.Join(root, "a"))
	repo(t, filepath.Join(root, "group", "b"))
	repo(t, filepath.Join(root, "a", "nested")) // inside a checkout: not looked for
	repo(t, filepath.Join(root, ".hidden", "c"))
	repo(t, filepath.Join(root, "node_modules", "d"))
	repo(t, filepath.Join(root, "1", "2", "3", "4", "deep"))
	if runtime.GOOS != "windows" {
		os.Symlink(filepath.Join(root, "a"), filepath.Join(root, "link"))
	}
	got := Scan(context.Background(), []string{root}, DefaultLimits)
	want := []string{filepath.Join(root, "a"), filepath.Join(root, "group", "b")}
	slices.Sort(got.Checkouts)
	if !slices.Equal(got.Checkouts, want) || got.Stopped != "" {
		t.Fatalf("Scan found %v (stopped %q), want %v", got.Checkouts, got.Stopped, want)
	}
	few := Scan(context.Background(), []string{root}, Limits{Depth: 4, Checkouts: 1, Time: time.Minute})
	if len(few.Checkouts) != 1 || !strings.Contains(few.Stopped, "1 checkouts") {
		t.Fatalf("a scan past its count: %+v", few)
	}
}

func TestAProjectIsItsRepositoryWithBranchesBeyondTheBaseAndWorktreesAsTags(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "proj")
	git := repo(t, dir)
	git("switch", "-q", "-c", "feature")
	git("commit", "-q", "--allow-empty", "-m", "one")
	git("commit", "-q", "--allow-empty", "-m", "two")
	git("branch", "merged", "main") // no commits beyond main
	git("branch", "vuoro/fix-r1", "feature")
	git("switch", "-q", "main")
	wt := filepath.Join(filepath.Dir(dir), "proj-wt")
	git("worktree", "add", "-q", "-b", "other", wt)
	os.WriteFile(filepath.Join(wt, "tracked"), []byte("x"), 0o600)
	Git{Dir: wt}.Run(ctx, "add", "tracked")
	git("-C", wt, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "wt")
	os.WriteFile(filepath.Join(wt, "tracked"), []byte("changed"), 0o600)

	fromMain, err := Inspect(ctx, "", dir, "")
	if err != nil {
		t.Fatal(err)
	}
	fromWT, err := Inspect(ctx, "", wt, "")
	if err != nil {
		t.Fatal(err)
	}
	if fromMain.Key != fromWT.Key || !same(fromMain.Path, dir) || !same(fromWT.Path, dir) {
		t.Fatalf("a checkout and its worktree are two projects: %q at %q, %q at %q", fromMain.Key, fromMain.Path, fromWT.Key, fromWT.Path)
	}
	if fromMain.Base != "main" || fromMain.Error != "" {
		t.Fatalf("base %q, error %q", fromMain.Base, fromMain.Error)
	}
	names := []string{}
	for _, b := range fromMain.Branches {
		names = append(names, b.Name)
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"feature", "other"}) {
		t.Fatalf("branches %v; merged and vuoro's own branches should not be rows", names)
	}
	for _, b := range fromMain.Branches {
		switch b.Name {
		case "feature":
			if b.Ahead != 2 || len(b.Worktrees) != 0 {
				t.Errorf("feature: %+v", b)
			}
		case "other":
			if b.Ahead != 1 || len(b.Worktrees) != 1 || !same(b.Worktrees[0], wt) {
				t.Errorf("other: %+v", b)
			}
		}
	}
	dirty := map[string]bool{}
	for _, w := range fromMain.Worktrees {
		dirty[w.Branch] = w.Dirty
	}
	if dirty["main"] || !dirty["other"] {
		t.Fatalf("dirty marks %v", dirty)
	}
	if fromMain.Fixes["vuoro/fix-r1"] == "" {
		t.Fatalf("fix branches %v", fromMain.Fixes)
	}

	tip := git("rev-parse", "feature")
	in, inBase, _ := fromMain.Containing(ctx, tip, []string{"feature", "other"})
	if inBase || !slices.Equal(in, []string{"feature"}) {
		t.Fatalf("Containing(feature's tip) = %v, %v", in, inBase)
	}
	if _, inBase, _ := fromMain.Containing(ctx, git("rev-parse", "main"), []string{"feature"}); !inBase {
		t.Fatal("the base's own commit should be in the base")
	}
	if !fromMain.Merged(ctx, "vuoro/fix-r1", "feature") {
		t.Fatal("a fix branch at feature's tip is merged into feature")
	}
}

func TestABaseIsChosenOrFoundOrSaidToBeMissing(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	git := repo(t, dir)
	git("branch", "-m", "trunk")
	git("switch", "-q", "-c", "feature")
	git("commit", "-q", "--allow-empty", "-m", "one")
	p, err := Inspect(ctx, "", dir, "")
	if err != nil || p.Error == "" || len(p.Branches) != 0 {
		t.Fatalf("no main or master should say so: %+v %v", p, err)
	}
	if p, _ = Inspect(ctx, "", dir, "trunk"); p.Error != "" || len(p.Branches) != 1 || p.Branches[0].Ahead != 1 {
		t.Fatalf("a chosen base: %+v", p)
	}
	if p, _ = Inspect(ctx, "", dir, "--output=/tmp/x"); p.Error == "" {
		t.Fatal("an option as a base was taken")
	}
	if _, err := Inspect(ctx, "", t.TempDir(), ""); err == nil {
		t.Fatal("a folder with no checkout was read as a project")
	}
}
