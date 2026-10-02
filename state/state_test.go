package state

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWritesAreWholeAndPrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "b", "run.json")
	if err := WriteJSON(path, map[string]int{"n": 1}); err != nil {
		t.Fatal(err)
	}
	if err := WriteJSON(path, map[string]int{"n": 2}); err != nil {
		t.Fatal(err)
	}
	var got map[string]int
	if err := ReadJSON(path, &got); err != nil || got["n"] != 2 {
		t.Fatalf("%v %v", got, err)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Fatalf("a temporary file was left: %v", entries)
	}
	if runtime.GOOS == "windows" {
		return
	}
	fi, _ := os.Stat(path)
	di, _ := os.Stat(filepath.Dir(path))
	if fi.Mode().Perm() != 0o600 || di.Mode().Perm() != 0o700 {
		t.Fatalf("file %v, dir %v", fi.Mode(), di.Mode())
	}
}
