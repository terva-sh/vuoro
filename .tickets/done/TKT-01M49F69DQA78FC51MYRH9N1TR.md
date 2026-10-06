---
schema: 3
id: TKT-01M49F69DQA78FC51MYRH9N1TR
title: Start a local review of a branch from the window
type: task
status: done
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
claim: null
archive: null
created_at: 2026-10-06T20:41:27Z
updated_at: 2026-10-06T22:28:12Z
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

- [x] A branch review runs the picked preset and opens in the window
- [x] A dirty worktree is reported, and uncommitted changes are not reviewed
- [x] Nothing is written to the checkout

## Notes

**agent:claude/frontend-modules-2026-10-06** at 2026-10-06T22:24:41Z

Server side landed: POST /api/local/review {key, branch, preset} (app/local.go reviewBranch) starts the run and answers {run, uncommitted}. Refresh now calls Manager.LoadFinished, so a finished vuoro review run shows without a restart. Criterion 1's 'opens in the window' is the Review button, which TKT-01M49F69S builds; I tick it there.

## Summary

POST /api/local/review {key, branch, preset} starts section 6.7's <base>...<branch> run in the window (app/local.go reviewBranch).

- It accepts only a branch the Local tab lists, and reads the project again, so the base and tip are current.
- It answers {run, uncommitted}, where uncommitted lists the worktrees holding the branch with uncommitted changes.
- The run stays open, and the row's Review button (TKT-01M49F69S) opens it.
- The checkout's refs and status are unchanged by a review (tested).
- Refresh calls Manager.LoadFinished, which picks up runs another process finished (closed, failed, or cancelled) and never one it is still writing.
