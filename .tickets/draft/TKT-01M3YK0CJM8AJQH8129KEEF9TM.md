---
schema: 3
id: TKT-01M3YK0CJM8AJQH8129KEEF9TM
title: Agent driver for terva over terva rpc
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

Implement the driver interface from the spike for terva: spawn `terva rpc`
with the spike's lockdown flags, negotiate with `hello`, and use
`TERVACORE_RPC_TOKEN` so only vuoro can drive the process. Map `text_delta`,
`tool_call`, `tool_result`, `usage`, and `done` onto vuoro's events. Use
`--session PATH` under vuoro's state directory so a conversation can resume.

vuoro passes no model credential. terva uses its own configuration.

## Acceptance criteria

- [ ] A two-turn conversation streams through the driver against a recorded diff
- [ ] Abort stops the turn, and done arrives with no error
- [ ] Tests run without the network, from a recorded stream
- [ ] A missing or too-old terva binary produces a message that says which and what version is needed
