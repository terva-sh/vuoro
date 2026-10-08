package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/terva-sh/vuoro/agent"
	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
)

// accountForge answers whose token it holds, and nothing else.
type accountForge struct{ me string }

func (f *accountForge) Capabilities() forge.Capabilities { return forge.Capabilities{} }
func (f *accountForge) Open(context.Context, forge.Ref) (*forge.Change, error) {
	return nil, forge.ErrNotFound
}
func (f *accountForge) Me(context.Context) (string, error) { return f.me, nil }

type setupView struct {
	ConfigError string `json:"config_error"`
	ConfigPath  string `json:"config_path"`
	Editable    bool
	Base        string
	Forges      []struct {
		forgeForm
		EnvSet bool `json:"env_set"`
	}
	Sources    []tokenSource
	Error      string
	Background backgroundView
}

func getSetup(t *testing.T, h http.Handler) setupView {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/setup", nil))
	var v setupView
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil || w.Code != 200 {
		t.Fatalf("setup: %d %s", w.Code, w.Body)
	}
	return v
}

// post sends a setup request and reads the reply, failing the test if the
// sentinel token is anywhere in it.
func post(t *testing.T, h http.Handler, path string, body any) (int, setupView, string) {
	t.Helper()
	b, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", path, strings.NewReader(string(b))))
	if strings.Contains(w.Body.String(), setupSecret) {
		t.Fatalf("%s replied with the token: %s", path, w.Body)
	}
	var v setupView
	json.Unmarshal(w.Body.Bytes(), &v)
	return w.Code, v, w.Body.String()
}

const setupSecret = "sentinel-setup-secret"

func TestSetupAddsTestsChangesAndRemovesAForge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vuoro", "config.json")
	var dialled []string
	a := &App{StateDir: t.TempDir(), Config: &config.Config{}, ConfigPath: path,
		Dial: func(kind, base string, token config.Secret) (forge.Provider, error) {
			dialled = append(dialled, kind+" "+base+" "+token.Reveal())
			return &accountForge{me: "pat"}, nil
		}}
	h := a.Handler()
	v := getSetup(t, h)
	if !v.Editable || v.Base != "" || len(v.Forges) != 0 || v.ConfigPath != path {
		t.Fatalf("a fresh install: %+v", v)
	}
	if !slices.ContainsFunc(v.Sources, func(s tokenSource) bool { return s.ID == "env" }) {
		t.Fatalf("sources %+v", v.Sources)
	}

	// Testing a source resolves the token, hands it to the forge only, and
	// says whose it is.
	form := forgeForm{Kind: "forgejo", URL: "https://forge.example/", TokenEnv: "VUORO_SETUP_TOKEN"}
	if code, _, body := post(t, h, "/api/setup/test", form); code != 400 || !strings.Contains(body, "is not set") {
		t.Fatalf("an unset variable: %d %s", code, body)
	}
	t.Setenv("VUORO_SETUP_TOKEN", setupSecret)
	code, _, body := post(t, h, "/api/setup/test", form)
	if code != 200 || !strings.Contains(body, `"username":"pat"`) || !strings.Contains(body, `"matches":true`) {
		t.Fatalf("test: %d %s", code, body)
	}
	// A forge that cannot say how far a token reaches leaves it unknown,
	// and the test still passes.
	if !strings.Contains(body, `"access":{"level":"unknown"}`) {
		t.Fatalf("access: %s", body)
	}
	if len(dialled) != 1 || dialled[0] != "forgejo https://forge.example "+setupSecret {
		t.Fatalf("dialled %q", dialled)
	}
	form.Username = "someone-else"
	if _, _, body := post(t, h, "/api/setup/test", form); !strings.Contains(body, `"matches":false`) {
		t.Fatalf("another account: %s", body)
	}

	// Saving writes the source, never the token, and the inbox has the
	// account at once.
	form.Name, form.Username = "internal", "pat"
	code, v, body = post(t, h, "/api/setup/forges", map[string]any{"base": "", "forge": form})
	if code != 200 || len(v.Forges) != 1 || !v.Forges[0].EnvSet || v.Base == "" {
		t.Fatalf("save: %d %s", code, body)
	}
	file, _ := os.ReadFile(path)
	if strings.Contains(string(file), setupSecret) || !strings.Contains(string(file), `"token_env": "VUORO_SETUP_TOKEN"`) {
		t.Fatalf("the file holds %s", file)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/inbox", nil))
	if !strings.Contains(w.Body.String(), `"name":"internal"`) {
		t.Fatalf("the inbox did not pick up the account: %s", w.Body)
	}

	// A hand edit made meanwhile is never overwritten. Opening setup again
	// reads it, and a save then keeps what the form does not show.
	stale := v.Base
	edited := strings.Replace(string(file), `"username": "pat"`, `"username": "pat",`+"\n"+`      "bots": ["ci-bot"]`, 1)
	os.WriteFile(path, []byte(edited), 0o600)
	form.TokenEnv, form.TokenCommand = "", []string{"secret-tool", "lookup", "service", "vuoro", "host", "forge.example"}
	if code, _, body := post(t, h, "/api/setup/forges", map[string]any{"base": stale, "original": "internal", "forge": form}); code != 400 || !strings.Contains(body, "changed since") {
		t.Fatalf("over a hand edit: %d %s", code, body)
	}
	if b, _ := os.ReadFile(path); string(b) != edited {
		t.Fatal("the hand edit was lost")
	}
	v = getSetup(t, h)
	if code, _, body := post(t, h, "/api/setup/forges", map[string]any{"base": v.Base, "original": "internal", "forge": form}); code != 200 {
		t.Fatalf("edit: %d %s", code, body)
	}
	if f := a.config().Forges[0]; len(f.Bots) != 1 || f.TokenEnv != "" || f.TokenCommand[0] != "secret-tool" {
		t.Fatalf("after the edit %+v", f)
	}

	// Removing it leaves a file that still loads, with no forge.
	v = getSetup(t, h)
	if code, v, body := post(t, h, "/api/setup/forges/remove", map[string]any{"base": v.Base, "name": "internal"}); code != 200 || len(v.Forges) != 0 {
		t.Fatalf("remove: %d %s", code, body)
	}
	if c, err := config.Load(path); err != nil || len(c.Forges) != 0 {
		t.Fatalf("after removing: %+v %v", c, err)
	}
}

func TestSetupNeverSavesOverABrokenFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	broken := `{"forges": [`
	os.WriteFile(path, []byte(broken), 0o600)
	a := &App{StateDir: t.TempDir(), ConfigPath: path, Config: &config.Config{}}
	h := a.Handler()
	v := getSetup(t, h)
	if v.Editable || v.ConfigError == "" {
		t.Fatalf("a broken file: %+v", v)
	}
	form := forgeForm{Name: "x", Kind: "gitlab", URL: "https://gitlab.example", Username: "pat", TokenEnv: "T"}
	if code, _, body := post(t, h, "/api/setup/forges", map[string]any{"base": "", "forge": form}); code != 400 || !strings.Contains(body, "Fix it by hand") {
		t.Fatalf("save over a broken file: %d %s", code, body)
	}
	if b, _ := os.ReadFile(path); string(b) != broken {
		t.Fatal("the broken file was overwritten")
	}
	// Fixed by hand, it loads again when setup opens.
	os.WriteFile(path, []byte(`{"forges": []}`), 0o600)
	if v := getSetup(t, h); !v.Editable || v.ConfigError != "" {
		t.Fatalf("after the fix: %+v", v)
	}
}

func TestTokenSourcesAreThoseThisMachineHas(t *testing.T) {
	on := func(goos string, progs ...string) []string {
		var ids []string
		for _, s := range tokenSources(goos, func(p string) (string, error) {
			if slices.Contains(progs, p) {
				return "/usr/bin/" + p, nil
			}
			return "", errors.New("not found")
		}) {
			ids = append(ids, s.ID)
		}
		return ids
	}
	for _, c := range []struct {
		goos  string
		progs []string
		want  string
	}{
		{"linux", []string{"secret-tool", "pass"}, "secret-service pass env command"},
		{"linux", nil, "env command"},
		{"darwin", []string{"security", "op"}, "keychain 1password env command"},
		{"windows", []string{"security", "secret-tool"}, "env command"},
	} {
		if got := strings.Join(on(c.goos, c.progs...), " "); got != c.want {
			t.Errorf("%s with %v: %s, want %s", c.goos, c.progs, got, c.want)
		}
	}
}

func TestEachToolStateHasItsHint(t *testing.T) {
	for want, c := range map[string]struct {
		p   agent.Probe
		err error
	}{
		"missing":    {err: errors.Join(agent.ErrMissing)},
		"too_old":    {err: errors.Join(agent.ErrTooOld)},
		"broken":     {err: errors.New("claude --version: exit status 1")},
		"signed_out": {p: agent.Probe{Version: "2.1.300"}},
		"ok":         {p: agent.Probe{Version: "2.1.300", Account: "claude.ai max"}},
	} {
		if got := toolState(c.p, c.err); got != want {
			t.Errorf("toolState = %s, want %s", got, want)
		}
	}
}

// accessForge is a forge that says how far its token reaches.
type accessForge struct {
	accountForge
	asked *int
}

func (f *accessForge) Access(context.Context) (forge.Access, error) {
	*f.asked++
	return forge.Access{Level: forge.AccessBroad, Scopes: []string{"all"}, Broad: []string{"all"}}, nil
}

// The settings page asks how far each configured forge's token reaches,
// through the client vuoro already holds, so the token is resolved once.
func TestSettingsAsksHowFarAConfiguredTokenReaches(t *testing.T) {
	t.Setenv("VUORO_SETTINGS_TOKEN", setupSecret)
	dialled, asked := 0, 0
	a := &App{StateDir: t.TempDir(), Config: &config.Config{Forges: []config.Forge{{Name: "internal", Kind: "forgejo", URL: "https://forge.example", Username: "pat", TokenEnv: "VUORO_SETTINGS_TOKEN"}}},
		Dial: func(string, string, config.Secret) (forge.Provider, error) {
			dialled++
			return &accessForge{accountForge{me: "pat"}, &asked}, nil
		}}
	h := a.Handler()
	for range 2 {
		code, _, body := post(t, h, "/api/setup/access", map[string]string{"name": "internal"})
		if code != 200 || !strings.Contains(body, `"level":"broad"`) {
			t.Fatalf("access: %d %s", code, body)
		}
	}
	if dialled != 1 || asked != 2 {
		t.Fatalf("dialled %d, asked %d: the client is held, and each check asks again", dialled, asked)
	}
	if code, _, body := post(t, h, "/api/setup/access", map[string]string{"name": "elsewhere"}); code == 200 || !strings.Contains(body, `no forge is named`) {
		t.Fatalf("an unknown forge: %d %s", code, body)
	}
}
