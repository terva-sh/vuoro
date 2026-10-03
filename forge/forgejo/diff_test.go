package forgejo

import (
	"github.com/terva-sh/vuoro/forge/patch"
	"testing"
)

func TestDiffInventoryAndIncompletePatches(t *testing.T) {
	fs := []apiFile{{Filename: "space name.go", Status: "changed"}, {Filename: "renamed.go", Previous: "old.go", Status: "renamed"}, {Filename: "image.png", Status: "changed"}, {Filename: "truncated.go", Status: "changed"}, {Filename: "deleted.go", Status: "deleted"}, {Filename: "added.go", Status: "added"}}
	diff := "diff --git a/space name.go b/space name.go\n--- a/space name.go\n+++ b/space name.go\n@@ -2,2 +2,2 @@\n same\n-old\n+new\n" +
		"diff --git a/old.go b/renamed.go\nsimilarity index 100%\nrename from old.go\nrename to renamed.go\n" +
		"diff --git a/image.png b/image.png\nBinary files differ\n" +
		"diff --git a/truncated.go b/truncated.go\n@@ -1,3 +1,3 @@\n-a\n+b\n" +
		"diff --git a/deleted.go b/deleted.go\n@@ -1 +0,0 @@\n-gone\n" +
		"diff --git a/added.go b/added.go\n@@ -0,0 +1 @@\n+new\n"
	got := fileDiffs(fs, diff)
	if got[0].Omitted || got[1].OldPath != "old.go" || !got[1].Renamed || got[1].Omitted || !got[2].Omitted || !got[3].Omitted || got[3].Patch != "" || !got[4].Deleted || got[4].Omitted || !got[5].New || got[5].Omitted {
		t.Fatalf("inventory: %+v", got)
	}
	old, new, ok := patch.Line(got[0].Patch, true, 3)
	if !ok || old != 0 || new != 3 {
		t.Fatalf("added anchor: %d %d %v", old, new, ok)
	}
	old, new, ok = patch.Line(got[4].Patch, false, 1)
	if !ok || old != 1 || new != 0 {
		t.Fatalf("removed anchor: %d %d %v", old, new, ok)
	}
}
func TestQuotedPathsAndNoNewlineMarker(t *testing.T) {
	got := fileDiffs([]apiFile{{Filename: "café.go", Status: "changed"}}, "diff --git \"a/caf\\303\\251.go\" \"b/caf\\303\\251.go\"\n@@ -1 +1 @@\n-a\n\\ No newline at end of file\n+b\n\\ No newline at end of file\n")
	if len(got) != 1 || got[0].Omitted || got[0].Patch == "" {
		t.Fatalf("quoted diff: %+v", got)
	}
}

func TestFileTotalsDetectACompletelyMissingHunk(t *testing.T) {
	added, deleted := 2, 2
	got := fileDiffs([]apiFile{{Filename: "a.go", Status: "changed", Additions: &added, Deletions: &deleted}}, "diff --git a/a.go b/a.go\n@@ -1 +1 @@\n-old\n+new\n")
	if !got[0].Omitted || got[0].Patch != "" {
		t.Fatal("truncated diff with one complete hunk accepted")
	}
}
