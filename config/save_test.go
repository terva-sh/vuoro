package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestASaveKeepsWhatTheFormDoesNotShow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vuoro", "config.json")
	c, base, err := Read(path)
	if err != nil || base != "" || len(c.Forges) != 0 {
		t.Fatalf("a missing file: %+v %q %v", c, base, err)
	}
	c.Forges = append(c.Forges, Forge{Name: "internal", Kind: "forgejo", URL: "https://forge.example", Username: "pat", TokenEnv: "VUORO_INTERNAL_TOKEN", Bots: []string{"ci-bot"}})
	c.Presets = []Preset{{Name: "deep", Reviewers: []Reviewer{{Profile: "security"}}}}
	if err := Save(path, c, base); err != nil {
		t.Fatal(err)
	}
	back, base2, err := Read(path)
	if err != nil || base2 == "" || back.Forges[0].Bots[0] != "ci-bot" || back.Presets[0].Name != "deep" {
		t.Fatalf("read back %+v %v", back, err)
	}
	if runtime.GOOS != "windows" {
		if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
			t.Fatalf("mode %v", fi.Mode().Perm())
		}
	}
	// Saving every forge away leaves a file that still loads.
	back.Forges = nil
	if err := Save(path, back, base2); err != nil {
		t.Fatal(err)
	}
	if c, err := Load(path); err != nil || len(c.Forges) != 0 {
		t.Fatalf("after removing every forge: %+v %v", c, err)
	}
}

func TestASaveNeverOverwritesAnEditOrABrokenFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte(`{"forges": []}`), 0o600)
	c, base, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	edit := `{"forges": [], "inbox": {"dormant_days": 30}}`
	os.WriteFile(path, []byte(edit), 0o600)
	if err := Save(path, c, base); !errors.Is(err, ErrChanged) {
		t.Fatalf("over a hand edit: %v", err)
	}
	if b, _ := os.ReadFile(path); string(b) != edit {
		t.Fatalf("the edit was lost: %s", b)
	}
	// A file that was there and is gone is a change too.
	os.Remove(path)
	if err := Save(path, c, base); !errors.Is(err, ErrChanged) {
		t.Fatalf("over a removed file: %v", err)
	}
	broken := `{"forges": [`
	os.WriteFile(path, []byte(broken), 0o600)
	if err := Save(path, c, hash([]byte(broken))); err == nil || !strings.Contains(err.Error(), "will not overwrite") {
		t.Fatalf("over a broken file: %v", err)
	}
	if b, _ := os.ReadFile(path); string(b) != broken {
		t.Fatal("the broken file was overwritten")
	}
	// A configuration Load would refuse is never written.
	c.Forges = []Forge{{Name: "x", Kind: "bitbucket", URL: "https://x.example", Username: "u", TokenEnv: "T"}}
	os.Remove(path)
	if err := Save(path, c, ""); err == nil {
		t.Fatal("an unsupported forge was saved")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("a refused save wrote the file")
	}
}

// Two saves from the same base cannot both pass the check: one writes, and
// the other is told the file changed.
func TestConcurrentSavesFromOneBaseLetOnlyOneThrough(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(path, []byte(`{"forges": []}`), 0o600)
	_, base, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	const n = 8
	errs := make(chan error, n)
	var start sync.WaitGroup
	start.Add(1)
	for i := range n {
		go func() {
			start.Wait()
			c := &Config{Forges: []Forge{{Name: fmt.Sprintf("f%d", i), Kind: "gitlab", URL: fmt.Sprintf("https://f%d.example", i), Username: "pat", TokenEnv: "T"}}}
			errs <- Save(path, c, base)
		}()
	}
	start.Done()
	won := 0
	for range n {
		switch err := <-errs; {
		case err == nil:
			won++
		case !errors.Is(err, ErrChanged):
			t.Fatalf("a losing save: %v", err)
		}
	}
	if won != 1 {
		t.Fatalf("%d saves from one base went through", won)
	}
}
