package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/terva-sh/tuohi/dialog"
)

// childArgs carries vuoro's arguments into a child test binary, and marks it
// as one. tuohi needs the UI loop on the main goroutine, which in a test
// binary only TestMain has, so a test that opens the window runs vuoro in a
// child, as tuohi's examples do.
const childArgs = "VUORO_TEST_CHILD_ARGS"

func TestMain(m *testing.M) {
	if args, ok := os.LookupEnv(childArgs); ok {
		os.Exit(run(strings.Fields(args), os.Stdout, os.Stderr))
	}
	os.Exit(m.Run())
}

// TestCheck opens the window on vuoro's own server, and passes when the page
// reports that it reached both the server and Go. It needs a display, so it
// runs only with TUOHI_REQUIRE_GUI=1, as `just test-gui` sets.
func TestCheck(t *testing.T) {
	if os.Getenv("TUOHI_REQUIRE_GUI") != "1" {
		t.Skip("opens a window; set TUOHI_REQUIRE_GUI=1 where one can open, as `just test-gui` does")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0])
	cmd.Env = append(os.Environ(), childArgs+"=-check")
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("still running after a minute:\n%s", out)
	}
	if err != nil {
		t.Fatalf("%v:\n%s", err, out)
	}
	if !strings.Contains(string(out), "check: ok=true") {
		t.Fatalf("no passing report:\n%s", out)
	}
	t.Log(strings.TrimSpace(string(out)))
}

func TestVersionFlagPrintsTheVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-version"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	if got, want := stdout.String(), "vuoro "+version+"\n"; got != want {
		t.Fatalf("stdout %q, want %q", got, want)
	}
}

func TestUnknownFlagIsAUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-nope"}, &stdout, &stderr); code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "-nope") {
		t.Fatalf("stderr %q does not name the flag", stderr.String())
	}
}

func TestANonLoopbackAddressIsRefused(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-browser", "-addr", "0.0.0.0:0"}, &stdout, &stderr); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "127.0.0.1 only") {
		t.Fatalf("stderr %q does not say why", stderr.String())
	}
}

// A configuration file that does not load at launch is still the one setup
// names, and fixing it by hand takes effect when setup opens again.
func TestABrokenConfigAtLaunchCanBeFixedWithoutARestart(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the XDG directories place the config file on Linux only")
	}
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))
	path := filepath.Join(dir, "config", "vuoro", "config.json")
	os.MkdirAll(filepath.Dir(path), 0o700)
	os.WriteFile(path, []byte(`{"forges": [`), 0o600)
	a, _, err := newApp("http://127.0.0.1:1/mcp")
	if err != nil {
		t.Fatal(err)
	}
	a.Tools = nil // probing the person's own tools is not this test's business
	setup := func() (out struct {
		ConfigError string `json:"config_error"`
		ConfigPath  string `json:"config_path"`
		Editable    bool   `json:"editable"`
	}) {
		w := httptest.NewRecorder()
		a.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/api/setup", nil))
		json.Unmarshal(w.Body.Bytes(), &out)
		return out
	}
	if got := setup(); got.ConfigError == "" || got.ConfigPath != path || got.Editable {
		t.Fatalf("a broken file at launch: %+v", got)
	}
	os.WriteFile(path, []byte(`{"forges": []}`), 0o600)
	if got := setup(); got.ConfigError != "" || !got.Editable {
		t.Fatalf("after fixing it: %+v", got)
	}
}

func TestChooseFolderAsksForADirectoryAndAnswersEmptyWhenCancelled(t *testing.T) {
	var asked dialog.Options
	answer := []string{"/home/pat/work"}
	var fail error
	choose := chooseFolder(func(o dialog.Options) ([]string, error) { asked = o; return answer, fail })
	if got := choose("Add a folder"); got != "/home/pat/work" || asked.Type != dialog.TypeDirectory || asked.Title != "Add a folder" {
		t.Fatalf("chose %q with %+v", got, asked)
	}
	answer = nil
	if got := choose("Add a folder"); got != "" {
		t.Fatalf("a cancelled chooser answered %q", got)
	}
	answer, fail = []string{"/x"}, errors.New("no window")
	if got := choose("Add a folder"); got != "" {
		t.Fatalf("a chooser that failed answered %q", got)
	}
}
