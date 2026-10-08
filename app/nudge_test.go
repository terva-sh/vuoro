package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/inbox"
	"github.com/terva-sh/vuoro/review"
	"github.com/terva-sh/vuoro/turn"
)

// told records the notifications an app sends.
type told struct {
	mu   sync.Mutex
	sent [][2]string
}

func (n *told) notify(title, body string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.sent = append(n.sent, [2]string{title, body})
	return nil
}

func (n *told) all() [][2]string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([][2]string(nil), n.sent...)
}

func mine(number int, title string, since time.Time, summary ...string) inbox.Row {
	return inbox.Row{Ref: forge.Ref{Host: "lab.test", Project: "team/repo", Number: number}, Kind: forge.GitLab, Title: title,
		Turn: turn.Turn{Owner: turn.Mine, Since: &since}, Summary: summary}
}

// A refresh tells, in one notification, the changes that became the
// person's turn: none on the first refresh of a new install, the change and
// why when it is one, a count when it is several, and a turn again once it
// began again.
func TestTurnsAreToldOncePerRefreshAndBatched(t *testing.T) {
	n := &told{}
	a := &App{StateDir: t.TempDir(), Config: &config.Config{}, Notify: n.notify}
	t0 := time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC)

	a.nudgeTurns(inbox.Display{Mine: []inbox.Row{mine(1, "Old work", t0)}})
	if len(n.all()) != 0 {
		t.Fatalf("a new install announced its inbox: %q", n.all())
	}
	a.nudgeTurns(inbox.Display{Mine: []inbox.Row{mine(1, "Old work", t0)}})
	if len(n.all()) != 0 {
		t.Fatalf("an unchanged turn was told again: %q", n.all())
	}

	a.nudgeTurns(inbox.Display{Mine: []inbox.Row{mine(1, "Old work", t0), mine(2, "Fix totals", t0.Add(time.Hour), "Mason replied on 2 threads.")}})
	if got := n.all(); len(got) != 1 || got[0] != [2]string{"Your turn: team/repo!2 Fix totals", "Mason replied on 2 threads."} {
		t.Fatalf("one change: %q", got)
	}

	// The first change's turn began again, and two more turned: one
	// notification for all three.
	a.nudgeTurns(inbox.Display{Mine: []inbox.Row{
		mine(1, "Old work", t0.Add(2*time.Hour)), mine(2, "Fix totals", t0.Add(time.Hour)),
		mine(3, "Third", t0), mine(4, "Fourth", t0), mine(5, "Fifth", t0)}})
	got := n.all()
	if len(got) != 2 || got[1][0] != "4 changes are your turn" || got[1][1] != "team/repo!1 Old work\nteam/repo!3 Third\nteam/repo!4 Fourth\nand 1 more" {
		t.Fatalf("several: %q", got)
	}

	// A change that stops being the person's turn and becomes it again is
	// told again.
	a.nudgeTurns(inbox.Display{Mine: []inbox.Row{mine(1, "Old work", t0.Add(2*time.Hour))}})
	a.nudgeTurns(inbox.Display{Mine: []inbox.Row{mine(1, "Old work", t0.Add(2*time.Hour)), mine(2, "Fix totals", t0.Add(time.Hour))}})
	if got := n.all(); len(got) != 3 || got[2][0] != "Your turn: team/repo!2 Fix totals" || got[2][1] != "It is your turn." {
		t.Fatalf("again: %q", got)
	}

	// What was told outlives a restart.
	b := &App{StateDir: a.StateDir, Config: &config.Config{}, Notify: n.notify}
	b.nudgeTurns(inbox.Display{Mine: []inbox.Row{mine(1, "Old work", t0.Add(2*time.Hour)), mine(2, "Fix totals", t0.Add(time.Hour))}})
	if len(n.all()) != 3 {
		t.Fatalf("a restart told it again: %q", n.all())
	}

	// With notifications off, nothing is shown, and what turned is still
	// noted, so turning them on does not announce it later.
	off := false
	b.Config = &config.Config{Inbox: &config.Inbox{Notify: &off}}
	b.nudgeTurns(inbox.Display{Mine: []inbox.Row{mine(6, "Quiet", t0)}})
	b.Config = &config.Config{}
	b.nudgeTurns(inbox.Display{Mine: []inbox.Row{mine(6, "Quiet", t0)}})
	if len(n.all()) != 3 {
		t.Fatalf("notifications off: %q", n.all())
	}
}

