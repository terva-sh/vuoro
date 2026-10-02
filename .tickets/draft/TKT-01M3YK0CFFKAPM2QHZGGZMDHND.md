---
schema: 3
id: TKT-01M3YK0CFFKAPM2QHZGGZMDHND
title: Decide the agent driver interface for terva and Claude Code
type: spike
status: draft
status_reason: null
priority: high
due_on: null
labels:
  - agent
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies: []
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

vuoro drives agent tools as subprocesses and imports none of them, so the
person chooses which one runs (docs/plan.md section 6). The first two drivers
are terva, over `terva rpc`, and Claude Code, over `claude -p` with
`--input-format stream-json --output-format stream-json`. That stream is the
protocol Claude Code's own SDKs speak.

Run both by hand against one recorded diff, and decide:

- **The interface.** Start a session (working directory, system prompt, tool
  policy, model), send a message, stream events, cancel, close, and resume. The
  event set that both can honour: text, tool call and result, usage and cost,
  done and error, and an approval request.
- **The lockdown, per tool.** The agent reads a worktree and nothing else. It
  gets no shell, no web, no MCP servers of the person's (which can include a
  forge connector that writes), and no extensions. For Claude Code, try
  `--tools Read,Grep,Glob`, `--strict-mcp-config`, `--restricted`, and a
  permission mode. For terva, try `--tools`, `--no-mcp`, `--no-ext`, and
  `--approval plan`. Prove each one: ask the agent to run a command, and watch
  it refuse.
- **How an agent changes drafts.** Either vuoro parses structured output
  (Claude Code has `--json-schema`; terva has none), or vuoro serves the agent
  a small MCP server whose only tools edit vuoro's pending drafts. The second
  works the same for both tools, and it keeps "agent drafts, person posts"
  structural.
- **What a run records.** The tool, its version, the model, and the account
  kind where the tool reports it, so the person can see what their code was
  sent to.
- **Discovery.** How vuoro finds each binary, and how it says that one is
  missing or too old.

Prior art: terva-sh/terva-action-code-review drives `terva rpc` with
`--portable=strict --no-tools --no-memory --no-project --no-session` in
`src/terva.mjs`. It is inspiration, not a dependency.

## Acceptance criteria

- [ ] docs/plan.md section 6 records the interface, the lockdown per tool, and the drafts channel, with the open-question marker removed
- [ ] For each tool, a recorded transcript shows the agent refusing to run a command under the chosen lockdown
- [ ] A throwaway Go program holds a two-turn conversation with each tool, streamed
