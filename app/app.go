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
	"github.com/terva-sh/vuoro/forge/gitlab"
	"github.com/terva-sh/vuoro/review"
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

	probeOnce sync.Once
	probes    map[string]probe

	mu      sync.Mutex
	clients map[string]*gitlab.Client // by host
	changes map[string]*forge.Change  // by Ref.String(), the last read of each
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
	mux.HandleFunc("POST /api/changes", a.openChange)
	mux.HandleFunc("GET /api/runs", a.listRuns)
	mux.HandleFunc("POST /api/runs", a.startRun)
	mux.HandleFunc("GET /api/runs/{id}", a.getRun)
	mux.HandleFunc("POST /api/runs/{id}/close", a.closeRun)
	mux.HandleFunc("POST /api/runs/{id}/sessions/{sid}/messages", a.say)
	mux.HandleFunc("POST /api/runs/{id}/sessions/{sid}/stop", a.stop)
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
		a.Runs.Drivers = offered
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
	}{Forges: []forgeView{}, Tools: []probe{}, Presets: a.presets(), Profiles: review.Builtin}
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
		if u, err := url.Parse(f.URL); err == nil && u.Host == host {
			return &a.config().Forges[i]
		}
	}
	return nil
}

// client returns the client for host, resolving the token once per launch.
func (a *App) client(ctx context.Context, host string) (*gitlab.Client, error) {
	a.mu.Lock()
	c := a.clients[host]
	a.mu.Unlock()
	if c != nil {
		return c, nil
	}
	var token config.Secret
	if f := a.forgeFor(host); f != nil {
		t, err := f.Token(ctx)
		if err != nil {
			return nil, err
		}
		token = t
	}
	c, err := gitlab.New("https://"+host, token)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	if a.clients == nil {
		a.clients = map[string]*gitlab.Client{}
	}
	a.clients[host] = c
	a.mu.Unlock()
	return c, nil
}

func (a *App) open(ctx context.Context, raw string) (*forge.Change, error) {
	ref, err := forge.ParseGitLabURL(raw)
	if err != nil {
		return nil, err
	}
	c, err := a.client(ctx, ref.Host)
	if err != nil {
		return nil, err
	}
	ch, err := c.Open(ctx, ref)
	if err != nil {
		return nil, err
	}
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

// source is where a review fetches ch from: the project over the person's
// configured git transport, its merge request head, and its target branch.
func (a *App) source(ch *forge.Change) (review.Source, error) {
	t := ch.TargetBranch
	if t == "" || strings.ContainsAny(t, ": \t\n*?[\\^~") || strings.HasPrefix(t, "-") || strings.Contains(t, "..") {
		return review.Source{}, fmt.Errorf("the target branch name %q cannot be fetched safely", t)
	}
	remote := "https://" + ch.Ref.Host + "/" + ch.Ref.Project + ".git"
	if f := a.forgeFor(ch.Ref.Host); f != nil && f.Clone != "https" {
		remote = "git@" + ch.Ref.Host + ":" + ch.Ref.Project + ".git"
	}
	return review.Source{
		Remote: remote,
		Key:    ch.Ref.Host + "/" + ch.Ref.Project,
		Refspecs: []string{
			fmt.Sprintf("+refs/merge-requests/%d/head:refs/vuoro/mr/%d", ch.Ref.Number, ch.Ref.Number),
			"+refs/heads/" + t + ":refs/vuoro/target/" + t,
		},
		Head: ch.Diff.Head,
		Base: ch.Diff.Base,
	}, nil
}

func (a *App) startRun(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL       string            `json:"url"`
		Preset    string            `json:"preset,omitempty"`
		Reviewers []review.Reviewer `json:"reviewers,omitempty"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	a.probeTools()
	reviewers := req.Reviewers
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
	run, err := a.Runs.Start(ch, src, reviewers)
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
	var api *gitlab.APIError
	switch {
	case errors.Is(err, review.ErrNoRun), errors.Is(err, gitlab.ErrNotFound):
		status = http.StatusNotFound
	case errors.As(err, &api):
		status = http.StatusBadGateway
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