// A notification the desktop did not show is tried again: its turns and its
// review stay untold until one is shown.
func TestANotificationThatFailedIsTriedAgain(t *testing.T) {
	n := &told{}
	failing := true
	a := &App{StateDir: t.TempDir(), Config: &config.Config{}, Notify: func(title, body string) error {
		if failing {
			return errors.New("no notification service")
		}
		return n.notify(title, body)
	}}
	t0 := time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC)
	a.nudgeTurns(inbox.Display{Mine: []inbox.Row{mine(1, "Old work", t0)}}) // the baseline
	turned := inbox.Display{Mine: []inbox.Row{mine(1, "Old work", t0), mine(2, "Fix totals", t0)}}
	a.nudgeTurns(turned)
	long := review.RunView{ID: "r1", Kind: forge.GitLab, Ref: forge.Ref{Host: "lab.test", Project: "team/repo", Number: 7}, Title: "Fix totals",
		Created: time.Now().Add(-3 * time.Minute), ReviewComplete: true}
	a.RunFinished(long)
	failing = false
	a.nudgeTurns(turned)
	a.RunFinished(long)
	got := n.all()
	if len(got) != 2 || got[0][0] != "Your turn: team/repo!2 Fix totals" || got[1][0] != "Review finished: team/repo!7 Fix totals" {
		t.Fatalf("told %q", got)
	}
	a.nudgeTurns(turned)
	if len(n.all()) != 2 {
		t.Fatalf("told again once shown: %q", n.all())
	}
}

// Refreshes from the window, the background, and the tray may overlap; a
// turn is still told once.
func TestOverlappingRefreshesTellATurnOnce(t *testing.T) {
	a, _ := inboxApp(t)
	n := &told{}
	a.Notify = n.notify
	os.WriteFile(filepath.Join(a.StateDir, "nudges.json"), []byte(`{"baseline":true,"turns":{}}`), 0o600)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() { a.RefreshNow(context.Background()) })
	}
	wg.Wait()
	if got := n.all(); len(got) != 1 || got[0][0] != "2 changes are your turn" {
		t.Fatalf("told %q", got)
	}
}

// What was notified is kept in memory as well, so a state directory that
// cannot be written repeats no notification and loses no baseline while
// vuoro runs, and the failure is reported.
func TestAnUnwritableStateDirectoryRepeatsNothing(t *testing.T) {
	n := &told{}
	file := filepath.Join(t.TempDir(), "a-file")
	os.WriteFile(file, nil, 0o600)
	var logged []string
	a := &App{StateDir: filepath.Join(file, "state"), Config: &config.Config{}, Notify: n.notify,
		Log: func(format string, args ...any) { logged = append(logged, format) }}
	t0 := time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC)
	a.nudgeTurns(inbox.Display{Mine: []inbox.Row{mine(1, "Old work", t0)}})
	turned := inbox.Display{Mine: []inbox.Row{mine(1, "Old work", t0), mine(2, "Fix totals", t0)}}
	a.nudgeTurns(turned)
	a.nudgeTurns(turned)
	if got := n.all(); len(got) != 1 || got[0][0] != "Your turn: team/repo!2 Fix totals" {
		t.Fatalf("told %q", got)
	}
	if !slices.ContainsFunc(logged, func(l string) bool { return strings.Contains(l, "could not be saved") }) {
		t.Fatalf("logged %q", logged)
	}
}

// A review that finishes before a new install's first inbox refresh does
// not stand in for that refresh: the first refresh still only takes note.
func TestAReviewsEndIsNotTheInboxsBaseline(t *testing.T) {
	n := &told{}
	a := &App{StateDir: t.TempDir(), Config: &config.Config{}, Notify: n.notify}
	a.RunFinished(review.RunView{ID: "r1", Kind: review.Local, Title: "main..topic", Created: time.Now().Add(-5 * time.Minute), ReviewComplete: true})
	a.nudgeTurns(inbox.Display{Mine: []inbox.Row{mine(1, "Old work", time.Now()), mine(2, "Older work", time.Now())}})
	if got := n.all(); len(got) != 1 || got[0][0] != "Review finished: main..topic" {
		t.Fatalf("told %q", got)
	}
}

// A review that ran two minutes or more says it finished, once, even across
// a restart; a short one, or one not yet complete, says nothing.
func TestALongReviewSaysItFinishedOnce(t *testing.T) {
	n := &told{}
	a := &App{StateDir: t.TempDir(), Config: &config.Config{}, Notify: n.notify}
	long := review.RunView{ID: "r1", Kind: forge.GitLab, Ref: forge.Ref{Host: "lab.test", Project: "team/repo", Number: 7}, Title: "Fix totals",
		Created: time.Now().Add(-3 * time.Minute), ReviewComplete: true}
	short := long
	short.ID, short.Created = "r2", time.Now()
	// The short review is talked to later, and that turn finishes once the
	// run is old: it is still a short review.
	later := short
	later.Created = time.Now().Add(-time.Hour)
	partial := long
	partial.ID, partial.ReviewComplete = "r3", false
	for _, v := range []review.RunView{short, later, partial, long, long} {
		a.RunFinished(v)
	}
	if got := n.all(); len(got) != 1 || got[0] != [2]string{"Review finished: team/repo!7 Fix totals", "Its findings are ready to read."} {
		t.Fatalf("told %q", got)
	}
	(&App{StateDir: a.StateDir, Config: &config.Config{}, Notify: n.notify}).RunFinished(long)
	if len(n.all()) != 1 {
		t.Fatalf("a restart told it again: %q", n.all())
	}
}

