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
	"strings"
	"sync"
	"time"

	"github.com/terva-sh/vuoro/agent"
	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/forge/adapters"
	"github.com/terva-sh/vuoro/inbox"
	"github.com/terva-sh/vuoro/review"
	"github.com/terva-sh/vuoro/review/drafts"
)

// App holds what the API serves.
type App struct {
	// Config is the person's configuration. It is empty when there is no
	// file, and then a change opens anonymously, which public projects allow.
	Config *config.Config
	// ConfigErr is why the configuration did not load, shown to the person.
	ConfigErr error
	Runs      *review.Manager
	// Tools are the agent tools vuoro knows, by name. Each is probed once,
	// and only a tool that answers is offered to a run.
	Tools map[string]agent.Driver
	// Dial makes the client for a host. Nil means https on that host; a
	// test points it at a fake forge.
	Dial func(kind, base string, token config.Secret) (forge.Provider, error)
	// StateDir keeps the person's UI choices, in prefs.json. Empty keeps
	// none, and every action asks first.
	StateDir string

	prefsMu   sync.Mutex
	probeOnce sync.Once
	probes    map[string]probe

	clientMu     sync.Mutex
	mu           sync.Mutex
	clients      map[string]forge.Provider // by host and kind
	changes      map[string]*forge.Change  // by Ref.String(), the last read of each
	inboxOnce    sync.Once
	inboxService *inbox.Service
	inboxErr     error
}

type probe struct {
	Name    string `json:"name"`
	Tool    string `json:"tool"`
	Version string `json:"version,omitempty"`
	Account string `json:"account,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Handler returns the API's routes.
func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/setup", a.setup)
	mux.HandleFunc("GET /api/inbox", a.getInbox)
	mux.HandleFunc("POST /api/inbox/refresh", a.refreshInbox)
	mux.HandleFunc("POST /api/inbox/snooze", a.snoozeInbox)
	mux.HandleFunc("POST /api/prefs/confirm", a.setConfirm)
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
	mux.HandleFunc("POST /api/runs/{id}/close", a.closeRun)
	mux.HandleFunc("POST /api/runs/{id}/sessions/{sid}/messages", a.say)
	mux.HandleFunc("POST /api/runs/{id}/sessions/{sid}/stop", a.stop)
	a.writeRoutes(mux)
	return mux
}

func (a *App) config() *config.Config {
	if a.Config == nil {
		return &config.Config{}
	}
	return a.Config
}

// probeTools probes every tool once, and offers the run manager the ones
// that answered.
func (a *App) probeTools() {
	a.probeOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		a.probes = map[string]probe{}
		offered := map[string]agent.Driver{}
		for name, d := range a.Tools {
			p, err := d.Probe(ctx)
			pr := probe{Name: name, Tool: d.Name(), Version: p.Version, Account: p.Account}
			if err != nil {
				pr.Error = err.Error()
			} else {
				offered[name] = d
			}
			a.probes[name] = pr
		}
		if a.Runs != nil {
			a.Runs.Drivers = offered
		}
	})
}

func (a *App) presets() []config.Preset {
	if ps := a.config().Presets; len(ps) > 0 {
		return ps
	}
	return []config.Preset{{Name: "default", Reviewers: []config.Reviewer{{Profile: "code"}}}}
}

func (a *App) setup(w http.ResponseWriter, r *http.Request) {
	a.probeTools()
	type forgeView struct {
		Name     string `json:"name"`
		URL      string `json:"url"`
		Username string `json:"username"`
	}
	out := struct {
		ConfigError string           `json:"config_error,omitempty"`
		Forges      []forgeView      `json:"forges"`
		Tools       []probe          `json:"tools"`
		Presets     []config.Preset  `json:"presets"`
		Profiles    []review.Profile `json:"profiles"`
		Waived      []string         `json:"waived"` // confirmations the person turned off
	}{Forges: []forgeView{}, Tools: []probe{}, Presets: a.presets(), Profiles: review.Builtin, Waived: a.loadPrefs().Waived}
	if a.ConfigErr != nil {
		out.ConfigError = a.ConfigErr.Error()
	}
	for _, f := range a.config().Forges {
		out.Forges = append(out.Forges, forgeView{f.Name, f.URL, f.Username})
	}
	for _, p := range a.probes {
		out.Tools = append(out.Tools, p)
	}
	for _, p := range a.config().Profiles {
		out.Profiles = append(out.Profiles, review.Profile(p))
	}
	writeJSON(w, http.StatusOK, out)
}

// forgeFor returns the configured forge for host, or nil.
func (a *App) forgeFor(host string) *config.Forge {
	for i, f := range a.config().Forges {
		if u, err := url.Parse(f.URL); err == nil && strings.EqualFold(u.Host, host) {
			return &a.config().Forges[i]
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
	return review.Source{Remote: remote, Key: ch.Ref.Host + "/" + ch.Ref.Project,
		Refspecs: []string{"+" + meta.HeadRef + ":refs/vuoro/head", "+refs/heads/" + t + ":refs/vuoro/target/" + t}, Head: ch.Diff.Head, Base: ch.Diff.Base}, nil
}

func (a *App) startRun(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL         string            `json:"url"`
		Preset      string            `json:"preset,omitempty"`
		Reviewers   []review.Reviewer `json:"reviewers,omitempty"`
		Synthesizer *review.Reviewer  `json:"synthesizer,omitempty"`
		Verifier    *review.Reviewer  `json:"verifier,omitempty"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	a.probeTools()
	reviewers, synth, verifier := req.Reviewers, req.Synthesizer, req.Verifier
	if len(reviewers) == 0 {
		name := req.Preset
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
			writeError(w, fmt.Errorf("no preset named %q", name))
			return
		}
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
	// More than one reviewer means their drafts are merged (D11).
	if len(reviewers) > 1 && synth == nil {
		synth = &review.Reviewer{Tool: reviewers[0].Tool}
	}
	if len(reviewers) < 2 {
		synth = nil
	}
	// Every run checks its findings (D31), by default on the model that
	// merged them, or else the one that drafted them.
	if verifier == nil {
		from := reviewers[0]
		if synth != nil {
			from = *synth
		}
		verifier = &review.Reviewer{Tool: from.Tool, Model: from.Model}
	}
	run, err := a.Runs.Start(ch, src, reviewers, synth, verifier)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, run.View())
}

func (a *App) listRuns(w http.ResponseWriter, r *http.Request) {
	out := []review.RunView{}
	for _, run := range a.Runs.Runs() {
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
	switch {
	case errors.Is(err, review.ErrNoRun), errors.Is(err, forge.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, forge.ErrUnsupported):
		status = http.StatusUnprocessableEntity
	case errors.As(err, &api):
		status = http.StatusBadGateway
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
