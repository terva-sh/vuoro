package review

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func historyGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, b)
	}
	return strings.TrimSpace(string(b))
}

func historyCommit(t *testing.T, dir, name string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0600); err != nil {
		t.Fatal(err)
	}
	historyGit(t, dir, "add", name)
	historyGit(t, dir, "commit", "-qm", name)
	return historyGit(t, dir, "rev-parse", "HEAD")
}

func TestHeadComparisonCountsHistoryWithoutMovingOrRerunningReview(t *testing.T) {
	m, driver, ch, src := setup(t, func(call toolCaller, _ string) string {
		call("propose_draft", map[string]any{"severity": "low", "title": "A finding", "description": "Preserve this wording"})
		return "Done"
	})
	r, err := m.Start(ch, src, []Reviewer{{Profile: "code"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	before := wait(t, r, func(v RunView) bool { return v.Status == Idle && len(v.Drafts) == 1 })
	old := ch.Diff.Head
	historyCommit(t, src.Remote, "second")
	head := historyCommit(t, src.Remote, "third")
	ch.Diff.Head, src.Head = head, head
	s, err := m.CompareHead(context.Background(), r.ID, ch, src)
	if err != nil || s.Relation != "ahead" || s.Behind == nil || *s.Behind != 2 {
		t.Fatalf("linear: %+v %v", s, err)
	}
	historyGit(t, src.Remote, "switch", "-qc", "side", src.Base)
	historyCommit(t, src.Remote, "side-file")
	historyGit(t, src.Remote, "switch", "-q", "feature")
	historyGit(t, src.Remote, "merge", "--no-ff", "-qm", "merge", "side")
	head = historyGit(t, src.Remote, "rev-parse", "HEAD")
	ch.Diff.Head, src.Head = head, head
	s, err = m.CompareHead(context.Background(), r.ID, ch, src)
	if err != nil || s.Behind == nil || *s.Behind != 4 {
		t.Fatalf("merge: %+v %v", s, err)
	}
	after := r.View()
	driver.mu.Lock()
	launched := len(driver.specs)
	driver.mu.Unlock()
	if launched != 1 || after.Head != old || after.Drafts[0].Description != before.Drafts[0].Description {
		t.Fatal("comparison reran or mutated the review")
	}
	if historyGit(t, r.dir, "rev-parse", "HEAD") != old {
		t.Fatal("comparison moved the worktree")
	}
	loaded := &Manager{StateDir: m.StateDir}
	if err := loaded.Load(); err != nil {
		t.Fatal(err)
	}
	if loaded.Get(r.ID).Head != old {
		t.Fatal("reload lost pinned head")
	}
	historyGit(t, src.Remote, "switch", "-qc", "rewritten", src.Base)
	head = historyCommit(t, src.Remote, "replacement")
	historyGit(t, src.Remote, "update-ref", "refs/heads/feature", head)
	ch.Diff.Head, src.Head = head, head
	s, err = m.CompareHead(context.Background(), r.ID, ch, src)
	if err != nil || s.Relation != "rewritten" || s.Behind != nil {
		t.Fatalf("rewrite: %+v %v", s, err)
	}
	s = m.Repos.compare(context.Background(), src, strings.Repeat("0", 40))
	if s.Relation != "unavailable" || s.Behind != nil {
		t.Fatal("missing history invented a count")
	}
	ch.Ref.Number++
	if _, err = m.CompareHead(context.Background(), r.ID, ch, src); err == nil {
		t.Fatal("compared another change")
	}
}

func TestHeadComparisonCancellationWhileWaitingForRepository(t *testing.T) {
	r := &Repos{Dir: t.TempDir()}
	src := Source{Key: "host/team/repo", Head: strings.Repeat("a", 40)}
	unlock, err := r.lock(context.Background(), filepath.Join(r.Dir, "host/team/repo.git"))
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	end := make(chan HeadState, 1)
	go func() { end <- r.compare(ctx, src, strings.Repeat("b", 40)) }()
	select {
	case s := <-end:
		if s.Relation != "unavailable" || s.Behind != nil {
			t.Fatal("cancellation invented a count")
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled comparison remained blocked on repository lock")
	}
}
