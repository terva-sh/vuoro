---
schema: 3
id: TKT-01M3YMSV9MER5MR0B7P5BCNRD5
title: Per-run budget caps, and a monthly cost total
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - agent
assignees: []
milestone: v1.0.0
parent: TKT-01M3YMSTYMFRDFXD4QSVWHMM8P
origin: null
dependencies:
  - TKT-01M3YMSST3CNA34862NNERKMN2
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-02T15:47:52Z
updated_at: 2026-10-02T15:47:52Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
extensions: {}
---

## Description

An optional budget per review run that stops its sessions at the cap and says
so, and a running monthly total across runs.

## Acceptance criteria

- [ ] A run that reaches its budget stops its sessions and shows why
- [ ] The monthly total matches the sum of the runs' recorded costs
