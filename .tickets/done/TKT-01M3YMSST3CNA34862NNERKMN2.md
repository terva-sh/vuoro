---
schema: 3
id: TKT-01M3YMSST3CNA34862NNERKMN2
title: Show tokens and cost per session and per review run
type: task
status: done
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
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-02T15:47:50Z
updated_at: 2026-10-29T18:49:00Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude-code/release-0.1.0
  name: ""
extensions: {}
---

## Description

Parallel review runs spend real money or quota. Every session pane shows its
tool, model, tokens, and cost as the tool reports them: `usage` from
`terva rpc`, and the result message from Claude Code. The run shows a total.
Caps come at v1.0.0.

## Acceptance criteria

- [x] Each session pane shows tokens and cost, updated as the session runs
- [x] A review run shows the total across its sessions
- [x] A tool that reports no cost shows tokens only, labelled, never a guessed cost

## Summary

Sessions show tokens and cost, updated as they run, and runs show the total, as they already did. A cost the tool did not report is no longer $0.00 (D73): a session that used tokens with no cost shows 'cost unknown' beside its tokens, and a run, the runs card, and the headless report show 'at least $x' or 'unreported'. RunView.cost_unknown carries it; cost() and unreported() in web/src/ui.tsx render it. Tested in ui.test.tsx, runlist.test.tsx, and TestACostTheToolDidNotReportIsNeverZero, each failing without the fix.
