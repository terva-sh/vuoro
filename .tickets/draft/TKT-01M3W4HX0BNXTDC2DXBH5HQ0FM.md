---
schema: 3
id: TKT-01M3W4HX0BNXTDC2DXBH5HQ0FM
title: Decide how a change finds its git-ticket store and ticket
type: spike
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - git-ticket
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-01T16:25:25Z
updated_at: 2026-10-01T16:25:25Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
extensions: {}
---

## Description

docs/plan.md section 7 makes git-ticket integration optional. Before building
it, decide:

- How vuoro maps a forge project to a local checkout: configured paths, a
  scan of roots like git-ticket-canvas `-root`, or both.
- How a change links to a ticket: a `TKT-` ID in the title, branch, or
  description, or a `pr:` / `mr:` reference on the ticket. What wins when
  they disagree?
- Which checkout to read when the change's branch is not checked out. The
  ticket on the change's head may differ from main.
- What a disposition note looks like, and which actor it is written as.

Use the `ticket` library package, as git-ticket-canvas does. Do not shell out.

## Acceptance criteria

- [ ] docs/plan.md section 7 records the decisions
- [ ] Linking works for a change whose title or branch names a TKT- ID, demonstrated against this store
