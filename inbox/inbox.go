// Package inbox owns account-scoped discovery and derived cache refresh. It
// works without agent tools, review runners, or ticket stores. Refresh is called
// by the window; this package starts no background polling.
package inbox

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/state"
)

type Item struct {
	Change forge.Change `json:"change"`
	Roles  []forge.Role `json:"roles"`
}

type AccountSnapshot struct {
	Key         string    `json:"key"`
	Name        string    `json:"name"`
	Kind        string    `json:"kind"`
	Host        string    `json:"host"`
	Username    string    `json:"username"`
	Items       []Item    `json:"items"`
	LastAttempt time.Time `json:"last_attempt"`
	LastSuccess time.Time `json:"last_success"`
	// Error is a fixed, credential-free message, never an adapter's raw error.
	Error string `json:"error,omitempty"`
}

type Annotation struct {
	SnoozedUntil time.Time `json:"snoozed_until"`
}

type View struct {
	Accounts    []AccountSnapshot     `json:"accounts"`
	Annotations map[string]Annotation `json:"annotations"`
}

type Resolver func(context.Context, config.Forge) (forge.Provider, error)

type Service struct {
	dir         string
	accounts    []config.Forge
	resolve     Resolver
	gate        chan struct{}
	mu          sync.Mutex
	snapshots   []AccountSnapshot
	annotations map[string]Annotation
}

// New loads only the configured accounts' snapshots. Removed accounts never
// appear in the view. Empty dir keeps state in memory only.
func New(dir string, accounts []config.Forge, resolve Resolver) (*Service, error) {
	s := &Service{dir: dir, resolve: resolve, gate: make(chan struct{}, 1), annotations: map[string]Annotation{}}
	for _, a := range accounts {
		u, err := url.Parse(a.URL)
		if err != nil || u.Host == "" || a.Username == "" || a.Kind == "" {
			return nil, errors.New("inbox: invalid account")
		}
		a.Bots = append([]string(nil), a.Bots...)
		s.accounts = append(s.accounts, a)
		host := strings.ToLower(u.Host)
		sum := sha256.Sum256([]byte(a.Kind + "\x00" + host + "\x00" + a.Username))
		key := hex.EncodeToString(sum[:])
		snapshot := AccountSnapshot{Key: key, Name: a.Name, Kind: a.Kind, Host: host, Username: a.Username, Items: []Item{}}
		if dir != "" {
			var cached AccountSnapshot
			if err := state.ReadJSON(s.cachePath(key), &cached); err == nil {
				if cached.Key != key || cached.Host != host || cached.Kind != a.Kind || cached.Username != a.Username {
					return nil, errors.New("inbox: cache account mismatch")
				}
				snapshot = cached
				snapshot.Name = a.Name
			} else if !errors.Is(err, os.ErrNotExist) {
				return nil, errors.New("inbox: could not read cached account")
			}
		}
		s.snapshots = append(s.snapshots, snapshot)
	}
	if dir != "" {
		if err := state.ReadJSON(s.annotationPath(), &s.annotations); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, errors.New("inbox: could not read annotations")
		}
		if s.annotations == nil {
			s.annotations = map[string]Annotation{}
		}
	}
	return s, nil
}

func (s *Service) cachePath(key string) string {
	return filepath.Join(s.dir, "cache", "inbox", key+".json")
}
func (s *Service) annotationPath() string { return filepath.Join(s.dir, "annotations", "inbox.json") }

// Snapshot returns an independent copy, so neither the UI nor a caller can
// mutate cached facts while another account is refreshing.
func (s *Service) Snapshot() View {
	s.mu.Lock()
	defer s.mu.Unlock()
	view := clone(View{Accounts: s.snapshots, Annotations: s.annotations})
	for i := range view.Accounts {
		for j := range view.Accounts[i].Items {
			classify(&view.Accounts[i].Items[j].Change, s.accounts[i].Bots)
		}
	}
	return view
}

// Refresh serializes refreshes and bounds account work to four workers and two
// minutes per account. All provider calls receive the caller's cancellation.
// A failed account preserves its previous items and LastSuccess.
func (s *Service) Refresh(ctx context.Context) View {
	select {
	case s.gate <- struct{}{}:
	case <-ctx.Done():
		return s.Snapshot()
	}
	defer func() { <-s.gate }()
	var wg sync.WaitGroup
	jobs := make(chan int)
	for range min(4, len(s.accounts)) {
		wg.Go(func() {
			for i := range jobs {
				s.refreshAccount(ctx, i)
			}
		})
	}
	for i := range s.accounts {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return s.Snapshot()
}

func (s *Service) refreshAccount(ctx context.Context, i int) {
	s.mu.Lock()
	previous := clone(s.snapshots[i])
	s.mu.Unlock()
	next := previous
	next.LastAttempt = time.Now().UTC()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	items, err := s.readAccount(ctx, s.accounts[i], previous)
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		next.Error = safeError(err)
	} else {
		next.Items = items
		next.LastSuccess = time.Now().UTC()
		next.Error = ""
	}
	if s.dir != "" {
		if err := state.WriteJSON(s.cachePath(next.Key), next); err != nil {
			next = previous
			next.LastAttempt = time.Now().UTC()
			next.Error = "Could not save the inbox cache."
		}
	}
	s.mu.Lock()
	s.snapshots[i] = next
	s.mu.Unlock()
}