// The background refresh reads the inbox on its interval with no window,
// tells what turned, and stops with its context.
func TestTheBackgroundRefreshTellsWhatTurnedWithNoWindow(t *testing.T) {
	a, _ := inboxApp(t)
	n := &told{}
	a.Notify, a.RefreshEvery = n.notify, 20*time.Millisecond
	// Something was told before, so this is not a first run.
	os.WriteFile(filepath.Join(a.StateDir, "nudges.json"), []byte(`{"baseline":true,"turns":{}}`), 0o600)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Background(ctx); close(done) }()
	deadline := time.Now().Add(5 * time.Second)
	for len(n.all()) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the background refresh outlived its context")
	}
	if got := n.all(); len(got) != 1 || got[0][0] != "2 changes are your turn" {
		t.Fatalf("told %q", got)
	}
}

// The background interval follows the configuration, and setup edits it and
// the notifications switch in the file.
func TestSetupSavesTheBackgroundSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	a := &App{StateDir: t.TempDir(), Config: &config.Config{}, ConfigPath: path}
	h := a.Handler()
	v := getSetup(t, h)
	if v.Background.RefreshMinutes != 15 || !v.Background.Notify || v.Background.Login {
		t.Fatalf("defaults %+v", v.Background)
	}
	if a.refreshEvery() != 15*time.Minute {
		t.Fatalf("every %v", a.refreshEvery())
	}
	if code, _, body := post(t, h, "/api/setup/background", map[string]any{"base": v.Base, "refresh_minutes": 0, "notify": true}); code == 200 || !strings.Contains(body, "1 to 1440") {
		t.Fatalf("zero minutes: %d %s", code, body)
	}
	code, v, body := post(t, h, "/api/setup/background", map[string]any{"base": v.Base, "refresh_minutes": 5, "notify": false})
	if code != 200 || v.Background.RefreshMinutes != 5 || v.Background.Notify {
		t.Fatalf("saved: %d %s", code, body)
	}
	if a.refreshEvery() != 5*time.Minute || a.config().Notifies() {
		t.Fatalf("running: every %v, notify %v", a.refreshEvery(), a.config().Notifies())
	}
	saved, _ := os.ReadFile(path)
	var file struct {
		Inbox map[string]any `json:"inbox"`
	}
	json.Unmarshal(saved, &file)
	if file.Inbox["refresh_minutes"] != 5.0 || file.Inbox["notify"] != false {
		t.Fatalf("file %s", saved)
	}
	// Back to the defaults, the file keeps no setting.
	if code, _, body := post(t, h, "/api/setup/background", map[string]any{"base": v.Base, "refresh_minutes": 15, "notify": true}); code != 200 {
		t.Fatalf("defaults again: %d %s", code, body)
	}
	if saved, _ := os.ReadFile(path); strings.Contains(string(saved), "inbox") {
		t.Fatalf("the defaults were written out: %s", saved)
	}
}

type fakeAutostart struct{ on bool }

func (f *fakeAutostart) Enabled() (bool, error) { return f.on, nil }
func (f *fakeAutostart) Enable() error          { f.on = true; return nil }
func (f *fakeAutostart) Disable() error         { f.on = false; return nil }

// Start at login is off until the person turns it on, and is refused where
// the platform has no way to do it.
func TestStartAtLoginIsTheWindowsToggle(t *testing.T) {
	a := &App{StateDir: t.TempDir(), Config: &config.Config{}}
	h := a.Handler()
	if code, _, _ := post(t, h, "/api/setup/autostart", map[string]any{"enabled": true}); code == 200 {
		t.Fatal("start at login was accepted with no way to do it")
	}
	f := &fakeAutostart{}
	a.Autostart = f
	if v := getSetup(t, h); !v.Background.Login || v.Background.Autostart {
		t.Fatalf("before: %+v", v.Background)
	}
	if code, v, body := post(t, h, "/api/setup/autostart", map[string]any{"enabled": true}); code != 200 || !v.Background.Autostart || !f.on {
		t.Fatalf("on: %d %s", code, body)
	}
	if code, v, _ := post(t, h, "/api/setup/autostart", map[string]any{"enabled": false}); code != 200 || v.Background.Autostart || f.on {
		t.Fatalf("off: %+v", v.Background)
	}
}
