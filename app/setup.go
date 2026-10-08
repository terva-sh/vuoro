package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/terva-sh/vuoro/agent"
	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/forge/adapters"
	"github.com/terva-sh/vuoro/review"
)

// Setup in the window (D45) writes the configuration file a person could
// also edit by hand. It never takes a token: the person stores theirs where
// they choose, and the file names the command or the environment variable
// that supplies it (D29). A save refuses a file that changed or broke since
// the window read it, and saves are serialized.

func (a *App) setupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/setup", a.setup)
	mux.HandleFunc("POST /api/setup/test", a.testForge)
	mux.HandleFunc("POST /api/setup/forges", a.saveForge)
	mux.HandleFunc("POST /api/setup/forges/remove", a.removeForge)
	mux.HandleFunc("POST /api/setup/tools", a.checkTools)
	mux.HandleFunc("POST /api/setup/background", a.saveBackground)
	mux.HandleFunc("POST /api/setup/autostart", a.setAutostart)
}

// Autostarter starts vuoro at login (D55). The window turns it on and off;
// nothing is registered until the person asks.
type Autostarter interface {
	Enabled() (bool, error)
	Enable() error
	Disable() error
}

// backgroundView is how vuoro runs with its window closed (D55).
type backgroundView struct {
	RefreshMinutes int  `json:"refresh_minutes"`
	Notify         bool `json:"notify"`
	// Autostart is whether vuoro starts at login, and Login whether this
	// platform lets it.
	Autostart bool   `json:"autostart"`
	Login     bool   `json:"login"`
	LoginErr  string `json:"login_error,omitempty"`
}

func (a *App) backgroundView(cfg *config.Config) backgroundView {
	v := backgroundView{RefreshMinutes: int(cfg.RefreshEvery() / time.Minute), Notify: cfg.Notifies(), Login: a.Autostart != nil}
	if a.Autostart != nil {
		on, err := a.Autostart.Enabled()
		v.Autostart = on
		if err != nil {
			v.LoginErr = "vuoro could not tell whether it starts at login."
		}
	}
	return v
}

