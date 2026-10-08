package app

import (
	"errors"
	"fmt"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/terva-sh/vuoro/review"
	"github.com/terva-sh/vuoro/state"
)

// Confirmable are the actions that act on the forge as the person, where
// others see the result. Each asks first unless the person waived it.
var Confirmable = []string{"submit", "approve", "request_changes"}

// Schemes are the colour schemes the person can pin. The empty scheme
// follows the system's.
var Schemes = []string{"", "light", "dark"}

// Tabs are the home page's tabs (docs/plan.md section 6.8). The empty tab is
// the first, Reviews.
var Tabs = []string{"", "reviews", "local"}

// prefs are the person's choices made in the UI, kept in the state
// directory rather than the config file, which is theirs to write.
type prefs struct {
	// Waived are the confirmations the person asked not to see again.
	Waived []string `json:"waived"`
	// Scheme is the colour scheme the person pinned, or empty to follow the
	// system's.
	Scheme string `json:"scheme,omitempty"`
	// Tab is the home page's tab the person used last.
	Tab string `json:"tab,omitempty"`
	// Deliver is how the person last had an editor's fixes go back on a
	// change by each author, by "host/username": review.Suggestions, or
	// "change" for a stacked change (D59).
	Deliver map[string]string `json:"deliver,omitempty"`
}

// deliveryKey names a change's author for prefs.Deliver.
func deliveryKey(host, author string) string { return strings.ToLower(host + "/" + author) }

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
	if !slices.Contains(Schemes, p.Scheme) {
		p.Scheme = ""
	}
	if !slices.Contains(Tabs, p.Tab) {
		p.Tab = ""
	}
	maps.DeleteFunc(p.Deliver, func(_, v string) bool { return v != review.Suggestions && v != "change" })
	return p
}

// rememberDelivery keeps how the person had an editor's fixes go back on a
// change by this author, as the default for the next (D59).
func (a *App) rememberDelivery(host, author, deliver string) {
	if deliver == "" {
		deliver = "change"
	}
	a.prefsMu.Lock()
	defer a.prefsMu.Unlock()
	p := a.loadPrefs()
	if p.Deliver == nil {
		p.Deliver = map[string]string{}
	}
	p.Deliver[deliveryKey(host, author)] = deliver
	if path := a.prefsPath(); path != "" {
		_ = state.WriteJSON(path, p) // a default lost is only a default
	}
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
	a.savePrefs(w, p)
}

// setScheme pins the colour scheme, or with an empty one follows the
// system's again, and answers with the prefs.
func (a *App) setScheme(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Scheme string `json:"scheme"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if !slices.Contains(Schemes, req.Scheme) {
		writeError(w, fmt.Errorf("%q is not a colour scheme", req.Scheme))
		return
	}
	a.prefsMu.Lock()
	defer a.prefsMu.Unlock()
	p := a.loadPrefs()
	p.Scheme = req.Scheme
	a.savePrefs(w, p)
}

// setTab keeps the home page's tab the person moved to, and answers with
// the prefs.
func (a *App) setTab(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Tab string `json:"tab"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if !slices.Contains(Tabs, req.Tab) {
		writeError(w, fmt.Errorf("%q is not a tab of the home page", req.Tab))
		return
	}
	a.prefsMu.Lock()
	defer a.prefsMu.Unlock()
	p := a.loadPrefs()
	p.Tab = req.Tab
	a.savePrefs(w, p)
}

// savePrefs writes the prefs and answers with them. The caller holds
// prefsMu.
func (a *App) savePrefs(w http.ResponseWriter, p prefs) {
	if path := a.prefsPath(); path != "" {
		if err := state.WriteJSON(path, p); err != nil {
			writeError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, p)
}
