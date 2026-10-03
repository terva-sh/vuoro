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

func TestFailedReadIsAtomic(t *testing.T) {
	a := account("lab.test", forge.GitLab)
	p := provider(a)
	s, _ := New("", []config.Forge{a}, func(context.Context, config.Forge) (forge.Provider, error) { return p, nil })
	s.Refresh(context.Background())
	r := forge.Ref{Host: "lab.test", Project: "team/repo", Number: 2}
	p.members = append(p.members, forge.Discovered{Ref: r})
	got := s.Refresh(context.Background()).Accounts[0]
	if got.Error == "" || len(got.Items) != 1 {
		t.Fatal("partial reads replaced complete snapshot")
	}
}
