// Package app is vuoro's HTTP API under /api/: it opens changes on the
// person's forges and runs reviews of them. The server package mounts it
// behind the loopback guard, which is where the per-launch token is checked.
//
// Nothing here posts to a forge. An agent's output is a pending draft, and a
// person posts it (docs/plan.md section 6.2).
package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/terva-sh/vuoro/agent"
	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/forge/adapters"
	"github.com/terva-sh/vuoro/inbox"
	"github.com/terva-sh/vuoro/projects"
	"github.com/terva-sh/vuoro/review"
	"github.com/terva-sh/vuoro/review/drafts"
)

// App holds what the API serves.
type App struct {
	// Version is vuoro's release, or empty for a development build.
	Version string
	// Config is the person's configuration. It is empty when there is no
	// file, and then a change opens anonymously, which public projects allow.
	Config *config.Config
	// ConfigErr is why the configuration did not load, shown to the person.
	ConfigErr error
	// ConfigPath is the configuration file setup writes, and ConfigBase the
	// hash of it as read (config.Read). With no path, setup only shows.
	ConfigPath string
	ConfigBase string
	cfgMu      sync.RWMutex // guards Config, ConfigErr and ConfigBase once serving
	saveMu     sync.Mutex   // makes each setup save, and each reload, one step
	Runs       *review.Manager
	// Tools are the agent tools vuoro knows, by name. Each is probed once,
	// and only a tool that answers is offered to a run.
	Tools map[string]agent.Driver
	// Dial makes the client for a host. Nil means https on that host; a
	// test points it at a fake forge.
	Dial func(kind, base string, token config.Secret) (forge.Provider, error)
	// StateDir keeps the person's UI choices, in prefs.json. Empty keeps
	// none, and every action asks first.
	StateDir string
	// PromptsDir holds the person's prompt files; empty is prompts/ beside
	// the configuration file (section 6.11).
	PromptsDir string
	// Notify shows a desktop notification; nil sends none (D55).
	Notify Notifier
	// RefreshEvery overrides the configured background refresh interval;
	// a test sets it.
	RefreshEvery time.Duration
	// Autostart starts vuoro at login; nil where the platform cannot.
	Autostart Autostarter
	// Dock is whether vuoro has its macOS Dock icon as it runs, which is
	// fixed as it starts; nil where there is no Dock to choose (D58).
	Dock *bool
	// Updates finds and applies a newer release (D74); nil offers none, as
	// a test that is not about updating has it. Executable is the binary an
	// update replaces; nil is the running one.
	Updates    Updater
	Executable func() (string, error)
	updMu      sync.Mutex // guards upd, and makes each check and update one step
	upd        selfUpdates
	// Log reports what the person cannot act on in the window; nil drops it.
	Log     func(format string, args ...any)
	nudgeMu sync.Mutex // guards told and nudges.json
	told    *nudges    // what was notified, once read

	prefsMu sync.Mutex
	// background holds work a reply does not wait for, such as the inbox
	// catching up after a write.
	background sync.WaitGroup
	probeMu    sync.Mutex
	probed     bool
	probes     map[string]probe

	clientMu     sync.Mutex
	mu           sync.Mutex
	clients      map[string]forge.Provider // by host and kind
	changes      map[string]*forge.Change  // by Ref.String(), the last read of each
	inboxMu      sync.Mutex
	inboxReady   bool
	inboxService *inbox.Service
	inboxErr     error
	// projectsMu guards project triage's service, built on first use like
	// the inbox's.
	projectsMu     sync.Mutex
	projectsReady  bool
	projectService *projects.Service
	projectsErr    error
	local          localState
}

type probe struct {
	Name    string `json:"name"`
	Tool    string `json:"tool"`
	Version string `json:"version,omitempty"`
	Account string `json:"account,omitempty"`
	Error   string `json:"error,omitempty"`
	// ShellLogin says the person's login shell holds a credential this
	// process cannot see, which is what a Dock or login-item launch does.
	// It says that one is set, never what it is.
	ShellLogin bool   `json:"shell_login,omitempty"`
	state      string // for the setup screen's hint (toolState)
}

