---
schema: 3
id: TKT-01M49F69DQA78FC51MYRH9N1TR
title: Start a local review of a branch from the window
type: task
status: in-progress
status_reason: null
priority: normal
due_on: null
labels:
  - review
assignees: []
milestone: null
parent: TKT-01M49F68YV694W81S3ESJKVH02
origin: null
dependencies:
  - TKT-01M49F697KRB85MATTJDR3PE42
blocks_on: none
references: []
claim:
  actor: agent:claude/frontend-modules-2026-10-06
  branch: main
  worktree: /Users/drewshort/Workspace/github.com/terva-sh/vuoro
  commit: 7e4c2043a9fd921b307c81dcb840888b42096568
  session: null
  claimed_at: 2026-10-06T22:21:50Z
  expires_at: null
archive: null
created_at: 2026-10-06T20:41:27Z
updated_at: 2026-10-06T22:24:41Z
created_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
updated_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
extensions: {}
---

## Description

POST a project, branch, and preset to start section 6.7's <base>...<branch> run in the window. The run stays open as a window run does, and opens in LocalRun. Satisfies the first criterion of TKT-01M3YMSV6BABG7EW1NP8J6W5SR.

## Acceptance criteria

- [ ] A branch review runs the picked preset and opens in the window
- [x] A dirty worktree is reported, and uncommitted changes are not reviewed
- [x] Nothing is written to the checkout

## Notes

**agent:claude/frontend-modules-2026-10-06** at 2026-10-06T22:24:41Z

Server side landed: POST /api/local/review {key, branch, preset} (app/local.go reviewBranch) starts the run and answers {run, uncommitted}. Refresh now calls Manager.LoadFinished, so a finished vuoro review run shows without a restart. Criterion 1's 'opens in the window' is the Review button, which TKT-01M49F69S builds; I tick it there.
