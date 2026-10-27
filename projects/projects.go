// Package projects is project triage (docs/plan.md section 6.9): the projects
// the person added, a snapshot of every open change in each, and where each
// change sits. Like inbox, it works without agent tools, review runners, or
// ticket stores, and it starts no background refresh: the window asks.
package projects

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/inbox"
	"github.com/terva-sh/vuoro/state"
)

// MaxChanges bounds a project's open changes. A project with more is refused
// with its count rather than shown in part.
const MaxChanges = 300

// readWorkers bounds the concurrent change reads of one project.
const readWorkers = 6

// Entry is a project the person added.
type Entry struct {
	Key    string    `json:"key"`
	Kind   string    `json:"kind"`
	Host   string    `json:"host"`
	Path   string    `json:"path"`
	Name   string    `json:"name"`
	WebURL string    `json:"web_url"`
	Added  time.Time `json:"added"`
}

// Item is one open change in a project's snapshot.
type Item struct {
	Change forge.Change `json:"change"`
	// Error says, in a fixed credential-free message, why the last refresh
	// could not read this change. Its facts are then the last ones read.
	Error string `json:"error,omitempty"`
	// Unread is a listed change that has never been read.
	Unread bool `json:"unread,omitempty"`
}

// Snapshot is a project's open changes as last read.
type Snapshot struct {
	Project     Entry     `json:"project"`
	Items       []Item    `json:"items"`
	LastAttempt time.Time `json:"last_attempt"`
	LastSuccess time.Time `json:"last_success"`
	// Error is a fixed, credential-free message, never an adapter's raw error.
	Error string `json:"error,omitempty"`
	// Warning says that a successful refresh could not read some changes.
	Warning string `json:"warning,omitempty"`
	// Dependencies says whether the forge answered for every change's
	// dependencies. Without it, stacks come from target branches alone.
	Dependencies bool `json:"dependencies"`
}

// Found is one search result, with the account that found it.
type Found struct {
	forge.Project
	Kind    string `json:"kind"`
	Account string `json:"account"`
	// Added says the person already has this project.
	Added bool `json:"added"`
}

// SearchResult is what every account found, and the accounts that failed.
type SearchResult struct {
	Projects []Found  `json:"projects"`
	Errors   []string `json:"errors,omitempty"`
}

type Resolver func(context.Context, config.Forge) (forge.Provider, error)

// Service holds the person's projects and their snapshots.
type Service struct {
	dir      string
	accounts []config.Forge
	resolve  Resolver
	gate     chan struct{}

	mu        sync.Mutex
	entries   []Entry
	snapshots map[string]Snapshot
}

// New loads the person's projects and their cached snapshots. Empty dir keeps
// everything in memory.
func New(dir string, accounts []config.Forge, resolve Resolver) (*Service, error) {
	s := &Service{dir: dir, resolve: resolve, gate: make(chan struct{}, 1), snapshots: map[string]Snapshot{}}
	for _, a := range accounts {
		a.Bots = slices.Clone(a.Bots)
		s.accounts = append(s.accounts, a)
	}
	if dir == "" {
		return s, nil
	}
	if err := state.ReadJSON(s.listPath(), &s.entries); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, errors.New("projects: could not read the project list")
	}
	for _, e := range s.entries {
		var cached Snapshot
		if err := state.ReadJSON(s.cachePath(e.Key), &cached); err == nil && cached.Project.Key == e.Key {
			cached.Project = e
			s.snapshots[e.Key] = cached
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, errors.New("projects: could not read a cached project")
		}
	}
	return s, nil
}

func (s *Service) listPath() string { return filepath.Join(s.dir, "projects.json") }
func (s *Service) cachePath(key string) string {
	return filepath.Join(s.dir, "cache", "projects", key+".json")
}

// Key is a project's stable identity: its forge, host, and path, which GitLab
// matches without regard to case.
func Key(kind, host, path string) string {
	sum := sha256.Sum256([]byte(kind + "\x00" + strings.ToLower(host) + "\x00" + strings.ToLower(path)))
	return hex.EncodeToString(sum[:8])
}

// account is the configured account on host.
func (s *Service) account(host string) (config.Forge, bool) {
	for _, a := range s.accounts {
		if u, err := url.Parse(a.URL); err == nil && strings.EqualFold(u.Host, host) {
			return a, true
		}
	}
	return config.Forge{}, false
}

