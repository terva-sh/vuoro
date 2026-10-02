---
schema: 3
id: TKT-01M3W4HWXJYKCNXM4G7E86YX1M
title: "Agent: summarise a change, and what changed since my review"
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - agent
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies:
  - TKT-01M3W4HWSGA04D4VQ8WHBEJC2D
  - TKT-01M3YK0CH5RHPJJS7E0DX9SH8X
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-01T16:25:25Z
updated_at: 2026-10-02T16:51:18Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-mvp-1
  name: ""
extensions: {}
---

## Description

The first agent action, chosen because it is read-only and useful on every
change. From the change view, ask for a summary of the change, or of the
commits since my last review, through the driver the person chose. Output
streams into a panel and is never posted anywhere.

## Acceptance criteria

- [ ] A summary streams into the change view for a real GitLab change
- [ ] The since-my-review summary covers only commits after my last review
- [ ] Cancelling stops the stream and the provider call
