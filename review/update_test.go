package review

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/terva-sh/vuoro/agent"
	"github.com/terva-sh/vuoro/forge"
)

// diverged is a remote whose main and feature branches both moved on from
// one base. conflict makes main change the line feature changes.
func diverged(t *testing.T, conflict bool) (dir, head, target string) {
	t.Helper()
	dir = t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(name, body string) { os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644) }
	git("init", "-q", "-b", "main")
	write("calc.go", "package calc\n\nfunc Add(a, b int) int { return a + b }\n")
	write("README", "calc\n")
	git("add", ".")
	git("commit", "-q", "-m", "base")
	git("switch", "-q", "-c", "feature")
	write("calc.go", "package calc\n\nfunc Add(a, b int) int { return a - b }\n")
	git("commit", "-q", "-am", "Subtract instead")
	write("calc.go", "package calc\n\nfunc Add(a, b int) int { return a - b }\n\nfunc Sub(a, b int) int { return a - b }\n")
	git("commit", "-q", "-am", "Add Sub")
	head = git("rev-parse", "HEAD")
	git("switch", "-q", "main")
	if conflict {
		write("calc.go", "package calc\n\nfunc Add(a, b int) int { return b + a }\n")
	} else {
		write("README", "calc, a calculator\n")
	}
	git("commit", "-q", "-am", "Main moves on")
	target = git("rev-parse", "HEAD")
	return dir, head, target
}

func updateSetup(t *testing.T, conflict bool) (*Manager, *fakeDriver, *forge.Change, Source, string) {
	t.Helper()
	m, d, _, _ := setup(t, func(toolCaller, string) string { return "not a reviewer" })
	dir, head, target := diverged(t, conflict)
	ch := &forge.Change{Kind: forge.GitLab, Ref: forge.Ref{Host: "gitlab.example", Project: "g/calc", Number: 7}, Title: "Subtract",
		State: forge.Open, SourceBranch: "feature", TargetBranch: "main", Diff: forge.DiffRefs{Head: head, Base: target}}
	src := Source{Remote: dir, Key: "gitlab.example/g/calc", Head: head,
		Refspecs: []string{"+refs/heads/feature:refs/vuoro/head", "+refs/heads/main:refs/vuoro/target/main"}}
	return m, d, ch, src, target
}

var pat = Identity{Name: "Pat", Email: "pat@example.com"}

