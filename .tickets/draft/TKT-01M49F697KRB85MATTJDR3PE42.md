---
schema: 3
id: TKT-01M49F697KRB85MATTJDR3PE42
title: Keep local projects and scan roots, and list each one's branches
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - state
  - review
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
updated_at: 2026-10-06T20:41:27Z
created_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
updated_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
extensions: {}
---

## Description

Section 6.8: local.json in the state directory holds added projects, scan roots, hidden projects, and each project's base. A bounded scan finds checkouts under roots. GET /api/local lists projects keyed by git common directory, each branch with commits beyond the base, its worktrees and their dirty mark, its last run, and whether it needs the person. git runs read-only with hooks, fsmonitor, external diff, and textconv off.

## Acceptance criteria

- [ ] Projects can be added, removed, and hidden, and roots added and removed, through routes behind the guard; paths must be absolute
- [ ] The scan honours depth 4, no symlinks, skipped directories, 500 checkouts, and ten seconds, and reports when it stopped
- [ ] A main checkout and its worktrees are one project
- [ ] Each branch row carries ahead count, newest commit age, worktree tags with a dirty mark, its last run, and why it needs the person
- [ ] Tests run against temporary repositories, with no network