// saveBackground saves the background refresh interval and whether vuoro
// notifies. The default interval is saved as no setting, so the file keeps
// following the default.
func (a *App) saveBackground(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Base           string `json:"base"`
		RefreshMinutes int    `json:"refresh_minutes"`
		Notify         bool   `json:"notify"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if req.RefreshMinutes < 1 || req.RefreshMinutes > 1440 {
		writeError(w, errors.New("Refresh every 1 to 1440 minutes."))
		return
	}
	a.editConfig(w, req.Base, func(c *config.Config) error {
		if c.Inbox == nil {
			c.Inbox = &config.Inbox{}
		}
		c.Inbox.RefreshMinutes = req.RefreshMinutes
		if req.RefreshMinutes == config.DefaultRefreshMinutes {
			c.Inbox.RefreshMinutes = 0
		}
		c.Inbox.Notify = nil
		if !req.Notify {
			off := false
			c.Inbox.Notify = &off
		}
		if *c.Inbox == (config.Inbox{}) {
			c.Inbox = nil
		}
		return nil
	})
}

// setAutostart turns starting at login on or off.
func (a *App) setAutostart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if a.Autostart == nil {
		writeError(w, fmt.Errorf("%w: vuoro cannot start at login on this system", forge.ErrUnsupported))
		return
	}
	var err error
	if req.Enabled {
		err = a.Autostart.Enable()
	} else {
		err = a.Autostart.Disable()
	}
	if err != nil {
		writeError(w, fmt.Errorf("Starting at login could not be changed: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, a.setupView())
}

// forgeForm is one forge as the setup form edits it: everything but the
// token, which the form never holds.
type forgeForm struct {
	Name         string   `json:"name"`
	Kind         string   `json:"kind"`
	URL          string   `json:"url"`
	Username     string   `json:"username"`
	TokenCommand []string `json:"token_command,omitempty"`
	TokenEnv     string   `json:"token_env,omitempty"`
	Clone        string   `json:"clone,omitempty"`
}

func (f forgeForm) forge() config.Forge {
	return config.Forge{Name: strings.TrimSpace(f.Name), Kind: f.Kind, URL: strings.TrimRight(strings.TrimSpace(f.URL), "/"),
		Username: strings.TrimSpace(f.Username), TokenCommand: f.TokenCommand, TokenEnv: strings.TrimSpace(f.TokenEnv), Clone: f.Clone}
}

// forgeView is a configured forge as setup shows it. EnvSet says whether the
// named variable is set in vuoro's environment; its value is never sent.
type forgeView struct {
	forgeForm
	EnvSet bool `json:"env_set,omitempty"`
}

// toolView is what a tool's probe found, and the state the setup screen
// gives its hint for: ok, missing, too_old, signed_out, or broken.
type toolView struct {
	probe
	State string `json:"state"`
}

// tokenSource is a place a token can be kept, offered when its program is on
// this machine. Read is the token_command and Store the command the person
// runs in a terminal to keep the token there, both as argv, where {host}
// stands for the forge's host. vuoro never runs Store; the window shows it
// shell-quoted, so no host can become shell syntax when it is pasted.
type tokenSource struct {
	ID    string   `json:"id"`
	Label string   `json:"label"`
	Read  []string `json:"read,omitempty"`
	Store []string `json:"store,omitempty"`
	Note  string   `json:"note,omitempty"`
}

// tokenSources are the sources offered on goos, given what look finds on
// PATH. An environment variable and a command of the person's own are
// always offered.
func tokenSources(goos string, look func(string) (string, error)) []tokenSource {
	has := func(p string) bool { _, err := look(p); return err == nil }
	var out []tokenSource
	if goos == "darwin" && has("security") {
		out = append(out, tokenSource{ID: "keychain", Label: "macOS Keychain",
			Read:  []string{"security", "find-generic-password", "-s", "vuoro", "-a", "{host}", "-w"},
			Store: []string{"security", "add-generic-password", "-s", "vuoro", "-a", "{host}", "-w"},
			Note:  "It asks for the token, so it never lands in your shell history."})
	}
	if goos == "linux" && has("secret-tool") {
		out = append(out, tokenSource{ID: "secret-service", Label: "Secret Service (GNOME Keyring, KWallet)",
			Read:  []string{"secret-tool", "lookup", "service", "vuoro", "host", "{host}"},
			Store: []string{"secret-tool", "store", "--label=vuoro {host}", "service", "vuoro", "host", "{host}"},
			Note:  "It asks for the token, so it never lands in your shell history."})
	}
	if has("pass") {
		out = append(out, tokenSource{ID: "pass", Label: "pass",
			Read:  []string{"pass", "show", "vuoro/{host}"},
			Store: []string{"pass", "insert", "vuoro/{host}"},
			Note:  "The entry must hold the token alone, on one line."})
	}
	if has("op") {
		out = append(out, tokenSource{ID: "1password", Label: "1Password",
			Read: []string{"op", "read", "op://Private/vuoro {host}/credential"},
			Note: "Save the token in 1Password as an API Credential named \"vuoro {host}\" in your Private vault, or change the reference to the item you use."})
	}
	return append(out,
		tokenSource{ID: "env", Label: "An environment variable", Note: "Set it wherever vuoro is launched from. Anything vuoro's launcher starts can read it."},
		tokenSource{ID: "command", Label: "A command of your own", Note: "Any program that prints the token on one line. vuoro runs it without a shell."})
}

func (a *App) setupView() any {
	probes := a.probeTools()
	// One snapshot: the forges shown and the base a save sends back must
	// come from the same file.
	a.cfgMu.RLock()
	cfg, path, base, cfgErr := a.Config, a.ConfigPath, a.ConfigBase, a.ConfigErr
	a.cfgMu.RUnlock()
	if cfg == nil {
		cfg = &config.Config{}
	}
	out := struct {
		ConfigError string           `json:"config_error,omitempty"`
		ConfigPath  string           `json:"config_path,omitempty"`
		Editable    bool             `json:"editable"`
		Base        string           `json:"base"`
		Forges      []forgeView      `json:"forges"`
		Sources     []tokenSource    `json:"sources"`
		Tools       []toolView       `json:"tools"`
		Presets     []config.Preset  `json:"presets"`
		Profiles    []review.Profile `json:"profiles"`
		Waived      []string         `json:"waived"` // confirmations the person turned off
		Scheme      string           `json:"scheme"` // the pinned colour scheme, or empty for the system's
		Tab         string           `json:"tab"`    // the home page's tab used last
		Background  backgroundView   `json:"background"`
	}{ConfigPath: path, Editable: path != "" && cfgErr == nil, Base: base, Forges: []forgeView{}, Tools: []toolView{},
		Sources: tokenSources(runtime.GOOS, exec.LookPath), Presets: presetsOf(cfg), Profiles: review.Builtin}
	out.Background = a.backgroundView(cfg)
	prefs := a.loadPrefs()
	out.Waived, out.Scheme, out.Tab = prefs.Waived, prefs.Scheme, prefs.Tab
	if cfgErr != nil {
		out.ConfigError = cfgErr.Error()
	}
	for _, f := range cfg.Forges {
		v := forgeView{forgeForm: forgeForm{f.Name, f.Kind, f.URL, f.Username, f.TokenCommand, f.TokenEnv, f.Clone}}
		if f.TokenEnv != "" {
			v.EnvSet = strings.TrimSpace(os.Getenv(f.TokenEnv)) != ""
		}
		out.Forges = append(out.Forges, v)
	}
	for _, p := range probes {
		out.Tools = append(out.Tools, toolView{probe: p, State: p.state})
	}
	slices.SortFunc(out.Tools, func(x, y toolView) int { return strings.Compare(x.Name, y.Name) })
	for _, p := range cfg.Profiles {
		out.Profiles = append(out.Profiles, review.Profile(p))
	}
	return out
}

// toolState names what a probe's error means for the person.
func toolState(p agent.Probe, err error) string {
	switch {
	case errors.Is(err, agent.ErrMissing):
		return "missing"
	case errors.Is(err, agent.ErrTooOld):
		return "too_old"
	case err != nil:
		return "broken"
	case p.Account == "":
		return "signed_out"
	}
	return "ok"
}

func (a *App) setup(w http.ResponseWriter, r *http.Request) {
	a.reloadConfig()
	writeJSON(w, http.StatusOK, a.setupView())
}

// reloadConfig reads the file again when it changed since vuoro last read
// it, so a hand edit takes effect and setup edits what the file now holds. A
// file that no longer loads leaves the running configuration in place, with
// its error shown and setup unable to save until it is fixed.
func (a *App) reloadConfig() {
	a.saveMu.Lock()
	defer a.saveMu.Unlock()
	a.cfgMu.RLock()
	path, base, cfgErr := a.ConfigPath, a.ConfigBase, a.ConfigErr
	a.cfgMu.RUnlock()
	if path == "" {
		return
	}
	c, cur, err := config.Read(path)
	switch {
	case err != nil:
		a.cfgMu.Lock()
		a.ConfigErr = err
		a.cfgMu.Unlock()
	case cur != base || cfgErr != nil:
		a.setConfig(c, cur)
	}
}

func (a *App) checkTools(w http.ResponseWriter, r *http.Request) {
	if !readJSON(w, r, &struct{}{}) {
		return
	}
	a.reprobeTools()
	writeJSON(w, http.StatusOK, a.setupView())
}

// testForge resolves a token from the form's source and asks the forge whose
// it is. The token goes to the forge and nowhere else: not to the reply, a
// log, or an error.
func (a *App) testForge(w http.ResponseWriter, r *http.Request) {
	var req forgeForm
	if !readJSON(w, r, &req) {
		return
	}
	f := req.forge()
	if f.Name == "" {
		if u, err := url.Parse(f.URL); err == nil && u.Host != "" {
			f.Name = u.Host
		} else {
			f.Name = "this forge"
		}
	}
	probe := f
	if probe.Username == "" {
		probe.Username = "unknown" // the test is how the form learns it
	}
	if err := probe.Check(); err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Minute)
	defer cancel()
	token, err := f.Token(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	dial := a.Dial
	if dial == nil {
		dial = adapters.New
	}
	c, err := dial(f.Kind, f.URL, token)
	if err != nil {
		writeError(w, err)
		return
	}
	acct, ok := c.(forge.Account)
	if !ok {
		writeError(w, fmt.Errorf("%w: vuoro cannot ask this forge whose token it is", forge.ErrUnsupported))
		return
	}
	me, err := acct.Me(ctx)
	if err != nil {
		writeError(w, fmt.Errorf("The forge did not accept the token: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"username": me, "matches": f.Username == "" || strings.EqualFold(me, f.Username)})
}

// saveForge adds a forge, or replaces the one named original, and reloads.
func (a *App) saveForge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Base     string    `json:"base"`
		Original string    `json:"original"`
		Forge    forgeForm `json:"forge"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	a.editConfig(w, req.Base, func(c *config.Config) error {
		f := req.Forge.forge()
		if f.Kind == "" {
			return errors.New("Choose GitLab, Forgejo, or GitHub.")
		}
		if req.Original == "" {
			c.Forges = append(c.Forges, f)
			return nil
		}
		i := slices.IndexFunc(c.Forges, func(g config.Forge) bool { return g.Name == req.Original })
		if i < 0 {
			return fmt.Errorf("There is no forge named %q to change.", req.Original)
		}
		f.Bots = c.Forges[i].Bots // the form does not show them, and a save keeps them
		c.Forges[i] = f
		return nil
	})
}