func cloneGit(t *testing.T, m *Manager, args ...string) string {
	t.Helper()
	gitDir, _ := m.Repos.gitDir("gitlab.example/g/calc")
	out, err := exec.Command("git", append([]string{"--git-dir=" + gitDir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestACleanMergeNeedsNoAgent(t *testing.T) {
	m, d, ch, src, target := updateSetup(t, false)
	r, err := m.StartUpdate(ch, src, UpdateInput{Strategy: Merge, Own: true, Who: pat, Editor: Reviewer{Tool: "fake"}})
	if err != nil {
		t.Fatal(err)
	}
	v := wait(t, r, func(v RunView) bool { return v.Status == Idle || v.Status == Failed })
	if v.Status != Idle || v.Update.State() != "ready" || len(v.Sessions) != 0 || len(d.specs) != 0 {
		t.Fatalf("clean merge %+v %+v", v, v.Update)
	}
	parents := cloneGit(t, m, "log", "-1", "--format=%P%x00%an%x00%s", v.Update.Result)
	if parents != ch.Diff.Head+" "+target+"\x00Pat\x00Merge branch 'main' into feature" {
		t.Fatalf("merge commit %q", parents)
	}
	detail, err := m.UpdateDetail(context.Background(), r.ID)
	if err != nil || len(detail.Commits) != 1 || detail.State != "ready" {
		t.Fatalf("detail %+v %v", detail, err)
	}
}

func TestAConflictedMergeIsResolvedByTheEditorAndBuiltByVuoro(t *testing.T) {
	m, d, ch, src, target := updateSetup(t, true)
	var asked string
	outside := t.TempDir()
	canary := filepath.Join(outside, "ran")
	d.edit = func(spec agent.Spec, text string) string {
		asked = text
		body, _ := os.ReadFile(filepath.Join(spec.Dir, "calc.go"))
		if !strings.Contains(string(body), "<<<<<<<") || !strings.Contains(string(body), "|||||||") {
			return "no diff3 markers"
		}
		resolved := "package calc\n\nfunc Add(a, b int) int { return b - a }\n\nfunc Sub(a, b int) int { return a - b }\n"
		if strings.Contains(text, "half") {
			resolved = "<<<<<<< ours\n" + resolved
		}
		os.WriteFile(filepath.Join(spec.Dir, "calc.go"), []byte(resolved), 0o644)
		for _, args := range [][]string{{"add", "calc.go"}, {"commit", "-q", "-m", "Resolve calc.go"}} {
			cmd := exec.Command("git", args...)
			cmd.Dir, cmd.Env = spec.Dir, append([]string{"PATH=" + os.Getenv("PATH"), "HOME=" + spec.Dir}, spec.Editor.Env...)
			if out, err := cmd.CombinedOutput(); err != nil {
				return "git: " + string(out)
			}
		}
		// After committing, the agent plants commands git would run in its
		// repository. vuoro never runs git there, so none runs.
		script := filepath.Join(outside, "plant.sh")
		os.WriteFile(script, []byte("#!/bin/sh\ntouch "+canary+"\n"), 0o755)
		f, _ := os.OpenFile(filepath.Join(spec.Dir, ".git", "config"), os.O_APPEND|os.O_WRONLY, 0)
		f.WriteString("[core]\n\tfsmonitor = " + script + "\n\thooksPath = " + filepath.Dir(script) + "\n")
		f.Close()
		return "calc.go: kept both"
	}
	r, err := m.StartUpdate(ch, src, UpdateInput{Strategy: Merge, Own: true, Who: pat, Editor: Reviewer{Tool: "fake"}})
	if err != nil {
		t.Fatal(err)
	}
	v := wait(t, r, func(v RunView) bool { return v.Update != nil && len(v.Update.Stops) == 1 && idle(v) })
	stop := v.Update.Stops[0]
	if v.Update.State() != "resolving" || len(stop.Conflicted) != 1 || stop.Conflicted[0] != "calc.go" || stop.Seed == "" {
		t.Fatalf("stop %+v", v.Update)
	}
	if !strings.Contains(asked, "- calc.go") || !strings.Contains(asked, "Main moves on") || !strings.Contains(asked, "Subtract instead") {
		t.Fatalf("the editor's message: %s", asked)
	}
	spec := d.specs[0]
	if spec.Editor == nil || spec.Drafts != nil || spec.Dir != filepath.Join(m.editorDir(r.ID), "wt") || !strings.Contains(spec.SystemPrompt, "conflict markers") {
		t.Fatal("the stop's editor is not the sandboxed editor")
	}
	detail, err := m.UpdateDetail(context.Background(), r.ID)
	if err != nil || len(detail.Resolutions) != 1 || len(detail.Resolutions[0]) != 1 {
		t.Fatalf("detail %+v %v", detail, err)
	}
	u, err := m.AcceptStop(context.Background(), r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(canary); err == nil {
		t.Fatal("vuoro ran a command the editor's repository configured")
	}
	if u.State() != "ready" || u.Stops[0].Resolved != u.Result || len(u.Stops[0].Outside) != 0 {
		t.Fatalf("accepted %+v", u)
	}
	if got := cloneGit(t, m, "log", "-1", "--format=%P%x00%an", u.Result); got != ch.Diff.Head+" "+target+"\x00Pat" {
		t.Fatalf("merge commit parents %q", got)
	}
	if got := cloneGit(t, m, "show", u.Result+":calc.go"); !strings.Contains(got, "b - a") || strings.Contains(got, "<<<") {
		t.Fatalf("merged calc.go %q", got)
	}
	if _, err := os.Stat(m.editorDir(r.ID)); !os.IsNotExist(err) {
		t.Fatal("the stop's repository outlived its acceptance")
	}
	if _, err := m.AcceptStop(context.Background(), r.ID); err == nil {
		t.Fatal("a stop was accepted twice")
	}
	// The update and its result outlive a restart.
	loaded := &Manager{StateDir: m.StateDir, Repos: m.Repos}
	if err := loaded.Load(); err != nil {
		t.Fatal(err)
	}
	if lv := loaded.Get(r.ID).View(); lv.Update == nil || lv.Update.Result != u.Result || lv.Mode != UpdateMode {
		t.Fatalf("reloaded %+v", lv.Update)
	}
	if err := m.DiscardUpdate(context.Background(), r.ID); err != nil {
		t.Fatal(err)
	}
	if out := cloneGit(t, m, "for-each-ref", "refs/vuoro/update/"); out != "" {
		t.Fatalf("a discarded update left refs: %s", out)
	}
}

func TestAResolutionWithMarkersLeftIsRefused(t *testing.T) {
	m, d, ch, src, _ := updateSetup(t, true)
	d.edit = func(spec agent.Spec, text string) string {
		os.WriteFile(filepath.Join(spec.Dir, "calc.go"), []byte("package calc\n<<<<<<< ours\nfunc Add(a, b int) int { return a }\n"), 0o644)
		for _, args := range [][]string{{"add", "calc.go"}, {"commit", "-q", "-m", "Half done"}} {
			cmd := exec.Command("git", args...)
			cmd.Dir, cmd.Env = spec.Dir, append([]string{"PATH=" + os.Getenv("PATH"), "HOME=" + spec.Dir}, spec.Editor.Env...)
			cmd.Run()
		}
		return "done, I think"
	}
	r, err := m.StartUpdate(ch, src, UpdateInput{Strategy: Merge, Own: true, Who: pat, Editor: Reviewer{Tool: "fake"}})
	if err != nil {
		t.Fatal(err)
	}
	wait(t, r, func(v RunView) bool { return v.Update != nil && len(v.Update.Stops) == 1 && idle(v) })
	if _, err := m.AcceptStop(context.Background(), r.ID); err == nil || !strings.Contains(err.Error(), "markers are still in calc.go") {
		t.Fatalf("leftover markers: %v", err)
	}
	if r.View().Update.State() != "resolving" {
		t.Fatal("a refused resolution closed the stop")
	}
}

func TestUpdateRefusals(t *testing.T) {
	m, _, ch, src, _ := updateSetup(t, false)
	for name, in := range map[string]UpdateInput{
		"rebase of another's": {Strategy: Rebase, Own: false, Who: pat, Editor: Reviewer{Tool: "fake"}},
		"no identity":         {Strategy: Merge, Own: true, Editor: Reviewer{Tool: "fake"}},
		"unknown strategy":    {Strategy: "squash", Own: true, Who: pat, Editor: Reviewer{Tool: "fake"}},
	} {
		if _, err := m.StartUpdate(ch, src, in); err == nil {
			t.Errorf("%s: started", name)
		}
	}
	closed := *ch
	closed.State = forge.Merged
	if _, err := m.StartUpdate(&closed, src, UpdateInput{Strategy: Merge, Own: true, Who: pat, Editor: Reviewer{Tool: "fake"}}); err == nil {
		t.Error("a merged change was updated")
	}
	// A change that already holds its target has nothing to update.
	up := *ch
	up.TargetBranch = "feature"
	src.Refspecs = []string{"+refs/heads/feature:refs/vuoro/head", "+refs/heads/feature:refs/vuoro/target/feature"}
	r, err := m.StartUpdate(&up, src, UpdateInput{Strategy: Merge, Own: true, Who: pat, Editor: Reviewer{Tool: "fake"}})
	if err != nil {
		t.Fatal(err)
	}
	if v := wait(t, r, func(v RunView) bool { return v.Status == Failed || v.Status == Idle }); v.Status != Failed || !strings.Contains(v.Error, "already holds") {
		t.Fatalf("up to date: %s %q", v.Status, v.Error)
	}
}

