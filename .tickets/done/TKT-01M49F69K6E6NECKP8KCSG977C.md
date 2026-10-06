---
schema: 3
id: TKT-01M49F69K6E6NECKP8KCSG977C
title: Split the home page into Reviews and Local tabs
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - ui
assignees: []
milestone: null
parent: TKT-01M49F68YV694W81S3ESJKVH02
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-06T20:41:27Z
updated_at: 2026-10-06T21:07:17Z
created_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
updated_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
extensions: {}
---

## Description

Section 6.8: two tabs at the top of the home page, each with a count of what needs the person, and the last tab used kept in prefs.json.

## Acceptance criteria

- [x] Reviews holds today's inbox unchanged; Local holds the local view
- [x] Each tab shows its needs-you count
- [x] The last tab used is restored after a restart

## Summary

The home page has Reviews and Local tabs (web/src/app.tsx `Home`), each labelled with its needs-you count, and with arrow keys between them.

- The last tab is kept in prefs.json (`POST /api/prefs/tab`, returned as `tab` in /api/setup) and restored on load.
- The inbox stays mounted and polling behind Local, so its count is live; its keys are ignored while Local shows (`Inbox` `active` / `onCount`).
- Local mounts on first showing and reads `/api/local`. Until then its count comes from `GET /api/local?cached=1`, which never builds and reads the view persisted in cache/local.json.
- `web/src/local.tsx` is a minimal Local panel: per project, the branches that need the person and why, and Refresh. TKT-01M49F69S (Build the Local tab) replaces it with the full rows, Review, and adding projects and roots.
- Tests: app/prefs_test.go and app/local_test.go for the pref and the cached read; web/src/home.test.tsx for restore, counts, lazy reading and inactive keys.
