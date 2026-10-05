package inbox

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
)

type fixture struct {
	members []forge.Discovered
	changes map[string]*forge.Change
	err     error
	wait    bool
	opens   atomic.Int32
	// failing makes reading one change fail; delay slows every read.
	failing map[string]error
	delay   time.Duration
	live    atomic.Int32
	peak    atomic.Int32
}

func (f *fixture) Capabilities() forge.Capabilities { return forge.Capabilities{} }
func (f *fixture) Discover(ctx context.Context, _ string) ([]forge.Discovered, error) {
	if f.wait {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return f.members, f.err
}
func (f *fixture) Open(_ context.Context, r forge.Ref) (*forge.Change, error) {
	f.opens.Add(1)
	n := f.live.Add(1)
	defer f.live.Add(-1)
	for p := f.peak.Load(); n > p && !f.peak.CompareAndSwap(p, n); p = f.peak.Load() {
	}
	time.Sleep(f.delay)
	if err := f.failing[r.String()]; err != nil {
		return nil, err
	}
	return f.changes[r.String()], nil
}

func account(host, kind string) config.Forge {
	return config.Forge{Name: host, Kind: kind, URL: "https://" + host, Username: "me", Bots: []string{"robot"}}
}
func provider(a config.Forge) *fixture {
	host := strings.TrimPrefix(a.URL, "https://")
	r := forge.Ref{Host: host, Project: "team/repo", Number: 1}
	at := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	ch := &forge.Change{Kind: a.Kind, Ref: r, Updated: at, State: forge.Open, Author: forge.User{Username: "author"}, Reviewers: []forge.Reviewer{{User: forge.User{Username: "robot"}}, {User: forge.User{Username: "native-bot"}}}, Reviews: []forge.Review{{Reviewer: "robot"}, {Reviewer: "native-bot"}}, Threads: []forge.Thread{{Notes: []forge.Note{{Author: forge.User{Username: "robot"}}, {Author: forge.User{Username: "native-bot", Bot: true}}}}}}
	return &fixture{members: []forge.Discovered{{Ref: r, Roles: []forge.Role{forge.ReviewerRole}, Updated: at}, {Ref: r, Roles: []forge.Role{forge.AssigneeRole, forge.ReviewerRole}, Updated: at}}, changes: map[string]*forge.Change{r.String(): ch}}
}

func TestMixedAccountsRefreshAndFailure(t *testing.T) {
	as := []config.Forge{account("lab.test", forge.GitLab), account("jo.test", forge.Forgejo)}
	ps := map[string]*fixture{}
	for _, a := range as {
		ps[a.Name] = provider(a)
	}
	dir := t.TempDir()
	resolve := func(_ context.Context, a config.Forge) (forge.Provider, error) { return ps[a.Name], nil }
	s, err := New(dir, as, resolve)
	if err != nil {
		t.Fatal(err)
	}
	view := s.Refresh(context.Background())
	for _, a := range view.Accounts {
		if a.Error != "" || a.LastSuccess.IsZero() || len(a.Items) != 1 {
			t.Fatalf("snapshot: %+v", a)
		}
		item := a.Items[0]
		if len(item.Roles) != 2 || item.Change.Ref.String() != a.Host+"/team/repo!1" {
			t.Fatalf("identity/roles: %+v", item)
		}
		if !item.Change.Reviewers[0].Bot || !item.Change.Reviewers[1].Bot || !item.Change.Reviews[0].Bot || !item.Change.Reviews[1].Bot || !item.Change.Threads[0].Notes[0].Author.Bot {
			t.Fatal("bots not classified")
		}
	}
	ref := view.Accounts[0].Items[0].Change.Ref
	until := time.Now().Add(time.Hour)
	if err := s.Snooze(ref, until); err != nil {
		t.Fatal(err)
	}
	before := view.Accounts[0].LastSuccess
	ps[as[0].Name].err = &forge.APIError{Status: 429, Message: "sensitive-value"}
	ps[as[1].Name].members = nil
	view = s.Refresh(context.Background())
	if len(view.Accounts[0].Items) != 1 || !view.Accounts[0].LastSuccess.Equal(before) || !strings.Contains(view.Accounts[0].Error, "rate limited") || strings.Contains(view.Accounts[0].Error, "sensitive-value") {
		t.Fatal("failed account lost facts/status")
	}
	if len(view.Accounts[1].Items) != 0 || view.Accounts[1].Error != "" {
		t.Fatal("healthy account did not replace membership")
	}
	restored, err := New(dir, as, resolve)
	if err != nil {
		t.Fatal(err)
	}
	if !restored.Snapshot().Annotations[ref.String()].SnoozedUntil.Equal(until) {
		t.Fatal("snooze lost")
	}
	if len(restored.Snapshot().Accounts[0].Items) != 1 {
		t.Fatal("failed snapshot not persisted")
	}
	if err := os.RemoveAll(filepath.Join(dir, "cache")); err != nil {
		t.Fatal(err)
	}
	restored, err = New(dir, as, resolve)
	if err != nil {
		t.Fatal(err)
	}
	if len(restored.Snapshot().Annotations) != 1 {
		t.Fatal("cache deletion lost authored state")
	}
}

func TestIncrementalReadsAndIndependentCopies(t *testing.T) {
	a := account("lab.test", forge.GitLab)
	p := provider(a)
	s, _ := New("", []config.Forge{a}, func(context.Context, config.Forge) (forge.Provider, error) { return p, nil })
	view := s.Refresh(context.Background())
	view.Accounts[0].Items[0].Change.Title = "mutated"
	s.Refresh(context.Background())
	if p.opens.Load() != 1 || s.Snapshot().Accounts[0].Items[0].Change.Title != "" {
		t.Fatal("cache not reused/isolated")
	}
	if p.changes[p.members[0].Ref.String()].Reviews[0].Bot {
		t.Fatal("provider facts mutated by bot config")
	}
	p.members[1].Updated = time.Time{}
	s.Refresh(context.Background())
	if p.opens.Load() != 2 {
		t.Fatal("missing stamp incorrectly reused cache")
	}
	p.members[0].Updated = p.members[0].Updated.Add(time.Hour)
	p.members[1] = p.members[0]
	s.Refresh(context.Background())
	if p.opens.Load() != 3 {
		t.Fatal("updated stamp did not reopen")
	}
}

func TestCancelledAccountAndUntrustedErrors(t *testing.T) {
	a := account("lab.test", forge.GitLab)
	p := provider(a)
	s, _ := New("", []config.Forge{a}, func(context.Context, config.Forge) (forge.Provider, error) { return p, nil })
	original := s.Refresh(context.Background()).Accounts[0]
	p.wait = true
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	got := s.Refresh(ctx).Accounts[0]
	if len(got.Items) != 1 || !got.LastSuccess.Equal(original.LastSuccess) || !strings.Contains(got.Error, "timed out") {
		t.Fatal("cancel erased snapshot")
	}
	p.wait = false
	p.err = errors.New("sensitive-value")
	if strings.Contains(s.Refresh(context.Background()).Accounts[0].Error, "sensitive-value") {
		t.Fatal("untrusted error escaped")
	}
}

func TestBotConfigRemovalAndPrivateCache(t *testing.T) {
	a := account("lab.test", forge.GitLab)
	p := provider(a)
	dir := t.TempDir()
	resolve := func(context.Context, config.Forge) (forge.Provider, error) { return p, nil }
	s, _ := New(dir, []config.Forge{a}, resolve)
	v := s.Refresh(context.Background())
	info, err := os.Stat(s.cachePath(v.Accounts[0].Key))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatal("cache is not private")
	}
	a.Bots = nil
	s, err = New(dir, []config.Forge{a}, resolve)
	if err != nil {
		t.Fatal(err)
	}
	if s.Snapshot().Accounts[0].Items[0].Change.Reviews[0].Bot {
		t.Fatal("removed bot config stuck in derived cache")
	}
}