func (s *Service) readAccount(ctx context.Context, a config.Forge, previous AccountSnapshot) ([]Item, error) {
	if s.resolve == nil {
		return nil, forge.ErrUnsupported
	}
	p, err := s.resolve(ctx, a)
	if err != nil {
		return nil, err
	}
	d, ok := p.(forge.Discoverer)
	if !ok {
		return nil, forge.ErrUnsupported
	}
	discovered, err := d.Discover(ctx, a.Username)
	if err != nil {
		return nil, err
	}
	// Merge roles before reading a change, irrespective of which query found it.
	members := map[string]forge.Discovered{}
	for _, m := range discovered {
		if !strings.EqualFold(m.Ref.Host, previous.Host) || m.Ref.Project == "" || m.Ref.Number < 1 {
			return nil, errors.New("invalid discovered identity")
		}
		m.Ref.Host = previous.Host
		key := m.Ref.String()
		old := members[key]
		m.Roles = uniqueRoles(append(old.Roles, m.Roles...))
		missingStamp := old.Ref.Number != 0 && (old.Updated.IsZero() || m.Updated.IsZero())
		if old.Updated.After(m.Updated) {
			m.Updated = old.Updated
		}
		// Any missing stamp makes reuse unsafe, including one duplicate query.
		if missingStamp {
			m.Updated = time.Time{}
		}
		members[key] = m
	}
	cached := map[string]Item{}
	for _, item := range previous.Items {
		cached[item.Change.Ref.String()] = item
	}
	keys := make([]string, 0, len(members))
	for key := range members {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	items := make([]Item, 0, len(keys))
	for _, key := range keys {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		m := members[key]
		item, exists := cached[key]
		if !exists || m.Updated.IsZero() || item.Change.Updated.IsZero() || !m.Updated.Equal(item.Change.Updated) {
			ch, err := p.Open(ctx, m.Ref)
			if err != nil {
				return nil, err
			}
			if ch == nil || ch.Ref != m.Ref || ch.Kind != a.Kind {
				return nil, errors.New("invalid opened identity")
			}
			item.Change = *ch
		}
		// A change may close between discovery and reading; do not keep it open.
		if item.Change.State != forge.Open {
			continue
		}
		item.Roles = m.Roles
		item.Change.Capabilities = p.Capabilities()
		items = append(items, item)
	}
	return items, nil
}

func uniqueRoles(roles []forge.Role) []forge.Role {
	out := []forge.Role{}
	for _, r := range []forge.Role{forge.AuthorRole, forge.ReviewerRole, forge.AssigneeRole, forge.ParticipantRole} {
		for _, v := range roles {
			if v == r {
				out = append(out, r)
				break
			}
		}
	}
	return out
}

func safeError(err error) string {
	var api *forge.APIError
	switch {
	case errors.Is(err, context.Canceled):
		return "Refresh cancelled; showing the previous snapshot."
	case errors.Is(err, context.DeadlineExceeded):
		return "Refresh timed out; showing the previous snapshot."
	case errors.Is(err, forge.ErrUnsupported):
		return "Inbox discovery is not available for this account."
	case errors.As(err, &api):
		switch api.Status {
		case 401, 403:
			return "The account could not read its changes; check access and credentials."
		case 429:
			return "The forge rate limited refresh; try again later."
		}
	}
	return "Could not refresh this account; showing the previous snapshot."
}

// Snooze changes only authored state. Zero until removes a snooze. Annotations
// are saved before publication in memory, so a failed save never looks durable.
func (s *Service) Snooze(ref forge.Ref, until time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	annotations := clone(s.annotations)
	if until.IsZero() {
		delete(annotations, ref.String())
	} else {
		annotations[ref.String()] = Annotation{SnoozedUntil: until.UTC()}
	}
	if s.dir != "" {
		if err := state.WriteJSON(s.annotationPath(), annotations); err != nil {
			return errors.New("inbox: could not save snooze")
		}
	}
	s.annotations = annotations
	return nil
}
