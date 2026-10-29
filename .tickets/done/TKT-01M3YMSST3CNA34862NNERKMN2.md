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
updated_at: 2026-10-29T18:57:28Z
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

## Notes

**agent:claude-code/release-0.1.0** at 2026-10-29T18:57:28Z

vuoro review round 1 (run 20261029-185205-9a4f4b): a turn stopped before Claude Code reported usage still counted as a known cost (medium); now flagged in both the review and summary pumps and persisted. Also: the headless report prints a known zero as $0.00, plan 6.7 documents run.cost_unknown, and the tour, the Start a review text, and the editor's working note no longer promise mid-turn talk.

## Summary

Sessions show tokens and cost, updated as they run, and runs show the total, as they already did. A cost the tool did not report is never $0.00 (D73). A session's cost is unknown when it used tokens with no cost, or when a turn was stopped or failed before the tool reported its usage (Claude Code reports usage only as a turn completes); that flag is kept across restarts. A session shows 'cost unknown' or 'at least $x' beside its tokens, and a run, the runs card, and the headless report do the same, with run.cost_unknown in the -json envelope. Tested in ui.test.tsx, runlist.test.tsx, TestACostTheToolDidNotReportIsNeverZero and TestATurnStoppedBeforeItsUsageLeavesTheCostUnknown, each failing without its fix.
