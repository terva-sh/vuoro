package review

import (
	"strings"
	"testing"
)

// A patch's sections are matched to numstat's files by path, so a section
// that is not the file the list names at its place refuses the fit.
func TestPatchSectionsAreMatchedToTheirFilesByPath(t *testing.T) {
	patch := "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-x\n+y\n" +
		"diff --git a/link b/link\ndeleted file mode 100644\n-file\n" +
		"diff --git a/link b/link\nnew file mode 120000\n+a.go\n"
	sections := splitPatch(patch)
	if len(sections) != 2 || !strings.HasPrefix(sections[1], "diff --git a/link b/link\ndeleted") || !strings.Contains(sections[1], "new file mode 120000") {
		t.Fatalf("a type change was not kept as one section: %q", sections)
	}
	if !sectionsMatch(sections, []LeftOut{{Path: "a.go"}, {Path: "link"}}) {
		t.Fatal("matching sections were refused")
	}
	if sectionsMatch(sections, []LeftOut{{Path: "link"}, {Path: "a.go"}}) || sectionsMatch(sections, []LeftOut{{Path: "a.go"}}) {
		t.Fatal("sections out of step with the files were accepted")
	}
}
