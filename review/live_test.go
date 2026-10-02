package review

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/terva-sh/vuoro/agent"
	"github.com/terva-sh/vuoro/agent/claude"
	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/forge/gitlab"
	"github.com/terva-sh/vuoro/review/drafts"
)

// TestLive reviews a public gitlab.com merge request with Claude Code, on
// Haiku, from the anonymous fetch to the closed run. It spends a little of
// the person's Claude allowance, so it runs only when asked:
//
//	VUORO_LIVE_CLAUDE=1 VUORO_LIVE_GITLAB=1 go test -run TestLive -v ./review
func TestLive(t *testing.T) {
	if os.Getenv("VUORO_LIVE_CLAUDE") != "1" || os.Getenv("VUORO_LIVE_GITLAB") != "1" {
		t.Skip("set VUORO_LIVE_CLAUDE=1 and VUORO_LIVE_GITLAB=1 to review a merge request on gitlab.com")
	}
	ref := forge.Ref{Host: "gitlab.com", Project: "gitlab-org/cli", Number: 3991}
	c, _ := gitlab.New("https://gitlab.com", config.Secret{})
	ch, err := c.Open(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	srv := drafts.NewServer()
	hs := httptest.NewServer(srv)
	defer hs.Close()
	dir := t.TempDir()
	m := &Manager{
		StateDir: dir,
		Repos:    &Repos{Dir: filepath.Join(dir, "repos")},
		Drivers:  map[string]agent.Driver{"claude": &claude.Driver{ConfigDir: dir}},
		Drafts:   srv,
		MCPURL:   hs.URL + "/mcp/",
	}
	src := Source{
		Remote: "https://gitlab.com/gitlab-org/cli.git", Key: "gitlab.com/gitlab-org/cli",
		Refspecs: []string{"+refs/merge-requests/3991/head:refs/vuoro/mr/3991", "+refs/heads/main:refs/vuoro/target/main"},
		Head:     ch.Diff.Head, Base: ch.Diff.Base,
	}
	r, err := m.Start(ch, src, []Reviewer{{Profile: "code", Model: "haiku"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Minute)
	var v RunView
	for time.Now().Before(deadline) {
		v = r.View()
		if s := v.Sessions[0].Status; s == Idle || s == Failed || s == Ended {
			break
		}
		time.Sleep(2 * time.Second)
	}
	worktree := r.dir
	if err := m.Close(context.Background(), r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(worktree); !os.IsNotExist(err) {
		t.Fatalf("the worktree outlived the run: %v", err)
	}
	s := v.Sessions[0]
	if s.Status != Idle {
		t.Fatalf("session %s: %s", s.Status, s.Error)
	}
	for _, e := range s.Log {
		if e.Kind == "denied" {
			t.Errorf("the reviewer tried %s", e.Tool)
		}
		if e.Kind == "tool" && e.IsError {
			t.Logf("refused %s: %s", e.Tool, e.Output)
		}
	}
	t.Logf("%d drafts, $%.4f", len(v.Drafts), v.CostUSD)
	for _, d := range v.Drafts {
		t.Logf("%s [%s] %s %+v", d.ID, d.Severity, d.Title, d.Anchor)
	}
	if len(v.Drafts) == 0 {
		t.Log("no drafts: the reviewer found nothing, which a small change allows")
	}
}
