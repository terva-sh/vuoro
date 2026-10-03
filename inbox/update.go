package inbox

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/state"
)

// RefreshChange reads fresh facts after a human write. The refresh gate is
// acquired before the read, so an older refresh cannot later overwrite it.
// A targeted read does not advance whole-account LastSuccess.
func (s *Service) RefreshChange(ctx context.Context, ref forge.Ref) error {
	select {
	case s.gate <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-s.gate }()
	view := s.rawSnapshot()
	for i, a := range view.Accounts {
		if !strings.EqualFold(a.Host, ref.Host) {
			continue
		}
		index := -1
		for j, item := range a.Items {
			if item.Change.Ref == ref {
				index = j
				break
			}
		}
		if index < 0 {
			return nil
		}
		next := clone(a)
		ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		if s.resolve == nil {
			return forge.ErrUnsupported
		}
		p, err := s.resolve(ctx, s.accounts[i])
		if err == nil && p == nil {
			err = forge.ErrUnsupported
		}
		var ch *forge.Change
		if err == nil {
			if facts, ok := p.(forge.FactsReader); ok {
				ch, err = facts.ReadFacts(ctx, ref)
			} else {
				ch, err = p.Open(ctx, ref)
			}
		}
		if err == nil {
			err = ctx.Err()
		}
		if err == nil && (ch == nil || ch.Ref != ref || ch.Kind != a.Kind) {
			err = errors.New("invalid fresh identity")
		}
		if err != nil {
			next.Error = safeError(err)
		} else if ch.State != forge.Open {
			next.Items = append(next.Items[:index], next.Items[index+1:]...)
		} else {
			next.Items[index].Change = clone(*ch)
		}
		if s.dir != "" {
			if saveErr := state.WriteJSON(s.cachePath(next.Key), next); saveErr != nil {
				a.Error = "Could not save the inbox cache."
				s.mu.Lock()
				s.snapshots[i] = a
				s.mu.Unlock()
				return errors.New("inbox: could not save refreshed change")
			}
		}
		s.mu.Lock()
		s.snapshots[i] = next
		s.mu.Unlock()
		return err
	}
	return nil
}
