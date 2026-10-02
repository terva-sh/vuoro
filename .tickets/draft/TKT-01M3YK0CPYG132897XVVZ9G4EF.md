---
schema: 3
id: TKT-01M3YK0CPYG132897XVVZ9G4EF
title: Run reviewers in parallel, and synthesize their drafts
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
  - TKT-01M3YK0CM0SWMYXMM7A7M7J2QX
  - TKT-01M3YK0CNGAX0M1Y755995W7QN
  - TKT-01M3YMSSRC81M0BYHK15TW9VDY
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-02T15:16:29Z
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

Moved into the MVP on 2026-10-02: vuoro is an orchestrator, and this is the
orchestration. A preset names several reviewers, each a tool, a model, and a
profile, and they run as parallel sessions on one change, each in a pane the
person can talk to. When they finish, a synthesizer session lists every
reviewer's drafts through the drafts MCP server. It proposes merged drafts that
name their sources, and it says what it set aside and why.

Prior art: the review action's panels, and terva's raati. A finding that only
one reviewer raised is kept and labelled. It is not dropped by a vote.

## Acceptance criteria

- [ ] A preset of three reviewers on two drivers runs in parallel on one change
- [ ] The synthesizer's merged drafts name their sources, and set-aside findings are listed with a reason
- [ ] A finding only one reviewer raised is kept and labelled
