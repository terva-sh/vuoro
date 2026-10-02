---
schema: 3
id: TKT-01M3YMSSRC81M0BYHK15TW9VDY
title: Serve the drafts MCP server to each agent session
type: task
status: draft
status_reason: null
priority: high
due_on: null
labels:
  - agent
  - review
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

docs/plan.md section 6.2: an agent's only write path is a small MCP server
that vuoro serves to each session. Its tools list, propose, revise, and
withdraw pending drafts. Reviewers see their own drafts. A synthesizer can
list every reviewer's drafts in the run and propose merged ones that name
their sources.

Each tool is pointed at the server through its own MCP configuration, with the
person's MCP servers excluded (`--strict-mcp-config` for Claude Code, and
terva's equivalent from the driver spike). A proposed draft is validated, and a
bad one returns the rule it broke so the agent can correct it.

## Acceptance criteria

- [ ] A reviewer session on each driver proposes, revises, and withdraws a draft through the server
- [ ] A synthesizer lists another session's drafts and proposes a merged one that names its sources
- [ ] The server exposes no tool that reaches a forge, and a test lists its tools
- [ ] An invalid draft returns the rule it broke, and a test proves it
