package patch

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestIdenticalFilesHaveNoHunks(t *testing.T) {
	if got := Unified("a\nb\n", "a\nb\n"); got != "" {
		t.Fatalf("%q", got)
	}
}

func TestAOneLineChange(t *testing.T) {
	a := "1\n2\n3\n4\n5\n6\n7\n8\n9\n"
	b := "1\n2\n3\n4\nfive\n6\n7\n8\n9\n"
	want := "@@ -2,7 +2,7 @@\n 2\n 3\n 4\n-5\n+five\n 6\n 7\n 8\n"
	if got := Unified(a, b); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestNewAndDeletedFiles(t *testing.T) {
	if got := Unified("", "x\ny\n"); got != "@@ -0,0 +1,2 @@\n+x\n+y\n" {
		t.Fatalf("new file: %q", got)
	}
	if got := Unified("x\n", ""); got != "@@ -1 +0,0 @@\n-x\n" {
		t.Fatalf("deleted file: %q", got)
	}
}

// TestAgreesWithGit applies each generated patch with git apply, which
// checks every hunk header and line against the old file.
func TestAgreesWithGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	cases := [][2]string{
		{"a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk\nl\nm\n", "a\nB\nc\nd\ne\nf\ng\nh\ni\nj\nk\nL\nm\nn\n"},
		{"x\n", "y\nx\nz\n"},
		{strings.Repeat("same\n", 20) + "old\n" + strings.Repeat("same\n", 20), strings.Repeat("same\n", 20) + "new\nmore\n" + strings.Repeat("same\n", 20)},
		{"", "only\nnew\n"},
	}
	for i, c := range cases {
		dir := t.TempDir()
		f := filepath.Join(dir, "f.txt")
		if err := os.WriteFile(f, []byte(c[0]), 0o600); err != nil {
			t.Fatal(err)
		}
		p := "--- a/f.txt\n+++ b/f.txt\n" + Unified(c[0], c[1])
		if err := os.WriteFile(filepath.Join(dir, "p.diff"), []byte(p), 0o600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("git", "apply", "--unidiff-zero", "p.diff")
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("case %d: git apply: %v\n%s\n%s", i, err, out, p)
		}
		got, _ := os.ReadFile(f)
		if string(got) != c[1] {
			t.Fatalf("case %d: applied to %q, want %q", i, got, c[1])
		}
	}
}