func (a *App) removeForge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Base string `json:"base"`
		Name string `json:"name"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	a.editConfig(w, req.Base, func(c *config.Config) error {
		n := len(c.Forges)
		c.Forges = slices.DeleteFunc(c.Forges, func(g config.Forge) bool { return g.Name == req.Name })
		if len(c.Forges) == n {
			return fmt.Errorf("There is no forge named %q.", req.Name)
		}
		return nil
	})
}

// editConfig changes the configuration file as the window last read it, and
// reloads it. base is the hash the window was shown: a file edited by hand
// since is refused rather than overwritten.
func (a *App) editConfig(w http.ResponseWriter, base string, change func(*config.Config) error) {
	a.saveMu.Lock()
	defer a.saveMu.Unlock()
	a.cfgMu.RLock()
	path, cfgErr := a.ConfigPath, a.ConfigErr
	a.cfgMu.RUnlock()
	switch {
	case path == "":
		writeError(w, errors.New("vuoro does not know where its configuration file is, so setup cannot save."))
		return
	case cfgErr != nil:
		writeError(w, errors.New("The configuration file does not load. Fix it by hand first, so nothing in it is lost."))
		return
	}
	c, cur, err := config.Read(path)
	if err != nil {
		writeError(w, err)
		return
	}
	if cur != base {
		writeError(w, config.ErrChanged)
		return
	}
	if err := change(c); err != nil {
		writeError(w, err)
		return
	}
	if err := config.Save(path, c, cur); err != nil {
		writeError(w, err)
		return
	}
	saved, next, err := config.Read(path)
	if err != nil {
		writeError(w, err)
		return
	}
	a.setConfig(saved, next)
	writeJSON(w, http.StatusOK, a.setupView())
}

// setConfig swaps in a configuration that was just saved. Forge clients and
// the inbox are rebuilt from it on their next use, so a changed token source
// or a new account takes effect without a restart.
func (a *App) setConfig(c *config.Config, base string) {
	a.cfgMu.Lock()
	a.Config, a.ConfigErr, a.ConfigBase = c, nil, base
	a.cfgMu.Unlock()
	a.clientMu.Lock()
	a.mu.Lock()
	a.clients = nil
	a.mu.Unlock()
	a.clientMu.Unlock()
	a.inboxMu.Lock()
	a.inboxReady = false
	a.inboxMu.Unlock()
}
