---
schema: 3
id: TKT-01M3W4HX1PHPQ98KX0PE5HW60E
title: Show a change's ticket, and record review dispositions on it
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - git-ticket
  - review
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies:
  - TKT-01M3W4HX0BNXTDC2DXBH5HQ0FM
  - TKT-01M3W4HWSGA04D4VQ8WHBEJC2D
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

When a change links to a ticket in a known local store, show its description,
acceptance criteria, plan, and newest note beside the change. Let the reviewer
record each finding's disposition as a ticket note. The note is written to the
working tree and never committed, as git-ticket-canvas does.

Without a store, vuoro behaves exactly as it does without this ticket.

## Acceptance criteria

- [ ] A linked change shows its ticket's acceptance criteria in the change view
- [ ] A disposition note is written with an attributed actor, and git status shows it uncommitted
- [ ] With no store configured, nothing in the UI mentions tickets
