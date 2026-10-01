---
schema: 3
id: TKT-01M3W4HWPT9NYP4ADW339FJDSC
title: Derive the turn for each change, with its reasons
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - inbox
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies:
  - TKT-01M3W4HWJZ1013S2MPZ69B72AW
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

The `turn` package: pure functions from the decided model to `mine` or
`theirs`, a `since` time, and the reasons. No I/O. The reasons are the product
(docs/plan.md section 2.1), so each is a value a person can read, such as
"Mason replied on 2 threads", not a code.

## Acceptance criteria

- [ ] Every turn rule in docs/plan.md section 4 has a table test, built from recorded fixtures
- [ ] Every result carries at least one human-readable reason, and a test proves it
- [ ] The package imports no forge adapter, server, or network code
