---
schema: 3
id: TKT-01M3YK0CH5RHPJJS7E0DX9SH8X
title: Agent driver for Claude Code over stream-json
type: task
status: done
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
updated_at: 2026-10-02T17:05:02Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-mvp-1
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

- [x] A two-turn conversation streams through the driver against a recorded diff
- [x] Cancel stops the subprocess and the turn ends as cancelled
- [x] Tests run without the network, from a recorded stream
- [x] A missing or too-old claude binary produces a message that says which and what version is needed

## Implementation plan

The agent package holds the interface and events from docs/plan.md section 6.6. agent/claude spawns claude -p with the spike's lockdown, writes the MCP config to a file of mode 0600 (never argv), and names the session with --session-id or --resume. It maps init, stream_event text deltas, tool_use, tool_result, and result onto agent events, reports each turn's cost as the difference of total_cost_usd, and ends a session whose init offers a tool outside the allowed set. The tests use the test binary as a fake claude that replays the spike's recorded stream.

## Notes

**agent:claude/vuoro-mvp-1** at 2026-10-02T17:05:01Z

Verified on 2026-10-02:
- The offline tests replay the spike's recorded streams with the test binary standing in for claude. They cover two streamed turns, cancel mid-turn, a session whose init offers Bash (ended with ErrLockdown before any event gets through), a denied call, resume, and probe for missing, too old, and current versions. Probe's account kind carries no email.
- TestLive (VUORO_LIVE_CLAUDE=1) held a two-turn conversation with the real claude 2.1.285 over the spike fixture's calc.go. Turn 1 read the file and named the off-by-one bug. Turn 2 had no shell tool, and no canary file appeared. Cost: about $0.02 on Haiku.
- Cancel has been tested only against the fake, not live.

## Summary

agent defines the driver interface, and agent/claude drives Claude Code under the proven reviewer lockdown. It streams turns, reports usage per turn, cancels by stopping the process, and resumes by session ID. It ends any session that offers a tool outside the lockdown. The tests run offline from recorded streams, and an opt-in live test passed against claude 2.1.285.
