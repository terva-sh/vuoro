package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/terva-sh/vuoro/config"
)

// TestLiveSetupTest runs the window's Test button, the route behind it, on a
// real forge with the real adapters: the token comes from an environment
// source, as a form names it, and the reply names the account it signs in
// as, and never the token.
//
// VUORO_LIVE_SETUP_KIND=github VUORO_LIVE_SETUP_URL=https://github.com
// VUORO_LIVE_SETUP_USERNAME=login VUORO_LIVE_SETUP_TOKEN=...
// go test -run TestLiveSetupTest -v ./app
func TestLiveSetupTest(t *testing.T) {
	kind, url, user := os.Getenv("VUORO_LIVE_SETUP_KIND"), os.Getenv("VUORO_LIVE_SETUP_URL"), os.Getenv("VUORO_LIVE_SETUP_USERNAME")
	if kind == "" || url == "" || user == "" || os.Getenv("VUORO_LIVE_SETUP_TOKEN") == "" {
		t.Skip("set VUORO_LIVE_SETUP_KIND, _URL, _USERNAME and _TOKEN to test a token source on a real forge")
	}
	a := &App{StateDir: t.TempDir(), Config: &config.Config{}, ConfigPath: filepath.Join(t.TempDir(), "config.json")}
	h := a.Handler()
	form := forgeForm{Kind: kind, URL: url, TokenEnv: "VUORO_LIVE_SETUP_TOKEN"}
	code, _, body := post(t, h, "/api/setup/test", form)
	if code != 200 || !strings.Contains(strings.ToLower(body), `"username":"`+strings.ToLower(user)+`"`) || !strings.Contains(body, `"matches":true`) {
		t.Fatalf("test: %d %s", code, body)
	}
	if strings.Contains(body, os.Getenv("VUORO_LIVE_SETUP_TOKEN")) {
		t.Fatal("the reply holds the token")
	}
	if !strings.Contains(body, `"access":{"level":"`) {
		t.Fatalf("no access: %s", body)
	}
	t.Logf("access: %s", body[strings.Index(body, `"access"`):])
	form.Username = user + "-someone-else"
	if _, _, body := post(t, h, "/api/setup/test", form); !strings.Contains(body, `"matches":false`) {
		t.Fatalf("another account: %s", body)
	}
	// A token the forge does not accept is refused, and the reply says so.
	t.Setenv("VUORO_LIVE_SETUP_TOKEN", "vuoro-live-setup-not-a-token")
	if code, _, body := post(t, h, "/api/setup/test", form); code == 200 || !strings.Contains(body, "did not accept the token") {
		t.Fatalf("a bad token: %d %s", code, body)
	}
	t.Logf("%s %s: the test signed in as %s", kind, url, user)
}
