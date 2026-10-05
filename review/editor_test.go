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
}
