package app

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/terva-sh/vuoro/update"
)

// Updater finds and applies a newer vuoro release (D74). update.Client is
// one; a test supplies a fake.
type Updater interface {
	Latest(ctx context.Context, current string) (*update.Release, error)
	Apply(ctx context.Context, rel *update.Release, current, target string) error
}

// How often the window asks GitHub for a newer release, and how soon it asks
// again after the question failed, such as offline.
const (
	selfUpdateEvery      = 24 * time.Hour
	selfUpdateRetryAfter = time.Hour
)

// selfUpdates is what the window was last told about a newer release.
type selfUpdates struct {
	checked time.Time
	latest  *update.Release
	err     string
	applied string // the release now on disk, waiting for a reopen
}

type selfUpdateView struct {
	Current string `json:"current"`
	// Release says this vuoro is a release, so it can be updated. A
	// development build is never checked.
	Release bool   `json:"release"`
	Latest  string `json:"latest,omitempty"`
	Newer   bool   `json:"newer"`
	// Applied is the release that replaced this one on disk; it runs once
	// the person reopens vuoro.
	Applied string `json:"applied,omitempty"`
	Error   string `json:"error,omitempty"`
}

// selfUpdateState answers the window's question, asking GitHub at most once a
// day, or an hour after a failed question. updMu is held.
func (a *App) selfUpdateState(ctx context.Context) selfUpdateView {
	v := selfUpdateView{Current: a.Version}
	if a.Updates == nil {
		return v
	}
	if _, err := update.Newer(a.Version, "v0.0.0"); errors.Is(err, update.ErrNotARelease) {
		return v
	}
	v.Release = true
	u := &a.upd
	wait := selfUpdateEvery
	if u.err != "" {
		wait = selfUpdateRetryAfter
	}
	if u.applied == "" && (u.checked.IsZero() || time.Since(u.checked) >= wait) {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		rel, err := a.Updates.Latest(ctx, a.Version)
		cancel()
		u.checked, u.err = time.Now(), ""
		if err != nil {
			u.err = err.Error()
		} else {
			u.latest = rel
		}
	}
	v.Applied, v.Error = u.applied, u.err
	if u.latest != nil {
		v.Latest = u.latest.Tag
		v.Newer, _ = update.Newer(a.Version, u.latest.Tag)
	}
	if v.Applied != "" {
		v.Newer = false
	}
	return v
}

func (a *App) getSelfUpdate(w http.ResponseWriter, r *http.Request) {
	a.updMu.Lock()
	v := a.selfUpdateState(r.Context())
	a.updMu.Unlock()
	writeJSON(w, http.StatusOK, v)
}

// applySelfUpdate replaces vuoro's binary with the newer release, on the person's
// action alone. The running vuoro carries on; the new one runs when the person
// reopens it.
func (a *App) applySelfUpdate(w http.ResponseWriter, r *http.Request) {
	a.updMu.Lock()
	defer a.updMu.Unlock()
	v := a.selfUpdateState(r.Context())
	if !v.Newer {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "there is no newer release to update to"})
		return
	}
	exe := a.Executable
	if exe == nil {
		exe = update.Executable
	}
	target, err := exe()
	if err == nil {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
		err = a.Updates.Apply(ctx, a.upd.latest, a.Version, target)
		cancel()
	}
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	a.upd.applied = a.upd.latest.Tag
	writeJSON(w, http.StatusOK, a.selfUpdateState(r.Context()))
}
