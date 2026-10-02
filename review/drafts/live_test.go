package drafts

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/terva-sh/vuoro/agent"
	"github.com/terva-sh/vuoro/agent/claude"
	"github.com/terva-sh/vuoro/server"
)

// TestLiveClaude has a real Claude Code reviewer propose, revise, and
// withdraw a draft through this server, mounted on vuoro's loopback server
// behind its guard. It costs money and needs the person's login, so it runs
// only with VUORO_LIVE_CLAUDE=1.
func TestLiveClaude(t *testing.T) {
	if os.Getenv("VUORO_LIVE_CLAUDE") != "1" {
		t.Skip("talks to the real claude; set VUORO_LIVE_CLAUDE=1")
	}
	ln, err := server.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	st := NewStore()
	mcp := NewServer()
	hs := server.HTTPServer(server.New(server.Config{Addr: addr, Token: server.NewToken(), Dist: fstest.MapFS{}, MCP: mcp}))
	go hs.Serve(ln)
	defer hs.Close()

	dir := t.TempDir()
	src, err := os.ReadFile(filepath.Join("..", "..", "agent", "claude", "testdata", "calc.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "calc.go"), src, 0o600); err != nil {
		t.Fatal(err)
	}

	bearer := mcp.Register(st, Session{ID: "r1", Role: Reviewer, Drafted: "claude haiku"})
	d := &claude.Driver{ConfigDir: t.TempDir()}
	s, err := d.Start(context.Background(), agent.Spec{
		Dir: dir, Model: "haiku",
		SystemPrompt: "You are a code reviewer. You read files, and you record findings only with the vuoro drafts tools. Be brief.",
		Drafts: &agent.MCPServer{
			Name:   "vuoro",
			Config: map[string]any{"type": "http", "url": "http://" + addr + "/mcp/", "headers": map[string]string{"Authorization": "Bearer " + bearer}},
			Tools:  ToolNames(Reviewer),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	for _, msg := range []string{
		"Read calc.go. Propose one draft for the bug in Mean, anchored to its line.",
		"Revise that draft so its severity is critical.",
		"Withdraw that draft, then list your drafts including withdrawn ones.",
	} {
		if err := s.Send(context.Background(), msg); err != nil {
			t.Fatal(err)
		}
		for ev := range s.Events() {
			if ev.Kind == agent.ToolCall {
				t.Logf("tool call: %s %s", ev.Tool.Name, ev.Tool.Input)
			}
			if ev.Kind == agent.ToolResult && ev.Tool.IsError {
				t.Logf("tool error: %s", ev.Tool.Output)
			}
			if ev.Kind == agent.TurnDone {
				if ev.Err != nil {
					t.Fatalf("turn failed: %v", ev.Err)
				}
				break
			}
		}
	}

	all := st.All()
	t.Logf("store: %+v", all)
	if len(all) == 0 {
		t.Fatal("no draft reached the store")
	}
	last := all[len(all)-1]
	if last.Status != Withdrawn || last.Severity != "critical" || last.Revision < 3 || last.Anchor == nil || !strings.HasSuffix(last.Anchor.Path, "calc.go") {
		t.Fatalf("the draft did not go propose, revise, withdraw: %+v", last)
	}
}