// Handler returns the API's routes.
func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	a.setupRoutes(mux)
	a.localRoutes(mux)
	a.projectRoutes(mux)
	a.projectTriageRoutes(mux)
	a.updateRoutes(mux)
	a.promptRoutes(mux)
	a.askRoutes(mux)
	mux.HandleFunc("GET /api/about", a.about)
	mux.HandleFunc("GET /api/self-update", a.getSelfUpdate)
	mux.HandleFunc("POST /api/self-update", a.applySelfUpdate)
	mux.HandleFunc("GET /api/inbox", a.getInbox)
	mux.HandleFunc("POST /api/inbox/refresh", a.refreshInbox)
	mux.HandleFunc("POST /api/inbox/snooze", a.snoozeInbox)
	mux.HandleFunc("POST /api/prefs/confirm", a.setConfirm)
	mux.HandleFunc("POST /api/prefs/scheme", a.setScheme)
	mux.HandleFunc("POST /api/prefs/tab", a.setTab)
	mux.HandleFunc("POST /api/prefs/hint", a.setHint)
	mux.HandleFunc("POST /api/prefs/hints/reset", a.resetHints)
	mux.HandleFunc("POST /api/prefs/tour", a.setTour)
	mux.HandleFunc("POST /api/runs/{id}/drafts/{did}/confidence", a.overrideConfidence)
	mux.HandleFunc("POST /api/runs/{id}/report", a.report)
	mux.HandleFunc("POST /api/runs/{id}/drafts/{did}/suggest", a.askSuggestion)
	mux.HandleFunc("POST /api/changes", a.openChange)
	mux.HandleFunc("GET /api/runs", a.listRuns)
	mux.HandleFunc("POST /api/runs", a.startRun)
	mux.HandleFunc("POST /api/triage", a.startTriage)
	mux.HandleFunc("POST /api/runs/{id}/drafts/{did}/edit-reply", a.editReply)
	mux.HandleFunc("POST /api/runs/{id}/drafts/{did}/discard-reply", a.discardReply)
	mux.HandleFunc("GET /api/runs/{id}", a.getRun)
	mux.HandleFunc("POST /api/runs/{id}/head", a.compareHead)
	mux.HandleFunc("POST /api/runs/{id}/close", a.closeRun)
	mux.HandleFunc("POST /api/runs/{id}/editor", a.startEditor)
	mux.HandleFunc("GET /api/runs/{id}/editor", a.getEditor)
	mux.HandleFunc("POST /api/runs/{id}/editor/discard", a.discardEditor)
	mux.HandleFunc("POST /api/runs/{id}/editor/stack", a.previewStack)
	mux.HandleFunc("POST /api/runs/{id}/editor/push", a.pushEditor)
	mux.HandleFunc("GET /api/runs/{id}/editor/delivery", a.delivery)
	mux.HandleFunc("POST /api/runs/{id}/editor/suggestions", a.previewSuggestions)
	mux.HandleFunc("POST /api/runs/{id}/editor/suggest", a.suggestEditor)
	mux.HandleFunc("POST /api/runs/{id}/fixes", a.fixCommits)
	mux.HandleFunc("POST /api/runs/{id}/drafts/{did}/fixed", a.markFixed)
	mux.HandleFunc("GET /api/runs/{id}/patches/{sha}", a.getPatch)
	mux.HandleFunc("POST /api/runs/{id}/drafts/{did}/dismiss", a.dismiss)
	mux.HandleFunc("POST /api/runs/{id}/sessions/{sid}/messages", a.say)
	mux.HandleFunc("POST /api/runs/{id}/sessions/{sid}/stop", a.stop)
	a.writeRoutes(mux)
	return a.keepProjectRuns(mux)
}

func (a *App) config() *config.Config {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	if a.Config == nil {
		return &config.Config{}
	}
	return a.Config
}

// configErr is why the configuration did not load, or nil.
func (a *App) configErr() error {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	return a.ConfigErr
}