// TestOneUnreadableChangeKeepsTheRest holds decision D36: discovery is
// complete, so a change that cannot be read does not fail the account.
func TestOneUnreadableChangeKeepsTheRest(t *testing.T) {
	a := account("lab.test", forge.GitLab)
	p := provider(a)
	s, _ := New("", []config.Forge{a}, func(context.Context, config.Forge) (forge.Provider, error) { return p, nil })
	s.Refresh(context.Background())
	first := forge.Ref{Host: "lab.test", Project: "team/repo", Number: 1}
	never := forge.Ref{Host: "lab.test", Project: "team/repo", Number: 2}
	p.members = append(p.members, forge.Discovered{Ref: never, Roles: []forge.Role{forge.AuthorRole}})
	p.failing = map[string]error{never.String(): errors.New("sensitive-value")}

	got := s.Refresh(context.Background()).Accounts[0]
	if got.Error != "" || got.LastSuccess.IsZero() || len(got.Items) != 2 {
		t.Fatalf("error %q, %d items", got.Error, len(got.Items))
	}
	if !strings.Contains(got.Warning, "1 change could not be read") {
		t.Fatalf("warning %q", got.Warning)
	}
	d := s.Display(time.Now())
	if len(d.Unread) != 1 || d.Unread[0].Ref != never || d.Unread[0].Error == "" || strings.Contains(d.Unread[0].Error, "sensitive") {
		t.Fatalf("unread %+v", d.Unread)
	}
	if len(d.Mine)+len(d.Theirs) != 1 || d.Accounts[0].Warning == "" {
		t.Fatalf("the readable change is missing: %+v %+v", d.Mine, d.Theirs)
	}

	// A change read before keeps its last facts, marked, when a read fails.
	// It moved on the forge, so the cached copy cannot be reused unread.
	for i := range p.members {
		if p.members[i].Ref == first {
			p.members[i].Updated = p.members[i].Updated.Add(time.Hour)
		}
	}
	p.failing = map[string]error{first.String(): &forge.APIError{Status: 403, Message: "sensitive-value"}}
	delete(p.changes, never.String())
	p.changes[never.String()] = &forge.Change{Kind: a.Kind, Ref: never, State: forge.Open, Title: "Now readable", Author: forge.User{Username: "me"}}
	got = s.Refresh(context.Background()).Accounts[0]
	d = s.Display(time.Now())
	var stale *Row
	for _, rows := range [][]Row{d.Mine, d.Theirs} {
		for i := range rows {
			if rows[i].Ref == first {
				stale = &rows[i]
			}
		}
	}
	if stale == nil || stale.Error != "The account may not read this change." || len(d.Unread) != 0 {
		t.Fatalf("stale %+v unread %+v", stale, d.Unread)
	}
	if got.Error != "" || got.Warning == "" {
		t.Fatalf("account error %q warning %q", got.Error, got.Warning)
	}

	// The next good read clears the mark and the warning.
	p.failing = nil
	got = s.Refresh(context.Background()).Accounts[0]
	if got.Warning != "" {
		t.Fatalf("warning %q after a clean refresh", got.Warning)
	}
	for _, item := range got.Items {
		if item.Error != "" || item.Unread {
			t.Fatalf("item %s still marked", item.Change.Ref)
		}
	}
}

