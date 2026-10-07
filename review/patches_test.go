package review

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/review/drafts"
)

// A fix keeps its patch (D51): a commit vuoro pushed, and one the person
// linked, are saved beside the run record, and read again after the run is
// closed and vuoro restarts. One over the cap keeps only its files.
func TestAFixKeepsItsPatchPastTheRunsClose(t *testing.T) {
	m, d, ch, src := setup(t, func(call toolCaller, text string) string {
		call("propose_draft", map[string]any{"severity": "high", "title": "Add subtracts", "description": "d",
			"anchor": map[string]any{"path": "calc.go", "start_line": 3}})
		call("propose_draft", map[string]any{"severity": "medium", "title": "Add needs a comment", "description": "d"})
		return "done"
	})
	d.edit = fixFinding(t)
	r, err := m.Start(ch, src, []Reviewer{{Profile: "code", Tool: "fake"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	wait(t, r, idle)
	ctx, who := context.Background(), Identity{Name: "Pat", Email: "pat@example.com"}
	if _, err := m.StartEditor(ctx, r.ID, ch, who, EditorTask{Own: true, Items: []string{"d1"}}); err != nil {
		t.Fatal(err)
	}
	wait(t, r, func(v RunView) bool {
		return len(v.Sessions) == 2 && v.Sessions[1].FirstComplete && v.Sessions[1].Status == Idle
	})
	detail, _ := m.Editor(ctx, r.ID)
	first := detail.Commits[0].SHA
	git := func(args ...string) string {
		out, err := exec.Command("git", append([]string{"-C", src.Remote, "-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("switch", "-q", "main")
	if _, err := m.Editor(ctx, r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := m.PushEditor(ctx, r.ID, src.Remote, forge.StackTarget{Branch: "feature", Head: ch.Diff.Head}, first, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Patch(r.ID, strings.Repeat("0", 40)); !errors.Is(err, ErrNoPatch) {
		t.Fatalf("a commit never saved: %v", err)
	}

	// Someone pushes a large fix by hand, and the person links it to d2.
	git("switch", "-q", "feature")
	os.WriteFile(filepath.Join(src.Remote, "big.txt"), []byte(strings.Repeat("a line of text\n", maxPatch/15+100)), 0o644)
	git("add", "big.txt")
	git("commit", "-q", "-m", "Comment Add")
	big := git("rev-parse", "HEAD")
	git("switch", "-q", "main")
	moved := src
	moved.Head = big
	if err := m.MarkFixed(ctx, r.ID, "d2", big, &moved); err != nil {
		t.Fatal(err)
	}

	m.Close(ctx, r.ID)
	m.Shutdown()
	m2 := &Manager{StateDir: m.StateDir, Repos: m.Repos, Drivers: m.Drivers, Drafts: drafts.NewServer(), MCPURL: m.MCPURL}
	if err := m2.Load(); err != nil {
		t.Fatal(err)
	}
	v := m2.Get(r.ID).View()
	if len(v.Patches) != 2 || v.Patches[0].SHA != first || v.Patches[0].By != "editor" || v.Patches[1].By != "person" {
		t.Fatalf("patches %+v", v.Patches)
	}
	p, err := m2.Patch(r.ID, first)
	if err != nil {
		t.Fatal(err)
	}
	if p.Subject != "Fix d1" || !strings.Contains(p.Raw, "Vuoro-Finding: d1") || len(p.FileDiff) != 1 || p.FileDiff[0].NewPath != "calc.go" ||
		!strings.HasPrefix(p.FileDiff[0].Patch, "@@ ") || !strings.Contains(p.FileDiff[0].Patch, "+// wave 1\n") {
		t.Fatalf("the editor's patch %+v", p)
	}
	q, err := m2.Patch(r.ID, big)
	if err != nil || !q.TooLarge || q.Raw != "" || len(q.Files) != 1 || q.Files[0] != "big.txt" {
		t.Fatalf("the large patch %+v %v", q, err)
	}
	if _, err := os.Stat(r.patchPath(big)); !os.IsNotExist(err) {
		t.Fatalf("a patch over the cap was written: %v", err)
	}
}

func TestAPatchIsSplitIntoItsFiles(t *testing.T) {
	raw := `From 0123 Mon Sep 17 00:00:00 2001
From: Pat <pat@example.com>
Subject: [PATCH] Move things

Vuoro-Finding: d1
---
 a.go => b.go | 0
 new.go       | 1 +
 old.go       | 1 -
 img.png      | Bin

diff --git a/a.go b/b.go
similarity index 100%
rename from a.go
rename to b.go
diff --git a/new.go b/new.go
new file mode 100644
index 0000000..1111111
--- /dev/null
+++ b/new.go
@@ -0,0 +1 @@
+package x
diff --git a/old.go b/old.go
deleted file mode 100644
index 1111111..0000000
--- a/old.go
+++ /dev/null
@@ -1 +0,0 @@
-package x
diff --git a/img.png b/img.png
index 1111111..2222222 100644
GIT binary patch
literal 1
Icmeyc000000

`
	got := parsePatch(raw)
	if len(got) != 4 {
		t.Fatalf("files %+v", got)
	}
	if f := got[0]; !f.Renamed || f.OldPath != "a.go" || f.NewPath != "b.go" || f.Omitted {
		t.Fatalf("rename %+v", f)
	}
	if f := got[1]; !f.New || f.OldPath != "new.go" || f.Patch != "@@ -0,0 +1 @@\n+package x\n" {
		t.Fatalf("new %+v", f)
	}
	if f := got[2]; !f.Deleted || f.NewPath != "old.go" || f.Patch != "@@ -1 +0,0 @@\n-package x\n" {
		t.Fatalf("deleted %+v", f)
	}
	if f := got[3]; !f.Omitted || f.NewPath != "img.png" {
		t.Fatalf("binary %+v", f)
	}
}