// probeTools probes every tool once, and offers the run manager the ones
// that answered. It returns what each probe found.
func (a *App) probeTools() map[string]probe {
	a.probeMu.Lock()
	defer a.probeMu.Unlock()
	if a.probed {
		return a.probes
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	probes := map[string]probe{}
	offered := map[string]agent.Driver{}
	for name, d := range a.Tools {
		p, err := d.Probe(ctx)
		pr := probe{Name: name, Tool: d.Name(), Version: p.Version, Account: p.Account,
			ShellLogin: p.ShellLogin, state: toolState(p, err)}
		if err != nil {
			pr.Error = err.Error()
		} else {
			offered[name] = d
		}
		probes[name] = pr
	}
	if a.Runs != nil {
		a.Runs.SetDrivers(offered, pickTool(offered, probes))
	}
	a.probes, a.probed = probes, true
	return probes
}

// pickTool is the tool a reviewer that names none runs on, chosen from the
// ones offered to the run manager (D63). A signed-in tool beats a signed-out
// one, because a run on a signed-out tool starts and then dies at
// authentication, and a tie goes to the first by name.
//
// The order matters more than which tool wins it. Ranging a map here would
// pick a different tool on each start, so the same preset would run on a
// different tool each time with nothing in the window to say why.
//
// vuoro has no opinion about whether one agent tool reviews better than
// another, and a ranking written here would be a preference nobody chose.
// Someone who cares names the tool in the preset.
func pickTool(offered map[string]agent.Driver, probes map[string]probe) string {
	names := make([]string, 0, len(offered))
	for name := range offered {
		names = append(names, name)
	}
	slices.SortFunc(names, func(a, b string) int {
		if x, y := probes[a].state == "ok", probes[b].state == "ok"; x != y {
			if x {
				return -1
			}
			return 1
		}
		return strings.Compare(a, b)
	})
	if len(names) == 0 {
		return ""
	}
	return names[0]
}

// reprobeTools forgets what the probes found, after the person installed or
// signed in to a tool, and probes again.
func (a *App) reprobeTools() map[string]probe {
	a.probeMu.Lock()
	a.probed = false
	a.probeMu.Unlock()
	return a.probeTools()
}

func (a *App) presets() []config.Preset { return presetsOf(a.config()) }

// presetsOf is the person's presets followed by the ones vuoro ships, as
// profiles already compose (D61). One of theirs shadows a built-in of the
// same name rather than appearing beside it.
//
// Theirs come first so that the window's default, which is the first preset,
// stays the one they wrote. The clone matters: appending to c.Presets itself
// would write the built-ins into the configuration's own backing array.
func presetsOf(c *config.Config) []config.Preset {
	var own []config.Preset
	if c != nil {
		own = c.Presets
	}
	theirs := make(map[string]bool, len(own))
	for _, p := range own {
		theirs[p.Name] = true
	}
	out := slices.Clone(own)
	for _, p := range config.Builtin {
		if !theirs[p.Name] {
			out = append(out, p)
		}
	}
	return out
}

// forgeFor returns the configured forge for host, or nil.
func (a *App) forgeFor(host string) *config.Forge {
	// One snapshot of the file: a save between two reads could shorten or
	// reorder the list.
	forges := a.config().Forges
	for i, f := range forges {
		if u, err := url.Parse(f.URL); err == nil && strings.EqualFold(u.Host, host) {
			return &forges[i]
		}
	}
	return nil
}

// client returns the client for host, resolving the token once per launch.
func (a *App) client(ctx context.Context, host, kind string) (forge.Provider, error) {
	a.clientMu.Lock()
	defer a.clientMu.Unlock()
	key := kind + ":" + host
	a.mu.Lock()
	c := a.clients[key]
	a.mu.Unlock()
	if c != nil {
		return c, nil
	}
	var token config.Secret
	base := "https://" + host
	if f := a.forgeFor(host); f != nil {
		base = f.URL
		t, err := f.Token(ctx)
		if err != nil {
			return nil, err
		}
		token = t
	}
	dial := a.Dial
	if dial == nil {
		dial = adapters.New
	}
	c, err := dial(kind, base, token)
	if err != nil {
		return nil, err
	}
	// A forge that cannot keep pending comments privately keeps them in
	// vuoro's state (D43). Without a state directory it offers no drafts.
	if ld, ok := c.(forge.LocalDrafter); ok && a.StateDir != "" {
		ld.KeepDraftsIn(localDrafts{dir: filepath.Join(a.StateDir, "drafts")})
	}
	a.mu.Lock()
	if a.clients == nil {
		a.clients = map[string]forge.Provider{}
	}
	a.clients[key] = c
	a.mu.Unlock()
	return c, nil
}

func (a *App) open(ctx context.Context, raw string) (*forge.Change, error) {
	u, err := forge.ChangeURL(raw)
	if err != nil {
		return nil, err
	}
	kind := ""
	if f := a.forgeFor(u.Host); f != nil {
		kind = f.Kind
	}
	ref, kind, err := adapters.ParseURL(raw, kind)
	if err != nil {
		return nil, err
	}
	c, err := a.client(ctx, ref.Host, kind)
	if err != nil {
		return nil, err
	}
	ch, err := c.Open(ctx, ref)
	if errors.Is(err, forge.ErrNotFound) && a.forgeFor(ref.Host) == nil {
		return nil, fmt.Errorf("%w; it was read anonymously, because the config file names no forge for %s, so only public projects open", err, ref.Host)
	}
	if err != nil {
		return nil, err
	}
	// Keep configured automated accounts out of human-thread interactions.
	if f := a.forgeFor(ref.Host); f != nil && len(f.Bots) > 0 {
		copyChange := *ch
		copyChange.Threads = append([]forge.Thread(nil), ch.Threads...)
		for i := range copyChange.Threads {
			copyChange.Threads[i].Notes = append([]forge.Note(nil), ch.Threads[i].Notes...)
			for j := range copyChange.Threads[i].Notes {
				n := &copyChange.Threads[i].Notes[j]
				for _, bot := range f.Bots {
					if strings.EqualFold(n.Author.Username, bot) {
						n.Author.Bot = true
					}
				}
			}
		}
		ch = &copyChange
	}
	ch.Capabilities = c.Capabilities()
	a.mu.Lock()
	if a.changes == nil {
		a.changes = map[string]*forge.Change{}
	}
	a.changes[ref.String()] = ch
	a.mu.Unlock()
	return ch, nil
}

func (a *App) openChange(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	ch, err := a.open(r.Context(), req.URL)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ch)
}

