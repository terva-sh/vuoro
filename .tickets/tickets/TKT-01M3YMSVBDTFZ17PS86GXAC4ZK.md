---
schema: 3
id: TKT-01M3YMSVBDTFZ17PS86GXAC4ZK
title: Tray, background refresh, and quiet turn notifications
type: task
status: in-progress
status_reason: null
priority: normal
due_on: null
labels:
  - desktop
  - inbox
assignees: []
milestone: v1.0.0
parent: TKT-01M3YMSTYMFRDFXD4QSVWHMM8P
origin: null
dependencies:
  - TKT-01M3W4HWR5K8TXA6S6HBVXDXX5
blocks_on: none
references: []
claim:
  actor: agent:claude/forgejo-parity-20261006
  branch: tray
  worktree: /home/sothr/.t3/worktrees/vuoro/t3code-8cba4816
  commit: 27c8a16462d77f69755fe8540d9401ed0d6500f0
  session: null
  claimed_at: 2026-10-08T14:33:15Z
  expires_at: null
archive: null
created_at: 2026-10-02T15:47:52Z
updated_at: 2026-10-08T14:44:09Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
extensions: {}
---

## Description

vuoro lives in the tray, using tuohi's tray, notify, and autostart. It refreshes
in the background, and it notifies when a change becomes the person's turn,
with the reason, such as "Mason replied on 2 threads". It also notifies when a
long review run finishes. Notifications are batched and quiet by default.

## Acceptance criteria

- [ ] Closing the window leaves vuoro in the tray, still refreshing
- [x] A turn change produces one notification with its reason, and a burst of changes is batched

## Implementation plan

The owner's choices are D55. They were made on 2026-10-08, when the owner asked for this ticket to be taken up, which is its promotion from draft.

### Platform: tuohi v0.1.0 has every piece, without cgo
- `tray.Set` and `tray.Remove` run from `App.Start`.
- `notify.ShowOpts` with `UrgencyLow`.
- `autostart.New(id).Enable(args...)`.
- `App.Exit=false` keeps the process alive after the last window closes. `App.Show(view)` re-creates a closed view, so the tray's Open item is a single call.

There is one catch. On Linux, `tray.Set` succeeds even with no StatusNotifierWatcher on the session bus, and then no icon shows. tuohi exposes no check for this, so vuoro asks the bus itself with the platform's `dbus-send` (`NameHasOwner org.kde.StatusNotifierWatcher`). If the check cannot run, vuoro assumes there is no tray, so closing quits and the window can never be lost.

### Slices
1. **`review.Manager.Finished`** is a hook called, outside the run's lock, when a turn's end leaves a run idle. The app notifies the first time a review completes, if the run took two minutes or longer.
2. **Config.** `inbox.refresh_minutes` (default 15, range 1 to 1440) and `inbox.notify` (default on), with validation.
3. **Nudges** (`app/nudge.go`). After every inbox refresh, whether from the window or the background, the "mine" rows are compared with the turns already notified, kept in `StateDir/nudges.json`.
   - A row is new when its ref was not notified, or its turn's `Since` moved.
   - The first run with no file records a baseline and sends nothing.
   - One notification goes out per refresh: a single change gives its title and first summary line, and several give a count plus the first titles.
   - The notifier is injected, so tests need no desktop.
4. **Background refresh.** `App.Background(ctx)` refreshes on the configured interval, re-reading it on each tick so a change in the window applies without a restart. It starts with the server, independent of the window.
5. **Setup.** The setup view gains a background section: the interval, the notifications switch, and start at login, which shows whether the platform supports it. `POST /api/setup/background` saves the first two, with the config's base hash. `POST /api/setup/autostart` turns start at login on or off through an injected controller.
6. **main.go.**
   - If a tray host exists, set `Exit=false` and `Start=tray.Set` with the items Open vuoro, Refresh now, and Quit vuoro. Remove the tray after `Wait`.
   - A `--background` flag starts with the window closed, but only when a tray exists. Start at login uses it.
   - The toggles stay in the window, not the tray menu: tuohi's menu is fixed for its lifetime, so a tray checkbox would go stale after a change in setup.
7. **Docs.** D55, plan section 3.3, and the README.

### Tests
- Nudge diffing and batching, plus the baseline.
- Background interval reloading, using a short interval.
- Config validation.
- The setup routes, with a fake autostart.
- The Finished hook.
- The tray wiring itself runs only with `just test-gui`. The Linux tray-host check is a pure function over the command's output, with a test.

## Notes

**agent:claude/forgejo-parity-20261006** at 2026-10-08T14:44:09Z

Built as planned. `just ci` passes. Design: docs/plan.md section 3.3, and D55.

**Ticked, criterion 2 (one notification per turn change, with its reason; a burst is batched).**
- `TestTurnsAreToldOncePerRefreshAndBatched` covers:
  - the first-run baseline;
  - one change, told with its first summary line;
  - several changes as one count;
  - a turn that began again;
  - a change that left and came back;
  - surviving a restart;
  - notifications off.
- `TestTheBackgroundRefreshTellsWhatTurnedWithNoWindow` drives the real inbox with no window.
- `TestALongReviewSaysItFinishedOnce` and the verifier test's `Finished` assertions cover the long-review notice.

**Unticked, criterion 1 (closing the window leaves vuoro in the tray, still refreshing).** The refresh half is tested. The tray half is not: this session had no display (no `DISPLAY`, `WAYLAND_DISPLAY` or session type), so neither the tray icon nor hide-on-close was ever seen. Tested offline:
- the tray-host decision (`TestATrayIsUsedOnlyWhereTheDesktopShowsOne`);
- the mark (`TestTheMarkIsAVioletDisc`).

To tick it, someone at a desktop checks:
1. KDE, or GNOME with the AppIndicator extension: the icon shows, closing the window keeps vuoro running, Open and Quit work, and a second `vuoro` opens the first one's window.
2. GNOME without the extension: closing quits.
3. `vuoro -background`: it starts hidden, and the tray opens the window.
4. A notification appears, at low urgency.
5. Start at login, in setup, writes `~/.config/autostart/vuoro.desktop` with `-background`.
6. macOS and Windows, when available. An unbundled macOS binary gets `notify.ErrUnavailable`, which vuoro ignores, so notifications there need the bundled `.app` from the release ticket.

**Added beyond the plan:** tuohi's single-instance lock. Without it, hide-to-tray plus a second launch, or start at login plus a manual launch, would run two vuoros with two refreshes and doubled notifications. A later launch now opens the running one's window instead.
