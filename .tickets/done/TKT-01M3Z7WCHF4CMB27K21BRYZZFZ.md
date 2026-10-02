---
schema: 3
id: TKT-01M3Z7WCHF4CMB27K21BRYZZFZ
title: Verify each finding, and mark its confidence
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
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-02T21:21:18Z
updated_at: 2026-10-02T21:40:40Z
created_by:
  id: agent:claude/vuoro-mvp-1
  name: ""
updated_by:
  id: agent:claude/vuoro-mvp-1
  name: ""
extensions: {}
---

## Description

D31. After the synthesizer, or after the reviewers when there is none, a
read-only verifier session reads every finding the person works from and
checks it against the worktree. It marks each one confirmed, plausible, or
doubtful with a one-line reason, through a drafts tool that only the verifier
role has. The person can override a mark in the UI. Marks show as ✅ ❔ ⛈ beside
a word, never by colour alone.

## Acceptance criteria

- [x] A verifier marks every finding the person works from, with a reason
- [x] Only the verifier role can rate, and a test proves the tool lists per role
- [x] The person can override a mark, and the override is kept

## Implementation plan

A third drafts role, Verifier, whose tool list is list_drafts and rate_draft only; ToolNames(role) builds the driver's allowlist from the same list. Draft gains Confidence (the verifier's) and Override (the person's, which wins). The run gains a verifier session that waits for every reviewer and synthesizer to report, is sent the shown findings by id, and is sent back once for any it left unmarked. Presets gain an optional verifier; the default is the synthesizer's tool and model, else the first reviewer's. POST /api/runs/{id}/drafts/{did}/confidence sets or clears the person's mark. The UI shows ✅ ❔ ⛈ with a word on each finding row, the reason in the body, and buttons for the person's mark.

## Notes

**agent:claude/vuoro-mvp-1** at 2026-10-02T21:40:40Z

Tested with the fake driver (review/run_test.go TestAVerifierMarksEveryFindingAfterTheSynthesizerAndIsSentBackOnce): it starts after the synthesizer, is sent only the merged findings, cannot propose, is nudged once, and the person's mark survives a reload beside the verifier's. drafts tests cover the rating rules and the per-role tool lists over MCP. The UI was checked in an isolated copy of the !144 run by setting and changing a mark. Not yet run live against a real model: the first real run will be the owner's next review, and the verifier adds one session's cost to every run.

## Summary

Every run now ends with a read-only verifier that marks each finding the person works from confirmed, plausible, or doubtful with a reason, through rate_draft, the only tool it has. The person can set their own mark, which wins and is kept beside the verifier's.
