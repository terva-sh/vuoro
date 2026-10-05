package review

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/terva-sh/vuoro/agent"
	"github.com/terva-sh/vuoro/review/drafts"
)

func TestTheEditorFixesKeptFindingsInARepositoryOfItsOwn(t *testing.T) {
	m, d, ch, src := setup(t, func(call toolCaller, text string) string {
		call("propose_draft", map[string]any{"severity": "high", "title": "Add subtracts", "description": "d",
			"anchor": map[string]any{"path": "calc.go", "start_line": 3}})
		call("propose_draft", map[string]any{"severity": "low", "title": "Name it", "description": "d",
			"anchor": map[string]any{"path": "calc.go", "start_line": 3}})
		return "done"
	})
	var message string
	d.edit = func(spec agent.Spec, text string) string {
		message = text
		os.WriteFile(filepath.Join(spec.Dir, "calc.go"), []byte("package calc\n\nfunc Add(a, b int) int { return a + b }\n"), 0o644)
		for _, args := range [][]string{{"add", "calc.go"}, {"commit", "-q", "-m", "Add adds", "-m", "Vuoro-Finding: d1"}} {
			cmd := exec.Command("git", args...)
			cmd.Dir, cmd.Env = spec.Dir, append([]string{"PATH=" + os.Getenv("PATH"), "HOME=" + spec.Dir}, spec.Editor.Env...)
			if out, err := cmd.CombinedOutput(); err != nil {
				return "git: " + string(out)
			}
		}
		return "d1: fixed"
	}
	r, err := m.Start(ch, src, []Reviewer{{Profile: "code", Tool: "fake", Model: "m1"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	wait(t, r, idle)
	who := Identity{Name: "Pat", Email: "pat@example.com"}
	if _, err := m.StartEditor(context.Background(), r.ID, ch.Diff.Head, who); err == nil {
		t.Fatal("an editor started with nothing kept")
	}
	m.Decide(r.ID, "d1", drafts.Kept, "41", false)
	if _, err := m.StartEditor(context.Background(), r.ID, "0123456789abcdef", who); err == nil {
		t.Fatal("an editor started on a change that moved")
	}
	if _, err := m.StartEditor(context.Background(), r.ID, ch.Diff.Head, Identity{}); err == nil {
		t.Fatal("an editor started with no git identity")
	}
	ed, err := m.StartEditor(context.Background(), r.ID, ch.Diff.Head, who)
	if err != nil {
		t.Fatal(err)
	}
	if ed.Branch != "vuoro/fix-7-"+r.ID[len(r.ID)-6:] || ed.Base != ch.Diff.Head {
		t.Fatalf("editor %+v", ed)
	}
	if _, err := m.StartEditor(context.Background(), r.ID, ch.Diff.Head, who); !errors.Is(err, ErrEditorBusy) {
		t.Fatalf("a second editor: %v", err)
	}
	v := wait(t, r, func(v RunView) bool { return idle(v) && len(v.Sessions) == 2 && v.Sessions[1].FirstComplete })
	if e := v.Sessions[1]; e.Role != "editor" || e.Model != "m1" || !v.ReviewComplete {
		t.Fatalf("editor session %+v", e)
	}
	if !strings.Contains(message, "Add subtracts") || strings.Contains(message, "Name it") || !strings.Contains(message, "Finding 1 is d1") {
		t.Fatalf("the editor was sent %q", message)
	}

	d.mu.Lock()
	spec := d.specs[len(d.specs)-1]
	d.mu.Unlock()
	switch {
	case spec.Drafts != nil:
		t.Fatal("the editor was given the drafts server")
	case spec.Dir == r.dir || !strings.HasPrefix(spec.Dir, filepath.Join(m.StateDir, "editor", r.ID)):
		t.Fatalf("the editor works in %s", spec.Dir)
	case !slices.Equal(spec.Editor.Domains, DefaultEditorDomains):
		t.Fatalf("domains %q", spec.Editor.Domains)
	case !slices.Contains(spec.Editor.Env, "GIT_CONFIG_GLOBAL="+os.DevNull) || !slices.Contains(spec.Editor.Env, "GIT_AUTHOR_EMAIL=pat@example.com"):
		t.Fatalf("env %q", spec.Editor.Env)
	case !strings.Contains(spec.SystemPrompt, "Vuoro-Finding"):
		t.Fatal("the editor's prompt names no trailer")
	}
	// The editor's repository borrows nothing from vuoro's clone.
	if _, err := os.Stat(filepath.Join(spec.Dir, ".git", "objects", "info", "alternates")); err == nil {
		t.Fatal("the editor's repository borrows the clone's objects")
	}

	detail, err := m.Editor(context.Background(), r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Commits) != 1 || detail.Commits[0].Subject != "Add adds" || !slices.Equal(detail.Commits[0].Findings, []string{"d1"}) {
		t.Fatalf("commits %+v", detail.Commits)
	}
	if len(detail.Files) != 1 || !strings.Contains(detail.Files[0].Patch, "+func Add(a, b int) int { return a + b }") {
		t.Fatalf("files %+v", detail.Files)
	}

	m2 := &Manager{StateDir: m.StateDir, Repos: m.Repos, Drivers: m.Drivers, Drafts: m.Drafts, MCPURL: m.MCPURL}
	if err := m2.Load(); err != nil {
		t.Fatal(err)
	}
	if back := m2.Get(r.ID); back.View().Editor == nil || back.View().Sessions[1].Role != "editor" || back.who != who {
		t.Fatalf("reloaded %+v", back.View())
	}

	if err := m.DiscardEditor(context.Background(), r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(m.StateDir, "editor", r.ID)); !os.IsNotExist(err) {
		t.Fatalf("the editor's directory outlived its discard: %v", err)
	}
	if err := m.Say(r.ID, v.Sessions[1].ID, "carry on"); err == nil {
		t.Fatal("a discarded editor was resumed")
	}
	if _, err := m.StartEditor(context.Background(), r.ID, ch.Diff.Head, who); err != nil {
		t.Fatalf("an editor after a discard: %v", err)
	}
	wait(t, r, func(v RunView) bool { return idle(v) && len(v.Sessions) == 3 && v.Sessions[2].FirstComplete })
	m.Close(context.Background(), r.ID)
	if _, err := os.Stat(filepath.Join(m.StateDir, "editor", r.ID)); !os.IsNotExist(err) {
		t.Fatalf("the editor's directory outlived its review: %v", err)
	}

	// A closed review can still be fixed, and its editor talked to: the
	// editor needs the cache, not the run's worktree, which is gone.
	if _, err := os.Stat(r.dir); !os.IsNotExist(err) {
		t.Fatalf("the closed run's worktree is still there: %v", err)
	}
	if _, err := m.StartEditor(context.Background(), r.ID, ch.Diff.Head, who); err != nil {
		t.Fatalf("an editor on a closed run: %v", err)
	}
	v = wait(t, r, func(v RunView) bool {
		return len(v.Sessions) == 4 && v.Sessions[3].FirstComplete && v.Sessions[3].Status == Idle
	})
	if v.Status != Closed {
		t.Fatalf("the editor reopened the run: %s", v.Status)
	}
	if err := m.Say(r.ID, v.Sessions[3].ID, "and the rest"); err != nil {
		t.Fatalf("talking to a closed run's editor: %v", err)
	}
	if err := m.Say(r.ID, v.Sessions[0].ID, "hello"); err == nil {
		t.Fatal("a closed run's reviewer was resumed")
	}
	wait(t, r, func(v RunView) bool { return v.Sessions[3].Status == Idle && len(v.Sessions[3].Log) > 3 })
	if detail, err := m.Editor(context.Background(), r.ID); err != nil || len(detail.Commits) == 0 {
		t.Fatalf("a closed run's editor: %+v %v", detail, err)
	}
	if err := m.DiscardEditor(context.Background(), r.ID); err != nil {
		t.Fatal(err)
	}
}

// A head the clone fetched but never checked out lacks its files, and the
// editor's shallow seed needs them, so they are brought in first.
func TestTheEditorSeedsAHeadWhoseFilesTheCloneLacks(t *testing.T) {
	dir, base, head := remote(t)
	git := func(args ...string) string {
		out, err := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("config", "uploadpack.allowFilter", "true")
	state := t.TempDir()
	repos := &Repos{Dir: filepath.Join(state, "repos")}
	src := Source{Remote: "file://" + dir, Key: "example/g/calc", Head: head, Base: base,
		Refspecs: []string{"+refs/heads/feature:refs/vuoro/head", "+refs/heads/main:refs/vuoro/target"}}
	ctx := context.Background()
	if err := repos.Checkout(ctx, src, filepath.Join(state, "wt")); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "more.go"), []byte("package calc\n\nconst More = 1\n"), 0o644)
	git("add", ".")
	git("commit", "-q", "-m", "more")
	src.Head = git("rev-parse", "HEAD")
	gitDir, _ := repos.gitDir(src.Key)
	if err := repos.fetch(ctx, gitDir, src); err != nil {
		t.Fatal(err)
	}
	missing, err := repos.git(ctx, gitDir, "rev-list", "--objects", "--missing=print", "--no-object-names", src.Head+"^{tree}")
	if err != nil || !strings.Contains(missing, "?") {
		t.Fatalf("the new head's files are already here, so this test proves nothing: %q %v", missing, err)
	}
	wt := filepath.Join(state, "editor", "wt")
	if err := repos.SeedEditor(ctx, src.Key, "run1", src.Head, "vuoro/fix-7-run1", wt); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(wt, "more.go")); err != nil || !strings.Contains(string(b), "More") {
		t.Fatalf("the editor's checkout: %q %v", b, err)
	}
	if err := repos.SeedEditor(ctx, src.Key, "run2", strings.Repeat("ab", 20), "b", filepath.Join(state, "e2")); err == nil || !strings.Contains(err.Error(), "no longer holds") {
		t.Fatalf("a head the clone never had: %v", err)
	}
}
