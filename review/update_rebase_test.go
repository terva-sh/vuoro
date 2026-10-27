package review

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/terva-sh/vuoro/agent"
)

// resolveWith commits body as calc.go in the editor's repository.
func resolveWith(spec agent.Spec, body string) string {
	os.WriteFile(filepath.Join(spec.Dir, "calc.go"), []byte(body), 0o644)
	for _, args := range [][]string{{"add", "calc.go"}, {"commit", "-q", "-m", "Resolve calc.go"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir, cmd.Env = spec.Dir, append([]string{"PATH=" + os.Getenv("PATH"), "HOME=" + spec.Dir}, spec.Editor.Env...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return "git: " + string(out)
		}
	}
	return "calc.go: resolved"
}

func TestACleanRebaseReplaysEveryCommitWithItsAuthor(t *testing.T) {
	m, d, ch, src, target := updateSetup(t, false)
	r, err := m.StartUpdate(ch, src, UpdateInput{Strategy: Rebase, Own: true, Who: pat, Editor: Reviewer{Tool: "fake"}})
	if err != nil {
		t.Fatal(err)
	}
	v := wait(t, r, func(v RunView) bool { return v.Status == Idle || v.Status == Failed })
	if v.Status != Idle || v.Update.State() != "ready" || len(d.specs) != 0 {
		t.Fatalf("clean rebase %s %q %+v", v.Status, v.Error, v.Update)
	}
	log := cloneGit(t, m, "log", "--format=%an|%cn|%s", target+".."+v.Update.Result)
	if log != "t|Pat|Add Sub\nt|Pat|Subtract instead" {
		t.Fatalf("rebased log %q", log)
	}
	if base := cloneGit(t, m, "merge-base", target, v.Update.Result); base != target {
		t.Fatal("the result does not stand on the target")
	}
	detail, _ := m.UpdateDetail(context.Background(), r.ID)
	if len(detail.Commits) != 2 {
		t.Fatalf("detail commits %+v", detail.Commits)
	}
}

func TestARebaseStopsAtEachConflictingCommit(t *testing.T) {
	m, d, ch, src, target := updateSetup(t, true)
	var stops []string
	d.edit = func(spec agent.Spec, text string) string {
		stops = append(stops, text)
		if strings.Contains(text, "Add Sub") && len(stops) > 1 {
			return resolveWith(spec, "package calc\n\nfunc Add(a, b int) int { return b - a }\n\nfunc Sub(a, b int) int { return a - b }\n")
		}
		return resolveWith(spec, "package calc\n\nfunc Add(a, b int) int { return b - a }\n")
	}
	r, err := m.StartUpdate(ch, src, UpdateInput{Strategy: Rebase, Own: true, Who: pat, Editor: Reviewer{Tool: "fake"}})
	if err != nil {
		t.Fatal(err)
	}
	v := wait(t, r, func(v RunView) bool { return v.Update != nil && len(v.Update.Stops) == 1 && idle(v) })
	if v.Update.Stops[0].Subject != "Subtract instead" || len(v.Update.Remaining) != 2 {
		t.Fatalf("first stop %+v", v.Update)
	}
	if _, err := m.AcceptStop(context.Background(), r.ID); err != nil {
		t.Fatal(err)
	}
	// The next commit's context changed with the resolution, so it stops
	// too, with a session and repository of its own.
	v = wait(t, r, func(v RunView) bool { return len(v.Update.Stops) == 2 && idle(v) })
	if v.Update.Stops[1].Subject != "Add Sub" || v.Update.Stops[1].Session == v.Update.Stops[0].Session || len(d.specs) != 2 {
		t.Fatalf("second stop %+v", v.Update.Stops)
	}
	if !strings.Contains(stops[1], "## The commit (untrusted)") {
		t.Fatalf("the stop's message names no commit: %s", stops[1])
	}
	u, err := m.AcceptStop(context.Background(), r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if u.State() != "ready" {
		t.Fatalf("after the last stop %+v", u)
	}
	log := cloneGit(t, m, "log", "--format=%an|%cn|%s", target+".."+u.Result)
	if log != "t|Pat|Add Sub\nt|Pat|Subtract instead" {
		t.Fatalf("rebased log %q", log)
	}
	if got := cloneGit(t, m, "show", u.Result+":calc.go"); !strings.Contains(got, "b - a") || !strings.Contains(got, "func Sub") {
		t.Fatalf("rebased calc.go %q", got)
	}
}

func TestARebaseRefusesAHistoryWithAMerge(t *testing.T) {
	m, _, ch, src, _ := updateSetup(t, false)
	git := func(args ...string) string {
		out, err := exec.Command("git", append([]string{"-C", src.Remote, "-c", "user.name=t", "-c", "user.email=t@example.com"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("switch", "-q", "feature")
	git("merge", "-q", "--no-edit", "main")
	ch.Diff.Head = git("rev-parse", "HEAD")
	src.Head = ch.Diff.Head
	git("switch", "-q", "main")
	os.WriteFile(filepath.Join(src.Remote, "NEW"), []byte("x\n"), 0o644)
	git("add", "NEW")
	git("commit", "-q", "-m", "More on main")
	r, err := m.StartUpdate(ch, src, UpdateInput{Strategy: Rebase, Own: true, Who: pat, Editor: Reviewer{Tool: "fake"}})
	if err != nil {
		t.Fatal(err)
	}
	if v := wait(t, r, func(v RunView) bool { return v.Status == Failed || v.Status == Idle }); v.Status != Failed || !strings.Contains(v.Error, "merge commit") {
		t.Fatalf("merge in history: %s %q", v.Status, v.Error)
	}
}
