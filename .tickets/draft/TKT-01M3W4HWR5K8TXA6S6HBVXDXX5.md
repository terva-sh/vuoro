---
schema: 3
id: TKT-01M3W4HWR5K8TXA6S6HBVXDXX5
title: "Inbox view: every change, sorted by whose turn and how long"
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - inbox
  - ui
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies:
  - TKT-01M3W4HWGGB7RMVFAJPW2KPKY5
  - TKT-01M3W4HWNGYDHGXMSE91HKGTNT
  - TKT-01M3W4HWPT9NYP4ADW339FJDSC
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

The main window. Two groups, "my turn" and "their turn", each sorted by
`since`, oldest first. Each row shows the change, the forge and project, my
role, the reason, and the age. A refresh control, and a background refresh on
an interval.

A local snooze is the one authored annotation in v0.1. It hides a change until
a date or until the next forge event, and it is stored in state, never in a
forge.

## Acceptance criteria

- [ ] Changes from the GitLab adapter show in the two groups, with reason and age
- [ ] Opening a row opens the change view, and each row also links to the forge URL
- [ ] A snoozed change returns when its date passes or a new event arrives
