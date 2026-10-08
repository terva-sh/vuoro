---
schema: 3
id: TKT-01M4EHCC0H9KTWM09ZSW1NDD88
title: Give vuoro its own mark on the window, taskbar, and tray
type: task
status: in-progress
status_reason: null
priority: normal
due_on: null
labels:
  - desktop
  - ui
assignees: []
milestone: null
parent: null
origin: null
dependencies: []
blocks_on: none
references: []
claim:
  actor: agent:claude/forgejo-parity-20261006
  branch: app-icon
  worktree: /home/sothr/.t3/worktrees/vuoro/t3code-8cba4816
  commit: d8eca34c67ba4ce778d92c83147e168b669523ea
  session: null
  claimed_at: 2026-10-08T19:56:00Z
  expires_at: null
archive: null
created_at: 2026-10-08T19:55:55Z
updated_at: 2026-10-08T19:57:33Z
created_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
updated_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
extensions: {}
---

## Description

vuoro set no application icon, so its window, taskbar entry, and dock showed
tuohi's own default mark, a blue window with an orange A. Only the tray had a
vuoro mark, and that was a placeholder ring.

The owner chose a mark from six drawn concepts on 2026-10-09: a white
lowercase v on a violet tile, with an amber dot at the tip of its right arm.
They also chose to let vuoro write tuohi's desktop entry wherever it runs.
D56 records both.

## Acceptance criteria

- [ ] The window, the taskbar, and the dock show vuoro's mark rather than tuohi's
- [ ] The tray and the notifications show the same mark
- [ ] A GTK3 window on Wayland finds the mark through a desktop entry

## Implementation plan

Draw the mark in desktop.go's icon() as a few shapes tested point by point, supersampling only the pixels on an edge, so both sizes take about 7 ms at launch. Give tuohi.App the 256 px mark as Icon with DesktopEntry on; the tray and notifications keep the 64 px one. Record D56 and update plan section 3.3.

## Notes

**agent:claude/forgejo-parity-20261006** at 2026-10-08T19:57:33Z

The criteria name what a desktop shows, and this session has no display, so they stay unticked until someone looks at a running vuoro. Unit tests hold the mark's pixels at 256 and 22 px. Concept sheets were drawn in scratch, not committed.