// source fetches the adapter-supplied change ref and target branch over the
// person's configured git transport.
func (a *App) source(ch *forge.Change) (review.Source, error) {
	t := ch.TargetBranch
	if !forge.ValidBranch(t) {
		return review.Source{}, fmt.Errorf("the target branch cannot be fetched safely")
	}
	if ch.Diff.Base == "" || ch.Diff.Head == "" {
		return review.Source{}, fmt.Errorf("the forge did not supply the base and head commits for this review")
	}
	meta := ch.Checkout
	if err := forge.ValidateCheckout(meta, ch.Ref.Host); err != nil {
		return review.Source{}, err
	}
	remote := meta.HTTPS
	if f := a.forgeFor(ch.Ref.Host); f != nil && f.Clone != "https" {
		if meta.SSH == "" {
			return review.Source{}, fmt.Errorf("this forge supplies no SSH clone URL; select https in the config")
		}
		remote = meta.SSH
	}
	// The key names the clone's directory, so a port's colon, which Windows
	// refuses in a path, becomes an underscore, which no host name holds.
	key := strings.ReplaceAll(ch.Ref.Host, ":", "_") + "/" + ch.Ref.Project
	return review.Source{Remote: remote, Key: key,
		Refspecs: []string{"+" + meta.HeadRef + ":refs/vuoro/head", "+refs/heads/" + t + ":refs/vuoro/target/" + t}, Head: ch.Diff.Head, Base: ch.Diff.Base}, nil
}

// Plan resolves who a run starts: the reviewers given, or else the named
// preset's, or the first preset's when name is empty. More than one reviewer
// gets a synthesizer (D11), and every run gets a verifier (D31), by default on
// the model that merged the findings, or else the one that drafted them.
func (a *App) Plan(name string, reviewers []review.Reviewer, synth, verifier *review.Reviewer) ([]review.Reviewer, *review.Reviewer, *review.Reviewer, error) {
	if len(reviewers) == 0 {
		ps := a.presets()
		if name == "" {
			name = ps[0].Name
		}
		for _, p := range ps {
			if p.Name == name {
				for _, rv := range p.Reviewers {
					reviewers = append(reviewers, review.Reviewer(rv))
				}
				if p.Synthesizer != nil {
					s := review.Reviewer(*p.Synthesizer)
					synth = &s
				}
				if p.Verifier != nil {
					v := review.Reviewer(*p.Verifier)
					verifier = &v
				}
			}
		}
		if len(reviewers) == 0 {
			return nil, nil, nil, fmt.Errorf("no preset named %q", name)
		}
	}
	if len(reviewers) > 1 && synth == nil {
		synth = &review.Reviewer{Tool: reviewers[0].Tool}
	}
	if len(reviewers) < 2 {
		synth = nil
	}
	if verifier == nil {
		from := reviewers[0]
		if synth != nil {
			from = *synth
		}
		verifier = &review.Reviewer{Tool: from.Tool, Model: from.Model}
	}
	return reviewers, synth, verifier, nil
}

// Open reads a change by its URL, as the window opens one.
func (a *App) Open(ctx context.Context, raw string) (*forge.Change, error) { return a.open(ctx, raw) }

// Source is what a review of ch fetches.
func (a *App) Source(ch *forge.Change) (review.Source, error) { return a.source(ch) }

// ProbeTools finds which agent tools can run reviewers, once.
func (a *App) ProbeTools() { a.probeTools() }

