package app

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"

	"github.com/terva-sh/vuoro/state"
)

// Confirmable are the actions that act on the forge as the person, where
// others see the result. Each asks first unless the person waived it.
var Confirmable = []string{"submit", "approve"}

// prefs are the person's choices made in the UI, kept in the state
// directory rather than the config file, which is theirs to write.
type prefs struct {
	// Waived are the confirmations the person asked not to see again.
	Waived []string `json:"waived"`
}

func (a *App) prefsPath() string {
	if a.StateDir == "" {
		return ""
	}
	return filepath.Join(a.StateDir, "prefs.json")
}

// loadPrefs reads the prefs. With no file, or no state directory, every
// action asks.
func (a *App) loadPrefs() prefs {
	p := prefs{Waived: []string{}}
	if path := a.prefsPath(); path != "" {
		if err := state.ReadJSON(path, &p); err != nil && !errors.Is(err, os.ErrNotExist) {
			p = prefs{}
		}
	}
	p.Waived = slices.DeleteFunc(p.Waived, func(s string) bool { return !slices.Contains(Confirmable, s) })
	if p.Waived == nil {
		p.Waived = []string{}
	}
	return p
}

// setConfirm turns the confirmation of one action on or off, and answers
// with the prefs.
func (a *App) setConfirm(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Action string `json:"action"`
		Ask    bool   `json:"ask"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if !slices.Contains(Confirmable, req.Action) {
		writeError(w, fmt.Errorf("%q is not an action that asks first", req.Action))
		return
	}
	a.prefsMu.Lock()
	defer a.prefsMu.Unlock()
	p := a.loadPrefs()
	p.Waived = slices.DeleteFunc(p.Waived, func(s string) bool { return s == req.Action })
	if !req.Ask {
		p.Waived = append(p.Waived, req.Action)
	}
	if path := a.prefsPath(); path != "" {
		if err := state.WriteJSON(path, p); err != nil {
			writeError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, p)
}
