package app

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
)

func TestLocalDraftsRoundTripPrivatelyAndLeaveNoEmptyFile(t *testing.T) {
	s := localDrafts{dir: filepath.Join(t.TempDir(), "drafts")}
	ref := forge.Ref{Host: "forge.example:2222", Project: "o/r", Number: 3}
	if ds, err := s.Drafts(ref); err != nil || ds != nil {
		t.Fatalf("an unwritten change: %v %v", ds, err)
	}
	want := []forge.Draft{{ID: "local-1", Body: "inline", Anchor: &forge.Anchor{Path: "a.go", NewLine: 4}}, {ID: "local-2", Body: "reply", ThreadID: "31"}}
	if err := s.SaveDrafts(ref, want); err != nil {
		t.Fatal(err)
	}
	got, err := s.Drafts(ref)
	if err != nil || len(got) != 2 || got[0].Anchor.NewLine != 4 || got[1].ThreadID != "31" {
		t.Fatalf("%+v %v", got, err)
	}
	// The file is named by a hash, so the host's port and the project's
	// slash never become a path.
	entries, _ := os.ReadDir(s.dir)
	if len(entries) != 1 || strings.ContainsAny(entries[0].Name(), ":/") {
		t.Fatalf("files %v", entries)
	}
	if runtime.GOOS != "windows" {
		if fi, _ := os.Stat(s.path(ref)); fi.Mode().Perm() != 0o600 {
			t.Fatalf("mode %v", fi.Mode().Perm())
		}
	}
	if ds, _ := s.Drafts(forge.Ref{Host: "forge.example:2222", Project: "o/r", Number: 4}); ds != nil {
		t.Fatal("another change read these drafts")
	}
	if err := s.SaveDrafts(ref, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.path(ref)); !os.IsNotExist(err) {
		t.Fatal("an empty draft file was left")
	}
}

// drafter is a forge client that keeps its drafts in a store it is given.
type drafter struct {
	capsForge
	store forge.DraftStore
}

func (d *drafter) KeepDraftsIn(s forge.DraftStore) { d.store = s }

func TestAClientThatKeepsDraftsLocallyGetsAStore(t *testing.T) {
	d := &drafter{}
	dir := t.TempDir()
	a := &App{StateDir: dir, Config: &config.Config{}, Dial: func(string, string, config.Secret) (forge.Provider, error) { return d, nil }}
	if _, err := a.client(context.Background(), "jo.test", forge.Forgejo); err != nil {
		t.Fatal(err)
	}
	s, ok := d.store.(localDrafts)
	if !ok || s.dir != filepath.Join(dir, "drafts") {
		t.Fatalf("store %#v", d.store)
	}
}