func (s *Service) lister(ctx context.Context, a config.Forge) (forge.Provider, forge.ProjectLister, error) {
	if s.resolve == nil {
		return nil, nil, forge.ErrUnsupported
	}
	p, err := s.resolve(ctx, a)
	if err != nil {
		return nil, nil, err
	}
	l, ok := p.(forge.ProjectLister)
	if !ok {
		return nil, nil, forge.ErrUnsupported
	}
	return p, l, nil
}

// Accounts are the configured accounts, for the window to say where it
// searches.
func (s *Service) Accounts() []config.Forge { return slices.Clone(s.accounts) }

// Search asks every configured account whose forge can list projects. One
// account failing still answers with the others' projects.
func (s *Service) Search(ctx context.Context, query string) SearchResult {
	out := SearchResult{Projects: []Found{}}
	added := map[string]bool{}
	for _, e := range s.Entries() {
		added[e.Key] = true
	}
	for _, a := range s.accounts {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		_, l, err := s.lister(ctx, a)
		var found []forge.Project
		if err == nil {
			found, err = l.SearchProjects(ctx, query)
		}
		cancel()
		if errors.Is(err, forge.ErrUnsupported) {
			continue
		}
		if err != nil {
			out.Errors = append(out.Errors, a.Name+": "+searchError(err))
			continue
		}
		for _, p := range found {
			out.Projects = append(out.Projects, Found{Project: p, Kind: a.Kind, Account: a.Name, Added: added[Key(a.Kind, p.Host, p.Path)]})
		}
	}
	return out
}

// Entries are the person's projects, in the order they added them.
func (s *Service) Entries() []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.entries)
}

// Add adds the project at path on host, once the account there confirms it can
// see it. Adding a project already added answers its entry.
func (s *Service) Add(ctx context.Context, host, path string) (Entry, error) {
	a, ok := s.account(host)
	if !ok {
		return Entry{}, fmt.Errorf("no forge account is configured for %s; add one in Settings", host)
	}
	_, l, err := s.lister(ctx, a)
	if err != nil {
		if errors.Is(err, forge.ErrUnsupported) {
			return Entry{}, errors.New("project triage is not available for this forge yet")
		}
		return Entry{}, err
	}
	p, err := l.Project(ctx, path)
	if err != nil {
		if errors.Is(err, forge.ErrNotFound) {
			return Entry{}, errors.New("the account cannot see that project")
		}
		return Entry{}, err
	}
	e := Entry{Key: Key(a.Kind, p.Host, p.Path), Kind: a.Kind, Host: p.Host, Path: p.Path, Name: p.Name, WebURL: p.WebURL, Added: time.Now().UTC()}
	s.mu.Lock()
	defer s.mu.Unlock()
	if i := slices.IndexFunc(s.entries, func(x Entry) bool { return x.Key == e.Key }); i >= 0 {
		return s.entries[i], nil
	}
	entries := append(slices.Clone(s.entries), e)
	if s.dir != "" {
		if err := state.WriteJSON(s.listPath(), entries); err != nil {
			return Entry{}, errors.New("could not save the project list")
		}
	}
	s.entries = entries
	s.snapshots[e.Key] = Snapshot{Project: e, Items: []Item{}}
	return e, nil
}

// Remove drops a project and its cached snapshot.
func (s *Service) Remove(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.entries, func(x Entry) bool { return x.Key == key })
	if i < 0 {
		return errors.New("that project is not in the list")
	}
	entries := slices.Delete(slices.Clone(s.entries), i, i+1)
	if s.dir != "" {
		if err := state.WriteJSON(s.listPath(), entries); err != nil {
			return errors.New("could not save the project list")
		}
		_ = os.Remove(s.cachePath(key))
	}
	s.entries = entries
	delete(s.snapshots, key)
	return nil
}

// Snapshot returns an independent copy of a project's snapshot, with the
// account's configured bots marked.
func (s *Service) Snapshot(key string) (Snapshot, bool) {
	s.mu.Lock()
	snap, ok := s.snapshots[key]
	snap = clone(snap)
	s.mu.Unlock()
	if !ok {
		return Snapshot{}, false
	}
	a, _ := s.account(snap.Project.Host)
	for i := range snap.Items {
		inbox.Classify(&snap.Items[i].Change, a.Bots)
	}
	return snap, true
}

