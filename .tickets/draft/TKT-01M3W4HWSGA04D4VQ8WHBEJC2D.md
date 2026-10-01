---
schema: 3
id: TKT-01M3W4HWSGA04D4VQ8WHBEJC2D
title: "Change view: diff, threads, and review state"
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - review
  - ui
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies:
  - TKT-01M3W4HWNGYDHGXMSE91HKGTNT
  - TKT-01M3W4HWGGB7RMVFAJPW2KPKY5
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-01T16:25:25Z
updated_at: 2026-10-01T16:25:25Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
extensions: {}
---

## Description

Read one change in place: description, the diff by file with threads anchored
at their lines, general discussion, approvals, and pipeline status as a link.
Show "since my last review" as a diff mode when the forge records the SHA.

Large diffs need care. Collapse generated and very large files, and load
files on demand.

## Acceptance criteria

- [ ] A change's diff renders with every anchored thread shown at its line
- [ ] The since-my-last-review mode shows only commits after my last review
- [ ] A 5,000-line diff stays responsive, which means collapsed files load on demand
