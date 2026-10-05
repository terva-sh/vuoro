---
schema: 3
id: TKT-01M471G16F1TM5B28G1A9CT2YH
title: Let the editor start on a closed review
type: task
status: done
status_reason: null
priority: high
due_on: null
labels:
  - agent
  - review
assignees: []
milestone: v1.0.0
parent: null
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-05T22:03:37Z
updated_at: 2026-10-05T22:06:25Z
created_by:
  id: agent:claude-code/editor-20261005
  name: ""
updated_by:
  id: agent:claude-code/editor-20261005
  name: ""
extensions: {}
---

## Description

The editor seeds its own repository from vuoro's cache, so it does not need the run's worktree. A review the person has closed, often after submitting it, is exactly when they want the fixes; today the editor refuses a closed run. Found while preparing to hand back gitlab.com flywheel-io/infrastructure/terraform!2487, whose run is closed at an unchanged head.

## Acceptance criteria

- [x] A closed review run with kept findings at the change's current head can start an editor, and the person can talk to it
- [x] When the cache lacks the head's files, vuoro brings them in before seeding, and a missing head commit is refused with the reason

## Implementation plan

Drop the closed and worktree checks in StartEditor. send lets an editor session through on a closed run. SeedEditor hydrates the head's tree when blobs are missing, through a temporary worktree checkout that batches the lazy fetch, and refuses when the head commit itself is gone. The UI offers the button and the talk box for an editor on a closed run.

## Summary

StartEditor accepts a closed run, and send lets only the editor's session through on one. SeedEditor refuses a head the clone lost and hydrates missing files through a temporary checkout; a blob-less fetch test fails without it. !2487's cache holds all 636 objects at d554a377, so its closed run can start an editor as is.
