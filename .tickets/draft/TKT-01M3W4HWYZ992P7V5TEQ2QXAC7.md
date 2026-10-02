---
schema: 3
id: TKT-01M3W4HWYZ992P7V5TEQ2QXAC7
title: "Pending drafts: keep, edit, or drop what agents drafted"
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
  - TKT-01M3W4HWTXC6EM9SS84A4WMEPG
  - TKT-01M3YK0CM0SWMYXMM7A7M7J2QX
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

The pending-draft surface in the change view. Drafts from reviewers and from the
synthesizer appear at their lines, or in the summary when they have no anchor.
Each is marked with the tool and model that drafted it, and the synthesizer's
drafts show their sources. The person keeps, edits, or drops each one, and
submits through the same control as their own comments. Nothing is posted by an
agent.

## Acceptance criteria

- [ ] Agent findings appear as pending drafts anchored at file and line
- [ ] A drafted comment is marked as drafted until the person edits or accepts it
- [ ] No code path lets an agent run publish drafts, and a test proves it
