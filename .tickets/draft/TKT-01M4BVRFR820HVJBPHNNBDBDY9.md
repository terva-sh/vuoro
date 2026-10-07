---
schema: 3
id: TKT-01M4BVRFR820HVJBPHNNBDBDY9
title: Start another editor wave on a review from its last push
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - review
  - agent
assignees: []
milestone: null
parent: TKT-01M4BVRFAY1DN7317TACJXXXRA
origin: null
dependencies:
  - TKT-01M4BVRFD32Q0HM8X6NTTZSWSD
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-07T18:59:35Z
updated_at: 2026-10-07T18:59:35Z
created_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
updated_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
extensions: {}
---

## Description

D50, third bullet. After an editor's work is pushed, the person picks more findings and starts another editor on the same review.

- The run keeps a list of waves (EditorState per wave), the newest last; only the newest can be pushed or discarded, and unpushed work blocks a new wave.
- A wave starts from the last pushed head, on the same branch: the own branch (D41/D42 on a merge request), the stacked change's branch (D39), or the local vuoro/fix-<run> branch (D42), which is updated, not opened again.
- StartEditor accepts a change head equal to the reviewed head or to the newest wave's pushed SHA, and refuses one anyone else moved, with "review again".
- Findings fixed in an earlier wave are not offered for picking.

## Acceptance criteria

- [ ] After a push, another editor starts on the same review with findings not yet fixed
- [ ] A new wave starts from the last pushed head and updates the same branch or stacked change
- [ ] A change someone else moved refuses a new wave and asks for a review again
- [ ] The run shows each wave with its findings, commits, and push
