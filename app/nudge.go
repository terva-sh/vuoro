package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/inbox"
	"github.com/terva-sh/vuoro/review"
	"github.com/terva-sh/vuoro/state"
)

// A nudge is a quiet desktop notification (D24, D55): at most one per inbox
// refresh, when changes became the person's turn, and one when a long review
// finishes. Which turns were told is kept in the state directory, so a launch
// at login tells what turned overnight and nothing told before.

// Notifier shows a desktop notification. Nil sends none.
type Notifier func(title, body string) error

// longRun is how long a review runs before its end is worth a notification;
// a shorter one, the person is likely still watching.
const longRun = 2 * time.Minute

// keptRuns bounds the record of reviews whose end was told.
const keptRuns = 200

// nudges is what was told, in nudges.json.
type nudges struct {
	// Baseline says an inbox refresh has recorded the person's turns, so the
	// next one tells what changed. A review's end alone records none.
	Baseline bool `json:"baseline,omitempty"`
	// Turns holds each change that is the person's turn, by Ref.String(),
	// with when that turn began, as it stood after the last refresh.
	Turns map[string]string `json:"turns"`
	// Runs are the reviews whose end was told, oldest first.
	Runs []string `json:"runs,omitempty"`
}

func (a *App) nudgePath() string { return filepath.Join(a.StateDir, "nudges.json") }

// readNudges is what was told: read from nudges.json once, then kept in
// memory, so a file that cannot be written repeats nothing until vuoro
// restarts. The caller holds nudgeMu.
func (a *App) readNudges() nudges {
	if a.told == nil {
		var n nudges
		if a.StateDir != "" {
			if err := state.ReadJSON(a.nudgePath(), &n); err != nil && !errors.Is(err, fs.ErrNotExist) {
				a.logf("what vuoro has notified could not be read, so it starts again: %v", err)
			}
		}
		a.told = &n
	}
	n := *a.told
	n.Turns = maps.Clone(n.Turns)
	if n.Turns == nil {
		n.Turns = map[string]string{}
	}
	n.Runs = slices.Clone(n.Runs)
	return n
}

// saveNudges keeps n in memory and writes it to nudges.json. The caller
// holds nudgeMu.
func (a *App) saveNudges(n nudges) {
	a.told = &n
	if a.StateDir == "" {
		return
	}
	err := state.MkdirAll(a.StateDir)
	if err == nil {
		err = state.WriteJSON(a.nudgePath(), n)
	}
	if err != nil {
		a.logf("what vuoro has notified could not be saved, so it may notify again after a restart: %v", err)
	}
}

// logf reports a problem the person cannot act on in the window.
func (a *App) logf(format string, args ...any) {
	if a.Log != nil {
		a.Log(format, args...)
	}
}

// turnKey says when a row's turn began, so a turn that began again is new.
func turnKey(r inbox.Row) string {
	if r.Turn.Since == nil {
		return "-"
	}
	return r.Turn.Since.UTC().Format(time.RFC3339Nano)
}

// nudgeTurns tells, in one notification, the changes that became the
// person's turn since the last refresh. The first refresh with nothing kept
// only takes note, so a new install does not announce the whole inbox.
func (a *App) nudgeTurns(d inbox.Display) {
	a.nudgeMu.Lock()
	defer a.nudgeMu.Unlock()
	a.tellTurns(d)
}

// nudge tells what turned in the inbox as it stands, and returns that view.
// The view is read under nudgeMu: refreshes are serialized and a view shows
// the latest, so each nudge sees the inbox at least as new as the one before
// it, however the window's, the background's, and the tray's refreshes
// overlap, and an older view never overwrites what a newer one recorded.
func (a *App) nudge(s *inbox.Service) inbox.Display {
	a.nudgeMu.Lock()
	defer a.nudgeMu.Unlock()
	d := s.Display(time.Now())
	a.tellTurns(d)
	return d
}