// Refresh reads a project's open changes again. Refreshes are serialized, so
// a targeted read after a write cannot be overwritten by an older refresh. A
// failure keeps the last snapshot and its LastSuccess.
func (s *Service) Refresh(ctx context.Context, key string) (Snapshot, error) {
	select {
	case s.gate <- struct{}{}:
	case <-ctx.Done():
		return Snapshot{}, ctx.Err()
	}
	defer func() { <-s.gate }()
	s.mu.Lock()
	previous, ok := s.snapshots[key]
	previous = clone(previous)
	s.mu.Unlock()
	if !ok {
		return Snapshot{}, errors.New("that project is not in the list")
	}
	next := previous
	next.LastAttempt = time.Now().UTC()
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	items, deps, err := s.read(ctx, previous)
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		next.Error = safeError(err)
	} else {
		next.Items, next.Dependencies = items, deps
		next.LastSuccess = time.Now().UTC()
		next.Error, next.Warning = "", ""
		failed := 0
		for _, item := range items {
			if item.Error != "" {
				failed++
			}
		}
		if failed == 1 {
			next.Warning = "1 change could not be read; it shows its last known state."
		} else if failed > 1 {
			next.Warning = strconv.Itoa(failed) + " changes could not be read; they show their last known state."
		}
	}
	if err := s.save(next); err != nil {
		next = previous
		next.LastAttempt = time.Now().UTC()
		next.Error = "Could not save the project cache."
	}
	s.mu.Lock()
	if _, still := s.snapshots[key]; still {
		s.snapshots[key] = next
	}
	s.mu.Unlock()
	snap, _ := s.Snapshot(key)
	return snap, nil
}

func (s *Service) save(snap Snapshot) error {
	if s.dir == "" {
		return nil
	}
	return state.WriteJSON(s.cachePath(snap.Project.Key), snap)
}

// tooMany is a project with more open changes than MaxChanges.
type tooMany int

func (n tooMany) Error() string {
	return fmt.Sprintf("This project has %d open changes, and vuoro reads at most %d.", int(n), MaxChanges)
}

