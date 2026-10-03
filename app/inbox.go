package app

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/inbox"
)

func (a *App) inbox() (*inbox.Service, error) {
	a.inboxOnce.Do(func() {
		if a.ConfigErr != nil {
			a.inboxErr = errors.New("The inbox configuration could not load; check the configuration file.")
			return
		}
		a.inboxService, a.inboxErr = inbox.New(a.StateDir, a.config().Forges, func(ctx context.Context, f config.Forge) (forge.Provider, error) {
			u, err := url.Parse(f.URL)
			if err != nil {
				return nil, errors.New("invalid forge URL")
			}
			return a.client(ctx, u.Host, f.Kind)
		})
		if a.inboxErr != nil {
			a.inboxErr = errors.New("The inbox cache could not load; check local state.")
		}
	})
	return a.inboxService, a.inboxErr
}

func (a *App) getInbox(w http.ResponseWriter, r *http.Request) {
	s, err := a.inbox()
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.Display(time.Now()))
}

func (a *App) refreshInbox(w http.ResponseWriter, r *http.Request) {
	s, err := a.inbox()
	if err != nil {
		writeError(w, err)
		return
	}
	s.Refresh(r.Context())
	writeJSON(w, http.StatusOK, s.Display(time.Now()))
}

func (a *App) snoozeInbox(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Ref   forge.Ref `json:"ref"`
		Until time.Time `json:"until"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	s, err := a.inbox()
	if err != nil {
		writeError(w, err)
		return
	}
	found := false
	for _, account := range s.Snapshot().Accounts {
		for _, item := range account.Items {
			if item.Change.Ref == req.Ref {
				found = true
			}
		}
	}
	if !found {
		writeError(w, errors.New("This change is not in a configured inbox account."))
		return
	}
	if !req.Until.IsZero() && !req.Until.After(time.Now()) {
		writeError(w, errors.New("Choose a future snooze time."))
		return
	}
	if err := s.Snooze(req.Ref, req.Until); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.Display(time.Now()))
}
