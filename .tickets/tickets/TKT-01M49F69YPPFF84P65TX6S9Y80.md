---
schema: 3
id: TKT-01M49F69YPPFF84P65TX6S9Y80
title: Add, hide, and remove local projects and roots, with the folder chooser
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
parent: TKT-01M49F68YV694W81S3ESJKVH02
origin: null
dependencies: []
blocks_on: none
references: []
claim:
  actor: agent:claude/frontend-modules-2026-10-06
  branch: main
  worktree: /Users/drewshort/Workspace/github.com/terva-sh/vuoro
  commit: d220a011eb5a21d7cafb65b38a5ef54640388412
  session: null
  claimed_at: 2026-10-06T21:32:54Z
  expires_at: null
archive: null
created_at: 2026-10-06T20:41:27Z
updated_at: 2026-10-06T21:36:09Z
created_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
updated_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
extensions: {}
---

## Description

Section 6.8: the Local tab's way in. "Add a folder" and "Add a folder to scan" open the native directory chooser in the window, through a `vuoro.chooseFolder` binding that runs tuohi's `View.Dialog` with `TypeDirectory` and only returns the path; the page posts it to the guarded route as for any edit. `-browser` mode has no binding and takes a typed path. The tab lists what the person added, with Remove, and each project can be hidden and shown again.

Moved here from TKT-01M49F69S on 2026-10-06 so that projects can be added from the window before the branch rows and Review land.

## Acceptance criteria

- [ ] Add opens the native directory chooser in the window
- [x] A cancelled chooser adds nothing
- [x] -browser mode still takes a typed path
- [x] Added folders and roots are listed with Remove, and a project can be hidden and shown again

## Notes

**agent:claude/frontend-modules-2026-10-06** at 2026-10-06T21:36:09Z

Built: the vuoro.chooseFolder binding (main.go, tested with a stub chooser for a pick, a cancel and a failure), the Add buttons, the typed path for -browser, the Folders list with Remove and Show, and Hide on each project (web/src/local.test.tsx). just test-gui passes with the binding bound. Criterion 1 is unticked until someone clicks Add in the real window and sees the macOS panel, which no automated test here can open.