func (s *Service) read(ctx context.Context, previous Snapshot) ([]Item, bool, error) {
	a, ok := s.account(previous.Project.Host)
	if !ok {
		return nil, false, errNoAccount
	}
	p, l, err := s.lister(ctx, a)
	if err != nil {
		return nil, false, err
	}
	listed, err := l.ListOpen(ctx, previous.Project.Path)
	if err != nil {
		return nil, false, err
	}
	if len(listed) > MaxChanges {
		return nil, false, tooMany(len(listed))
	}
	for _, m := range listed {
		if !strings.EqualFold(m.Ref.Host, previous.Project.Host) || !strings.EqualFold(m.Ref.Project, previous.Project.Path) || m.Ref.Number < 1 {
			return nil, false, errors.New("invalid listed identity")
		}
	}
	cached := map[int]Item{}
	for _, item := range previous.Items {
		cached[item.Change.Ref.Number] = item
	}
	deps, depsOK := p.(forge.DependencyReader)
	results := make([]Item, len(listed))
	keep := make([]bool, len(listed))
	known := make([]bool, len(listed))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range min(readWorkers, len(listed)) {
		wg.Go(func() {
			for i := range jobs {
				results[i], keep[i] = readItem(ctx, p, a.Kind, listed[i].Ref, cached)
				if keep[i] && depsOK && results[i].Error == "" {
					blocked, err := deps.Dependencies(ctx, listed[i].Ref)
					if err == nil {
						results[i].Change.BlockedBy, results[i].Change.DependenciesKnown = blocked, true
						known[i] = true
					}
				}
			}
		})
	}
	for i := range listed {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	items := []Item{}
	all := depsOK
	for i, item := range results {
		if keep[i] {
			items = append(items, item)
			all = all && known[i]
		}
	}
	return items, all && len(items) > 0, nil
}

// readItem reads one listed change. It reports false for a change that closed
// after the listing, which leaves the snapshot.
func readItem(ctx context.Context, p forge.Provider, kind string, ref forge.Ref, cached map[int]Item) (Item, bool) {
	item, exists := cached[ref.Number]
	item.Error = ""
	var ch *forge.Change
	var err error
	if facts, ok := p.(forge.FactsReader); ok {
		ch, err = facts.ReadFacts(ctx, ref)
	} else {
		ch, err = p.Open(ctx, ref)
	}
	if err == nil && (ch == nil || ch.Ref.Number != ref.Number || ch.Kind != kind) {
		err = errors.New("invalid opened identity")
	}
	if err != nil {
		if !exists {
			item = Item{Change: forge.Change{Kind: kind, Ref: ref, State: forge.Open}, Unread: true}
		}
		item.Error = changeError(err)
		return item, true
	}
	if ch.State != forge.Open {
		return Item{}, false
	}
	item.Change = clone(*ch)
	item.Change.Files, item.Change.Drafts = nil, nil
	item.Unread = false
	return item, true
}

// RefreshChange reads one change again after the person's write, and updates
// every project that holds it. It does not advance LastSuccess.
func (s *Service) RefreshChange(ctx context.Context, ref forge.Ref) error {
	select {
	case s.gate <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-s.gate }()
	for _, e := range s.Entries() {
		if !strings.EqualFold(e.Host, ref.Host) || !strings.EqualFold(e.Path, ref.Project) {
			continue
		}
		s.mu.Lock()
		snap := clone(s.snapshots[e.Key])
		s.mu.Unlock()
		i := slices.IndexFunc(snap.Items, func(it Item) bool { return it.Change.Ref.Number == ref.Number })
		if i < 0 {
			continue
		}
		a, ok := s.account(e.Host)
		if !ok {
			return errNoAccount
		}
		ctx, cancel := context.WithTimeout(ctx, time.Minute)
		p, _, err := s.lister(ctx, a)
		if err != nil {
			cancel()
			return err
		}
		item, keep := readItem(ctx, p, a.Kind, snap.Items[i].Change.Ref, map[int]Item{ref.Number: snap.Items[i]})
		if keep && item.Error == "" {
			// A write never changes dependencies, so the last answer stands.
			item.Change.BlockedBy, item.Change.DependenciesKnown = snap.Items[i].Change.BlockedBy, snap.Items[i].Change.DependenciesKnown
		}
		cancel()
		if keep {
			snap.Items[i] = item
		} else {
			snap.Items = slices.Delete(snap.Items, i, i+1)
		}
		if err := s.save(snap); err != nil {
			return errors.New("projects: could not save the refreshed change")
		}
		s.mu.Lock()
		if _, still := s.snapshots[e.Key]; still {
			s.snapshots[e.Key] = snap
		}
		s.mu.Unlock()
	}
	return nil
}

var errNoAccount = errors.New("no account")

// searchError is the fixed message for an account whose search failed.
func searchError(err error) string {
	var api *forge.APIError
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "the search timed out."
	case errors.As(err, &api) && (api.Status == 401 || api.Status == 403):
		return "the account could not search; check access and credentials."
	case errors.As(err, &api) && api.Status == 429:
		return "the forge rate limited the search; try again later."
	}
	return "the search failed."
}

// changeError is the fixed message for one change that could not be read.
func changeError(err error) string {
	var api *forge.APIError
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "Reading this change timed out."
	case errors.Is(err, forge.ErrNotFound):
		return "The forge no longer shows this change to the account."
	case errors.As(err, &api) && (api.Status == 401 || api.Status == 403):
		return "The account may not read this change."
	case errors.As(err, &api) && api.Status == 429:
		return "The forge rate limited reading this change."
	}
	return "The forge's answer for this change could not be read."
}

func safeError(err error) string {
	var api *forge.APIError
	var many tooMany
	switch {
	case errors.As(err, &many):
		return many.Error()
	case errors.Is(err, errNoAccount):
		return "No forge account is configured for this project's host; add one in Settings."
	case errors.Is(err, context.Canceled):
		return "Refresh cancelled; showing the previous snapshot."
	case errors.Is(err, context.DeadlineExceeded):
		return "Refresh timed out; showing the previous snapshot."
	case errors.Is(err, forge.ErrUnsupported):
		return "Project triage is not available for this forge yet."
	case errors.Is(err, forge.ErrNotFound):
		return "The account can no longer see this project."
	case errors.As(err, &api):
		switch api.Status {
		case 401, 403:
			return "The account could not read this project; check access and credentials."
		case 429:
			return "The forge rate limited refresh; try again later."
		}
	}
	return "Could not refresh this project; showing the previous snapshot."
}
