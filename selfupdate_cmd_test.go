package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/terva-sh/vuoro/update"
)

// latestRelease serves a latest release named tag with no archives, which is
// all a command that does not apply an update reads.
func latestRelease(t *testing.T, tag string) update.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(update.Release{Tag: tag})
	}))
	t.Cleanup(srv.Close)
	return update.Client{API: srv.URL}
}

func TestSelfUpdateSaysWhatItWouldDo(t *testing.T) {
	target := filepath.Join(t.TempDir(), "vuoro")
	os.WriteFile(target, []byte("old"), 0o755)
	exe := func() (string, error) { return target, nil }
	for _, c := range []struct {
		version, latest string
		args            []string
		code            int
		out             string
	}{
		{"v0.2.0", "v0.2.0", nil, exitClean, "vuoro v0.2.0 is up to date"},
		{"v0.1.1", "v0.2.0", []string{"-check"}, exitFindings, "vuoro v0.2.0 is available; this is v0.1.1"},
		{"v0.1.1", "v0.2.0", []string{"-check", "-json"}, exitFindings, `"newer": true`},
		{"dev", "v0.2.0", nil, exitFailed, "not a release build"},
		{"v0.1.1", "v0.2.0", []string{"now"}, exitUsage, "takes no arguments"},
	} {
		var out, errb bytes.Buffer
		code := selfUpdateCmd{client: latestRelease(t, c.latest), version: c.version, executable: exe}.run(c.args, &out, &errb)
		if code != c.code || !strings.Contains(out.String()+errb.String(), c.out) {
			t.Errorf("%s %v: exit %d, want %d saying %q:\n%s%s", c.version, c.args, code, c.code, c.out, out.String(), errb.String())
		}
	}
	if b, _ := os.ReadFile(target); string(b) != "old" {
		t.Fatalf("a check or a refusal replaced the binary: %q", b)
	}
}

// Applying fails here, since the served release carries no archive, and the
// failure is the command's exit status 3 with the reason.
func TestSelfUpdateReportsAFailedUpdate(t *testing.T) {
	target := filepath.Join(t.TempDir(), "vuoro")
	os.WriteFile(target, []byte("old"), 0o755)
	var out, errb bytes.Buffer
	code := selfUpdateCmd{client: latestRelease(t, "v0.2.0"), version: "v0.1.1",
		executable: func() (string, error) { return target, nil }}.run(nil, &out, &errb)
	if code != exitFailed || !strings.Contains(errb.String(), "has no archive for") {
		t.Fatalf("exit %d:\n%s", code, errb.String())
	}
	if b, _ := os.ReadFile(target); string(b) != "old" {
		t.Fatalf("the binary was replaced: %q", b)
	}
}
