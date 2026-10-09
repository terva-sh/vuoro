---
schema: 3
id: TKT-01M4HAZD5QSQZRN4ZBGHE3NZ7A
title: Drop the name from the menu bar, and let the person keep the Dock icon
type: task
status: in-progress
status_reason: null
priority: normal
due_on: null
labels:
  - desktop
  - ui
  - cross-repo
assignees: []
milestone: null
parent: null
origin: null
dependencies: []
blocks_on: none
references: []
claim:
  actor: agent:claude/forgejo-parity-20261006
  branch: dock-setting
  worktree: /home/sothr/.t3/worktrees/vuoro/t3code-8cba4816
  commit: 962ec5779f36422d67ef4caabcee834964e0cf88
  session: null
  claimed_at: 2026-10-09T22:01:42Z
  expires_at: null
archive: null
created_at: 2026-10-09T22:01:42Z
updated_at: 2026-10-09T22:04:30Z
created_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
updated_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
extensions: {}
---

## Description

On macOS the menu bar showed "vuoro" beside the mark, which took a lot of room, and vuoro had no Dock icon: tuohi's tray makes an application a menu-bar app. The owner asked for the name to go and for the Dock icon to be a setting, even one that applies at the next start (D58).

### Approach

- Drop `Title` from the tray config. The tooltip and the menu's version line still name vuoro.
- tuohi v0.1.1 adds `tray.Config.Dock` (tuohi TKT-01M4HAEK8VHG2EY13RCNWVTC06, Let a tray application keep its macOS Dock icon). Bump the pin in its own commit.
- `desktop.dock` in the configuration file, read at startup on macOS with a tray. Settings, under Background, offers "Keep vuoro in the Dock as well as the menu bar" only where there is a Dock, and says when the saved choice waits for a restart. A window with no Dock sends no choice, so a file shared with a Mac keeps it.

### Alternatives

- A Dock icon only while the window is open: needs tuohi to switch the activation policy at run time. Deferred.
- A Dock icon always: a closed vuoro would keep a Dock slot. Rejected.

## Acceptance criteria

- [x] The macOS menu bar shows the mark without the name
- [x] Settings offers the Dock choice on macOS only, saves it as desktop.dock, and says when it waits for a restart
- [x] vuoro pins tuohi v0.1.1, in its own commit
- [ ] Checked on a Mac: the Dock icon follows the setting after a restart

## Notes

**agent:claude/forgejo-parity-20261006** at 2026-10-09T22:04:30Z

tuohi PR #80 merged (cf945c1); tuohi v0.1.1 is tagged and pushed to Forgejo and GitHub after GitHub's macOS and Windows jobs passed on that commit. Criterion 4 waits for a person on a Mac.
