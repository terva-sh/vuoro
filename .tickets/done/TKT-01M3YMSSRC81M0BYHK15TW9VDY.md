---
schema: 3
id: TKT-01M3YMSSRC81M0BYHK15TW9VDY
title: Serve the drafts MCP server to each agent session
type: task
status: done
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
updated_at: 2026-10-02T17:09:26Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-mvp-1
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

- [x] A synthesizer lists another session's drafts and proposes a merged one that names its sources
- [x] The server exposes no tool that reaches a forge, and a test lists its tools
- [x] An invalid draft returns the rule it broke, and a test proves it
- [x] A Claude Code reviewer session proposes, revises, and withdraws a draft through the server

## Implementation plan

review/drafts holds a per-run Store with validation rules named as in review-result.v1, and a Server that speaks MCP over Streamable HTTP with one JSON response per request. Each session registers for its own bearer token, which fixes its identity and role. server.Config mounts the Server at /mcp/ behind the guard. The tests cover the store rules, the HTTP protocol, and the tool list. A live test has a real Claude Code reviewer use the drafts tools through the loopback server.

## Notes

**agent:claude/vuoro-mvp-1** at 2026-10-02T17:05:41Z

Criterion 1 named each driver. Decision D28 takes terva out of the MVP, so it now names Claude Code. TKT-01M3YRE0E00TRVKXP8YVASHDGD (Prove terva's reviewer lockdown once terva can take a per-run server) carries terva's half.

**agent:claude/vuoro-mvp-1** at 2026-10-02T17:09:26Z

Verified on 2026-10-02:
- The offline tests cover the store's rules: each rule named, no quoting, ownership, and the synthesizer's sources. They also cover the MCP protocol over HTTP (initialize, tools/list, tools/call, a notification gets 202, GET gets 405), and bearer checks for none, wrong, and unregistered tokens. The tool list is exactly the four draft tools for both roles.
- TestLiveClaude (VUORO_LIVE_CLAUDE=1): Claude Code 2.1.285 on Haiku read calc.go, proposed d1 anchored on calc.go, revised it to critical, withdrew it, and listed it through http://127.0.0.1:PORT/mcp/ behind the guard. The driver's lockdown check accepted the session, which offered only the read tools and the four drafts tools.
- The synthesizer criterion is proven against registered sessions over MCP, not with a live synthesizer agent. TKT-01M3YK0CPYG132897XVVZ9G4EF (Run reviewers in parallel, and synthesize their drafts) runs one.

## Summary

review/drafts serves each run's pending drafts to agent sessions over MCP at /mcp/ on the loopback server, behind the guard, with a bearer token per session that fixes its role. Its four tools draft and never post. Invalid proposals come back naming the rule and field. A real Claude Code reviewer proposed, revised, and withdrew a draft through it.
