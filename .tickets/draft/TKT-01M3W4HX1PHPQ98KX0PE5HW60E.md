---
schema: 3
id: TKT-01M3W4HX1PHPQ98KX0PE5HW60E
title: Show a change's ticket, and give it to reviewers as context
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
updated_at: 2026-10-02T15:47:51Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
extensions: {}
---

## Description

The MVP half of the git-ticket integration. When a change links to a ticket in
a known local store, show its description, acceptance criteria, plan, and
newest note beside the change, and give them to every reviewer session as
context, so that the review checks the change against what was asked. Writing
dispositions back is v1.0.0.

Without a store, vuoro behaves exactly as it does without this ticket.

## Acceptance criteria

- [ ] A linked change shows its ticket's acceptance criteria in the change view
- [ ] With no store configured, nothing in the UI mentions tickets
- [ ] A reviewer's prompt includes the linked ticket's acceptance criteria
