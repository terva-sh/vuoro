---
schema: 3
id: TKT-01M3YMSST3CNA34862NNERKMN2
title: Show tokens and cost per session and per review run
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - agent
  - ui
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies:
  - TKT-01M3YK0CH5RHPJJS7E0DX9SH8X
  - TKT-01M3YK0CJM8AJQH8129KEEF9TM
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-02T15:47:50Z
updated_at: 2026-10-02T15:47:50Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
extensions: {}
---

## Description

Parallel review runs spend real money or quota. Every session pane shows its
tool, model, tokens, and cost as the tool reports them: `usage` from
`terva rpc`, and the result message from Claude Code. The run shows a total.
Caps come at v1.0.0.

## Acceptance criteria

- [ ] Each session pane shows tokens and cost, updated as the session runs
- [ ] A review run shows the total across its sessions
- [ ] A tool that reports no cost shows tokens only, labelled, never a guessed cost
