---
schema: 3
id: TKT-01M4EMZ8WKKPZHYASG0FAK063Q
title: Show which vuoro is running, and what it is built with
type: task
status: in-progress
status_reason: null
priority: normal
due_on: null
labels:
  - ui
  - desktop
assignees: []
milestone: null
parent: null
origin: null
dependencies: []
blocks_on: none
references: []
claim:
  actor: agent:claude/forgejo-parity-20261006
  branch: about-pane
  worktree: /home/sothr/.t3/worktrees/vuoro/t3code-8cba4816
  commit: 0aaf43de274af8d915c31129830d730c46493807
  session: null
  claimed_at: 2026-10-08T20:58:49Z
  expires_at: null
archive: null
created_at: 2026-10-08T20:58:40Z
updated_at: 2026-10-08T20:58:49Z
created_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
updated_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
extensions: {}
---

## Description

The window showed "vuoro dev" in its header and nothing more, so there was no way to see which build, which tuohi, or which agent tools were running without a terminal. The owner asked for a menu or pane that lists the versions.

## Acceptance criteria

- [x] The header's version opens a pane with vuoro's version, commit, platform, Go, and linked modules
- [x] The pane lists the agent tools with their versions, and the configuration and state paths
- [x] The pane copies the same facts as text for a bug report
- [ ] The tray menu names the running version

## Implementation plan

Add GET /api/about, from runtime/debug.ReadBuildInfo plus App.Version, ConfigPath, and StateDir. The header's version becomes a button opening an About modal that also lists setup.tools, with a copy-as-text button. Add a disabled version item to the tray menu.

## Notes

**agent:claude/forgejo-parity-20261006** at 2026-10-08T20:58:49Z

The pane was checked in headless Chromium against vuoro -browser. The tray item is in the menu's code, but no desktop has shown it yet, so criterion 4 waits on the same desktop check as TKT-01M3YMSVB.
