package inbox

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
)

type orderedFacts struct {
	*fixture
	started chan struct{}
	unblock chan struct{}
	reads   atomic.Int32
}

func (p *orderedFacts) ReadFacts(ctx context.Context, r forge.Ref) (*forge.Change, error) {
	n := p.reads.Add(1)
	ch := clone(*p.changes[r.String()])
	if n == 2 {
		close(p.started)
		select {
		case <-p.unblock:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		ch.Title = "older refresh"
	} else if n > 2 {
		ch.Title = "after human write"
	}
	return &ch, nil
}
func TestTargetedFreshReadCannotBeOverwrittenByOlderRefresh(t *testing.T) {
	a := account("lab.test", forge.GitLab)
	p := &orderedFacts{fixture: provider(a), started: make(chan struct{}), unblock: make(chan struct{})}
	s, _ := New(t.TempDir(), []config.Forge{a}, func(context.Context, config.Forge) (forge.Provider, error) { return p, nil })
	s.Refresh(context.Background())
	ref := p.members[0].Ref
	done := make(chan struct{})
	go func() { s.Refresh(context.Background()); close(done) }()
	<-p.started
	targeted := make(chan error, 1)
	go func() { targeted <- s.RefreshChange(context.Background(), ref) }()
	close(p.unblock)
	<-done
	if err := <-targeted; err != nil {
		t.Fatal(err)
	}
	if s.Snapshot().Accounts[0].Items[0].Change.Title != "after human write" || p.reads.Load() != 3 {
		t.Fatal("older refresh overwrote human write")
	}
	success := s.Snapshot().Accounts[0].LastSuccess
	p.changes[ref.String()].State = forge.Closed
	if err := s.RefreshChange(context.Background(), ref); err != nil {
		t.Fatal(err)
	}
	got := s.Snapshot().Accounts[0]
	if len(got.Items) != 0 || !got.LastSuccess.Equal(success) {
		t.Fatal("targeted close/whole-account freshness incorrect")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s.gate <- struct{}{}
	if err := s.RefreshChange(ctx, ref); err != context.Canceled {
		t.Fatal(err)
	}
	<-s.gate
}

func TestFailedTargetedSavePreservesFactsAndShowsError(t *testing.T) {
	a := account("lab.test", forge.GitLab)
	p := provider(a)
	dir := t.TempDir()
	s, _ := New(dir, []config.Forge{a}, func(context.Context, config.Forge) (forge.Provider, error) { return p, nil })
	s.Refresh(context.Background())
	ref := p.members[0].Ref
	p.changes[ref.String()].Title = "new facts"
	if err := os.RemoveAll(filepath.Join(dir, "cache")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cache"), []byte("block writes"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.RefreshChange(context.Background(), ref); err == nil {
		t.Fatal("failed save succeeded")
	}
	got := s.Snapshot().Accounts[0]
	if got.Items[0].Change.Title == "new facts" || got.Error == "" {
		t.Fatal("failed save exposed new facts or hid its error")
	}
}
