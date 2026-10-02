---
schema: 3
id: TKT-01M3YK0CFFKAPM2QHZGGZMDHND
title: Decide the agent driver interface for terva and Claude Code
type: spike
status: done
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
updated_at: 2026-10-02T16:51:47Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-mvp-1
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

- [x] docs/plan.md section 6 records the interface, Claude Code's lockdown, and the drafts channel, and names terva's lockdown as the one open question
- [x] For Claude Code, a recorded transcript shows the agent refused a command under the chosen lockdown
- [x] A throwaway Go program holds a streamed conversation of two turns or more with Claude Code

## Notes

**agent:claude/vuoro-mvp-1** at 2026-10-02T16:33:49Z

Where the spike stands on 2026-10-02. docs/plan.md section 6.6 has the detail, and spikes/agentdriver/ has the evidence.

Claude Code is settled. The interface and the lockdown flags are in section 6.6, and the recorded transcripts are in spikes/agentdriver/transcripts/. They prove:
- The session offers only Read, Grep, Glob, and the drafts tools, and none of the person's MCP servers or claude.ai connectors.
- dontAsk denies a defined but disallowed Bash, as a second layer.
- --restricted refuses reads outside the worktree.
- The reviewed worktree's CLAUDE.md, AGENTS.md, and hooks stay out. Without the flags, the change's hook ran.
- Drafts arrive through a stub stdio MCP server.
- Resume works across processes with --session-id and --resume.

terva is not settled, and none of the three criteria can be ticked yet, because each one names both tools:
- The workstation's terva login has expired, and no other provider is configured, so no terva transcript exists. The owner signs in again.
- terva 0.139.7 cannot give one run a server that vuoro supplies while it keeps the person's login (no --mcp-config; --team-socket is hard-wired; TERVA_HOME moves auth.json). The person's config hooks always load, and no flag turns them off. Under AGENTS.md's rule, a driver without a proven lockdown is not offered. So terva needs an upstream change before it can be a reviewer, or a decision from the owner. Asked on 2026-10-02.

**agent:claude/vuoro-mvp-1** at 2026-10-02T16:51:47Z

Criteria narrowed on 2026-10-02 with the owner's approval (decision D28). The original three named both tools. The terva half moved to TKT-01M3YRE0E00TRVKXP8YVASHDGD (Prove terva's reviewer lockdown once terva can take a per-run server), under the v1.0.0 epic.

## Summary

The driver interface, Claude Code's reviewer lockdown, and the drafts channel's shape are recorded in docs/plan.md section 6.6. The transcripts in spikes/agentdriver/transcripts prove the lockdown: only the read tools and vuoro's drafts server, a harness denial for a disallowed Bash, reads confined to the worktree, and the reviewed change's CLAUDE.md, AGENTS.md, and hooks kept out. terva 0.139.7 cannot be locked down, so D28 defers it past the MVP, and TKT-01M3YRE0E00TRVKXP8YVASHDGD carries that work.
