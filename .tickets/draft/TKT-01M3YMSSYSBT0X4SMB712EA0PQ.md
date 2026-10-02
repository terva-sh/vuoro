---
schema: 3
id: TKT-01M3YMSSYSBT0X4SMB712EA0PQ
title: Triage the threads on my change, and draft replies
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - agent
  - review
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies:
  - TKT-01M3YMSSRC81M0BYHK15TW9VDY
  - TKT-01M3YMSSPF763ZMG6P9FAWQYXK
  - TKT-01M3W4HWSGA04D4VQ8WHBEJC2D
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

The author's side of the MVP. On a change the person wrote, an agent reads the
open threads, groups them as must fix, discuss, or nit, and drafts replies
through the drafts MCP server. The person edits and posts. Fixing code is the
editor agent's job, at v1.0.0.

## Acceptance criteria

- [ ] Open threads on my change are grouped as must fix, discuss, or nit, with the reason for each
- [ ] Reply drafts appear on their threads, marked as drafted, and post only on my action
