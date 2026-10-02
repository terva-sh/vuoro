---
schema: 3
id: TKT-01M3YMSSVNA1TJVQ425EBHSWZ7
title: Pin review runs to their head commit, and flag them when it moves
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - review
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies:
  - TKT-01M3YK0CM0SWMYXMM7A7M7J2QX
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-02T15:47:50Z
updated_at: 2026-10-02T15:47:50Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
extensions: {}
---

## Description

A review run reviews one head SHA. When the change gains commits, vuoro flags
the run and each of its drafts as "reviewed at abc123, 2 commits behind". It
never re-runs on its own. Re-anchoring drafts and reviewing only the new
commits come at v1.0.0.

## Acceptance criteria

- [ ] A run records the head SHA it reviewed
- [ ] After a push, the run and its drafts show how many commits behind they are
- [ ] Nothing re-runs automatically
