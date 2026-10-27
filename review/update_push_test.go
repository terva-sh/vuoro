package review

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/terva-sh/vuoro/forge"
)

func remoteGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func readyUpdate(t *testing.T, strategy string, own bool) (*Manager, *Run, string, string) {
	t.Helper()
	m, _, ch, src, _ := updateSetup(t, false)
	r, err := m.StartUpdate(ch, src, UpdateInput{Strategy: strategy, Own: own, Who: pat, Editor: Reviewer{Tool: "fake"}})
	if err != nil {
		t.Fatal(err)
	}
	v := wait(t, r, func(v RunView) bool { return v.Status == Idle || v.Status == Failed })
	if v.Update.State() != "ready" {
		t.Fatalf("update %s %q %+v", v.Status, v.Error, v.Update)
	}
	return m, r, src.Remote, v.Update.Result
}

// A merge on the person's own change goes to its branch as a fast-forward,
// and only the result they saw, from the head the forge still reports.
func TestAnOwnMergeIsPushedToTheChangesBranch(t *testing.T) {
	m, r, remote, result := readyUpdate(t, Merge, true)
	ctx := context.Background()
	if _, err := m.PushUpdate(ctx, r.ID, remote, r.Head, "0123abcd", nil, nil); err == nil {
		t.Fatal("a result the person did not see was pushed")
	}
	if _, err := m.PushUpdate(ctx, r.ID, remote, "4567cdef", result, nil, nil); err == nil || !strings.Contains(err.Error(), "moved") {
		t.Fatalf("a moved branch was pushed over: %v", err)
	}
	p, err := m.PushUpdate(ctx, r.ID, remote, r.Head, result, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Branch != "feature" || p.URL != "" || remoteGit(t, remote, "rev-parse", "feature") != result {
		t.Fatalf("pushed %+v", p)
	}
	if _, err := m.PushUpdate(ctx, r.ID, remote, r.Head, result, nil, nil); err == nil {
		t.Fatal("an update was pushed twice")
	}
	if err := m.DiscardUpdate(ctx, r.ID); err == nil {
		t.Fatal("a pushed update was discarded")
	}
}

// A rebase forces with a lease against the head it started from, so a push
// that landed since is refused even when the forge's answer was stale.
func TestARebaseIsPushedWithALease(t *testing.T) {
	m, r, remote, result := readyUpdate(t, Rebase, true)
	ctx := context.Background()
	remoteGit(t, remote, "switch", "-q", "feature")
	remoteGit(t, remote, "commit", "-q", "--allow-empty", "-m", "Someone pushed")
	moved := remoteGit(t, remote, "rev-parse", "HEAD")
	remoteGit(t, remote, "switch", "-q", "main")
	if _, err := m.PushUpdate(ctx, r.ID, remote, r.Head, result, nil, nil); err == nil {
		t.Fatal("the lease let a rebase overwrite a push")
	}
	if got := remoteGit(t, remote, "rev-parse", "feature"); got != moved {
		t.Fatalf("feature is %s, want the other push %s kept", got, moved)
	}
	remoteGit(t, remote, "update-ref", "refs/heads/feature", r.Head)
	p, err := m.PushUpdate(ctx, r.ID, remote, r.Head, result, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Branch != "feature" || remoteGit(t, remote, "rev-parse", "feature") != result {
		t.Fatalf("pushed %+v", p)
	}
}

// On somebody else's change a merge goes to a branch of vuoro's and is
// opened as a draft change onto theirs, whose branch is never written.
func TestAnotherPersonsChangeGetsADraftChange(t *testing.T) {
	m, r, remote, result := readyUpdate(t, Merge, false)
	var opened string
	p, err := m.PushUpdate(context.Background(), r.ID, remote, r.Head, result, func(branch string) (string, error) {
		opened = branch
		return "https://gitlab.example/g/calc/-/merge_requests/8", nil
	}, func() error { t.Fatal("the forge was asked to update somebody else's branch"); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(p.Branch, "vuoro/update-7-") || opened != p.Branch || p.URL == "" {
		t.Fatalf("pushed %+v, opened %q", p, opened)
	}
	if remoteGit(t, remote, "rev-parse", p.Branch) != result || remoteGit(t, remote, "rev-parse", "feature") != r.Head {
		t.Fatal("the update went to the author's branch")
	}
}

// A clean update of the person's own change goes through the forge when it
// has the strategy, and through the push when it does not.
func TestACleanUpdateGoesThroughTheForge(t *testing.T) {
	m, r, remote, result := readyUpdate(t, Merge, true)
	ctx := context.Background()
	if _, err := m.PushUpdate(ctx, r.ID, remote, r.Head, result, nil, func() error { return forge.ErrHeadMoved }); err == nil || !strings.Contains(err.Error(), "moved") {
		t.Fatalf("a moved head: %v", err)
	}
	asked := 0
	p, err := m.PushUpdate(ctx, r.ID, remote, r.Head, result, nil, func() error { asked++; return nil })
	if err != nil || asked != 1 || !p.Forge || p.SHA != "" || p.Branch != "feature" {
		t.Fatalf("through the forge %+v %v", p, err)
	}
	if remoteGit(t, remote, "rev-parse", "feature") != r.Head {
		t.Fatal("vuoro pushed as well as the forge")
	}

	m, r, remote, result = readyUpdate(t, Rebase, true)
	p, err = m.PushUpdate(ctx, r.ID, remote, r.Head, result, nil, func() error { return forge.ErrUnsupported })
	if err != nil || p.Forge || remoteGit(t, remote, "rev-parse", "feature") != result {
		t.Fatalf("the fallback push %+v %v", p, err)
	}
}
