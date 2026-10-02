---
schema: 3
id: TKT-01M3YK0CNGAX0M1Y755995W7QN
title: Talk to the reviewing agent from the change view
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - agent
  - ui
  - review
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies:
  - TKT-01M3YK0CM0SWMYXMM7A7M7J2QX
  - TKT-01M3W4HWSGA04D4VQ8WHBEJC2D
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-02T15:16:29Z
updated_at: 2026-10-02T15:16:29Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
extensions: {}
---

## Description

A review run is a conversation, not a report. The change view keeps the
session open in a panel. The person can ask why, ask the agent to look at
something else, or ask it to drop, merge, or reword a finding. The agent's
draft edits arrive through the channel the spike chose, and they show as
edits to the pending drafts. Tool calls are visible as they happen.

Closing the change view keeps the session, and reopening resumes it.

## Acceptance criteria

- [ ] A follow-up question about a finding streams an answer in the panel
- [ ] Asking the agent to reword or withdraw a finding changes the pending draft, marked as drafted
- [ ] Reopening a change resumes its review conversation
