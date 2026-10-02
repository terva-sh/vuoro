---
schema: 3
id: TKT-01M3W4HWYZ992P7V5TEQ2QXAC7
title: Turn review findings into pending drafts I edit before posting
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
updated_at: 2026-10-02T15:16:41Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
extensions: {}
---

## Description

Turn the findings of a review run, from TKT-01M3YK0CM0SWMYXMM7A7M7J2QX (Run a
review: worktree, review profile, and validated findings), into pending draft
comments at their lines. A finding with no anchor goes into the draft summary.

Each draft is marked as drafted by an agent, and it names the tool and model
that drafted it. The person edits, deletes, or keeps each one, and submits
through the same control as their own comments. Nothing is posted by the agent.

## Acceptance criteria

- [ ] Agent findings appear as pending drafts anchored at file and line
- [ ] A drafted comment is marked as drafted until the person edits or accepts it
- [ ] No code path lets an agent run publish drafts, and a test proves it