// tellTurns is nudgeTurns with nudgeMu held.
func (a *App) tellTurns(d inbox.Display) {
	told := a.readNudges()
	now := map[string]string{}
	var turned []inbox.Row
	for _, r := range d.Mine {
		key := r.Ref.String()
		now[key] = turnKey(r)
		if told.Baseline && told.Turns[key] != now[key] {
			turned = append(turned, r)
		}
	}
	// A notification the desktop did not show leaves its turns untold, so
	// the next refresh tries again. With notifications off, or no notifier,
	// the turns are only noted.
	if len(turned) > 0 && a.Notify != nil && a.config().Notifies() {
		title, body := turnNotice(turned)
		if err := a.Notify(title, body); err != nil {
			for _, r := range turned {
				key := r.Ref.String()
				if old, ok := told.Turns[key]; ok {
					now[key] = old
				} else {
					delete(now, key)
				}
			}
		}
	}
	told.Turns, told.Baseline = now, true
	a.saveNudges(told)
}

// turnNotice words the notification for the changes that turned: the change
// and why when it is one, and a count with the first few when it is more.
func turnNotice(rows []inbox.Row) (title, body string) {
	name := func(r inbox.Row) string {
		ref := shortRef(r.Ref, r.Kind)
		if r.Title == "" {
			return ref
		}
		return ref + " " + r.Title
	}
	if len(rows) == 1 {
		r := rows[0]
		body = "It is your turn."
		if len(r.Summary) > 0 {
			body = r.Summary[0]
		}
		return "Your turn: " + name(r), body
	}
	var lines []string
	for _, r := range rows[:min(3, len(rows))] {
		lines = append(lines, name(r))
	}
	if more := len(rows) - len(lines); more > 0 {
		lines = append(lines, fmt.Sprintf("and %d more", more))
	}
	return fmt.Sprintf("%d changes are your turn", len(rows)), strings.Join(lines, "\n")
}

// shortRef names a change without its host, as team/repo!12, which is
// enough in a notification.
func shortRef(ref forge.Ref, kind string) string {
	return strings.TrimPrefix(ref.Display(kind), ref.Host+"/")
}

// RunFinished is review.Manager.Finished: a review that ran two minutes or
// more says it finished, once.
//
// The first finished turn that completes the review decides: a review that
// took under two minutes is recorded untold, so a later conversation with
// one of its sessions cannot make it look long.
func (a *App) RunFinished(v review.RunView) {
	if !v.ReviewComplete {
		return
	}
	a.nudgeMu.Lock()
	defer a.nudgeMu.Unlock()
	told := a.readNudges()
	if slices.Contains(told.Runs, v.ID) {
		return
	}
	// A notification the desktop did not show is not recorded, so the run's
	// next finished turn tries again.
	if time.Since(v.Created) >= longRun && a.Notify != nil && a.config().Notifies() {
		name := v.Title
		if v.Kind != review.Local && v.Ref != (forge.Ref{}) {
			name = shortRef(v.Ref, v.Kind) + " " + v.Title
		}
		if err := a.Notify("Review finished: "+strings.TrimSpace(name), "Its findings are ready to read."); err != nil {
			return
		}
	}
	told.Runs = append(told.Runs, v.ID)
	if n := len(told.Runs); n > keptRuns {
		told.Runs = told.Runs[n-keptRuns:]
	}
	a.saveNudges(told)
}

// Background refreshes the inbox on the configured interval until ctx ends,
// window or no window, and nudges after each refresh. It reads the interval
// again after each wait, so a change in setup applies without a restart.
func (a *App) Background(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(a.refreshEvery()):
		}
		a.RefreshNow(ctx)
	}
}

// RefreshNow refreshes the inbox and tells what turned, as the tray's
// Refresh now does.
func (a *App) RefreshNow(ctx context.Context) {
	s, err := a.inbox()
	if err != nil {
		return
	}
	s.Refresh(ctx)
	a.nudge(s)
}

// refreshEvery is the configured interval, or the test's.
func (a *App) refreshEvery() time.Duration {
	if a.RefreshEvery > 0 {
		return a.RefreshEvery
	}
	return a.config().RefreshEvery()
}