func (a *App) startRun(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL         string            `json:"url"`
		Preset      string            `json:"preset,omitempty"`
		Reviewers   []review.Reviewer `json:"reviewers,omitempty"`
		Synthesizer *review.Reviewer  `json:"synthesizer,omitempty"`
		Verifier    *review.Reviewer  `json:"verifier,omitempty"`
		Follows     string            `json:"follows,omitempty"` // an earlier review of the change (D50)
	}
	if !readJSON(w, r, &req) {
		return
	}
	a.probeTools()
	reviewers, synth, verifier, err := a.Plan(req.Preset, req.Reviewers, req.Synthesizer, req.Verifier)
	if err != nil {
		writeError(w, err)
		return
	}
	// Review what the forge says now, so the run is pinned to today's head.
	ch, err := a.open(r.Context(), req.URL)
	if err != nil {
		writeError(w, err)
		return
	}
	src, err := a.source(ch)
	if err != nil {
		writeError(w, err)
		return
	}
	run, err := a.Runs.Follow(ch, src, reviewers, synth, verifier, req.Follows)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, run.View())
}

func (a *App) listRuns(w http.ResponseWriter, r *http.Request) {
	out := []review.RunView{}
	for _, run := range a.Runs.Runs() {
		if run.Mode == review.ProjectMode || run.Mode == review.UpdateMode || run.Mode == review.AskMode {
			continue // a project's triage run shows on its project, an update or a conversation on its change
		}
		v := run.View()
		for i := range v.Sessions {
			v.Sessions[i].Log = nil // the list is a summary
		}
		out = append(out, v)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *App) getRun(w http.ResponseWriter, r *http.Request) {
	run := a.Runs.Get(r.PathValue("id"))
	if run == nil {
		writeError(w, review.ErrNoRun)
		return
	}
	writeJSON(w, http.StatusOK, run.View())
}

func (a *App) closeRun(w http.ResponseWriter, r *http.Request) {
	if err := a.Runs.Close(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	a.getRun(w, r)
}

// dismiss sets a finding of the person's own review aside (D42). It is local
// state, and nothing reaches the forge.
func (a *App) dismiss(w http.ResponseWriter, r *http.Request) {
	if err := a.Runs.Decide(r.PathValue("id"), r.PathValue("did"), drafts.Dropped, "", false); err != nil {
		writeError(w, err)
		return
	}
	a.getRun(w, r)
}

// askSuggestion asks the session that drafted a finding for a suggestion
// block, at the person's click, and answers with the run.
func (a *App) askSuggestion(w http.ResponseWriter, r *http.Request) {
	a.probeTools() // a run reloaded after a restart resumes on a probed tool
	if err := a.Runs.AskSuggestion(r.PathValue("id"), r.PathValue("did")); err != nil {
		writeError(w, err)
		return
	}
	a.getRun(w, r)
}

// report writes the run's hand-off report of kept findings to the run's
// directory, and answers with its text and where it is.
func (a *App) report(w http.ResponseWriter, r *http.Request) {
	text, path, err := a.Runs.WriteReport(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"markdown": text, "path": path})
}

// overrideConfidence records the person's own mark on a finding, which wins
// over the verifier's, or takes it back with an empty level. It changes
// vuoro's state only, never the forge.
func (a *App) overrideConfidence(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Level  drafts.Level `json:"level"`
		Reason string       `json:"reason,omitempty"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if err := a.Runs.Override(r.PathValue("id"), r.PathValue("did"), req.Level, req.Reason); err != nil {
		writeError(w, err)
		return
	}
	a.getRun(w, r)
}

func (a *App) say(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text string `json:"text"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Text) == "" {
		writeError(w, errors.New("say something first"))
		return
	}
	a.probeTools() // a run reloaded after a restart resumes on a probed tool
	a.refreshAsk(r.Context(), r.PathValue("id"))
	if err := a.Runs.Say(r.PathValue("id"), r.PathValue("sid"), req.Text); err != nil {
		writeError(w, err)
		return
	}
	a.getRun(w, r)
}

func (a *App) stop(w http.ResponseWriter, r *http.Request) {
	if err := a.Runs.Stop(r.PathValue("id"), r.PathValue("sid")); err != nil {
		writeError(w, err)
		return
	}
	a.getRun(w, r)
}

const maxRequest = 1 << 20

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(io.LimitReader(r.Body, maxRequest))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "the request is not the JSON this route takes"})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError reports err to the page. Every error that reaches here is
// already free of tokens: the forge client scrubs its own, and the config
// package never quotes a token command's output.
func writeError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	var api *forge.APIError
	var asked *review.AskedError
	switch {
	case errors.As(err, &asked):
		status = http.StatusConflict
	case errors.Is(err, review.ErrNoRun), errors.Is(err, review.ErrNoPatch), errors.Is(err, forge.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, forge.ErrUnsupported):
		status = http.StatusUnprocessableEntity
	case errors.As(err, &api):
		status = http.StatusBadGateway
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