// TestChangesAreReadConcurrently refreshes sixty changes that each take 50ms
// to read, in well under the three seconds a serial read would take.
func TestChangesAreReadConcurrently(t *testing.T) {
	a := account("lab.test", forge.GitLab)
	p := &fixture{changes: map[string]*forge.Change{}, delay: 50 * time.Millisecond}
	for n := 1; n <= 60; n++ {
		r := forge.Ref{Host: "lab.test", Project: "team/repo", Number: n}
		p.members = append(p.members, forge.Discovered{Ref: r, Roles: []forge.Role{forge.ReviewerRole}})
		p.changes[r.String()] = &forge.Change{Kind: a.Kind, Ref: r, State: forge.Open}
	}
	s, _ := New("", []config.Forge{a}, func(context.Context, config.Forge) (forge.Provider, error) { return p, nil })
	start := time.Now()
	got := s.Refresh(context.Background()).Accounts[0]
	if got.Error != "" || len(got.Items) != 60 {
		t.Fatalf("error %q, %d items", got.Error, len(got.Items))
	}
	if peak := p.peak.Load(); peak < 2 || peak > readWorkers {
		t.Fatalf("peak concurrency %d, want 2..%d", peak, readWorkers)
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Fatalf("refresh took %s", took)
	}
	for i := 1; i < len(got.Items); i++ {
		if got.Items[i-1].Change.Ref.String() >= got.Items[i].Change.Ref.String() {
			t.Fatal("items are not in a stable order")
		}
	}
}

func TestProviderOwnershipAndReviewOnlyBotMarker(t *testing.T) {
	a := account("lab.test", forge.GitLab)
	p := provider(a)
	raw := p.changes[p.members[0].Ref.String()]
	raw.Reviews = append(raw.Reviews, forge.Review{Reviewer: "review-bot", Bot: true})
	raw.Threads[0].Notes = append(raw.Threads[0].Notes, forge.Note{Author: forge.User{Username: "review-bot"}, Body: "original"})
	s, _ := New("", []config.Forge{a}, func(context.Context, config.Forge) (forge.Provider, error) { return p, nil })
	s.Refresh(context.Background())
	raw.Threads[0].Notes[2].Body = "provider changed"
	raw.Reviews[2].Bot = false
	raw.Reviewers[0].Username = "changed"
	got := s.Snapshot().Accounts[0].Items[0].Change
	if got.Threads[0].Notes[2].Body != "original" || got.Reviewers[0].Username != "robot" || !got.Reviews[2].Bot {
		t.Fatal("provider owns nested cached facts")
	}
	if !got.Threads[0].Notes[2].Author.Bot {
		t.Fatal("review-only bot marker not propagated")
	}
	p.err = errors.New("offline")
	got = s.Refresh(context.Background()).Accounts[0].Items[0].Change
	if got.Threads[0].Notes[2].Body != "original" {
		t.Fatal("failed refresh exposed provider mutations")
	}
}

type factsFixture struct {
	*fixture
	reads atomic.Int32
}

func (f *factsFixture) ReadFacts(ctx context.Context, r forge.Ref) (*forge.Change, error) {
	f.reads.Add(1)
	return f.changes[r.String()], nil
}
func TestFreshFactsReflectPermissionsWithoutUpdateStampChange(t *testing.T) {
	a := account("lab.test", forge.GitLab)
	p := &factsFixture{fixture: provider(a)}
	s, _ := New("", []config.Forge{a}, func(context.Context, config.Forge) (forge.Provider, error) { return p, nil })
	s.Refresh(context.Background())
	allowed := true
	p.changes[p.members[0].Ref.String()].CanMerge = &allowed
	got := s.Refresh(context.Background()).Accounts[0]
	if got.Error != "" || p.reads.Load() != 2 || p.opens.Load() != 0 || got.Items[0].Change.CanMerge == nil || !*got.Items[0].Change.CanMerge {
		t.Fatal("fresh permission facts were hidden by unchanged update stamp")
	}
}
