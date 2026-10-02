---
schema: 3
id: TKT-01M3YK0CH5RHPJJS7E0DX9SH8X
title: Agent driver for Claude Code over stream-json
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
  - TKT-01M3YK0CFFKAPM2QHZGGZMDHND
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

Implement the driver interface from the spike for Claude Code: spawn
`claude -p` with stream-json in and out, the spike's lockdown flags, and a
session ID so a conversation can resume. Map its events onto vuoro's. Read the
model and account kind from its init event, and record them on the run.

vuoro passes no model credential. Claude Code uses its own login or key.

## Acceptance criteria

- [ ] A two-turn conversation streams through the driver against a recorded diff
- [ ] Cancel stops the subprocess and the turn ends as cancelled
- [ ] Tests run without the network, from a recorded stream
- [ ] A missing or too-old claude binary produces a message that says which and what version is needed
