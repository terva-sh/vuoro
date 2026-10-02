---
schema: 3
id: TKT-01M3Z7WCX5A15ZC1C6715QWT3Q
title: Hand kept findings to a fixing agent as a Markdown report
type: task
status: draft
status_reason: null
priority: high
due_on: null
labels:
  - state
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
updated_at: 2026-10-02T21:21:18Z
created_by:
  id: agent:claude/vuoro-mvp-1
  name: ""
updated_by:
  id: agent:claude/vuoro-mvp-1
  name: ""
extensions: {}
---

## Description

D31. The kept findings of a run as a Markdown brief for an agent that fixes
them. For each finding it gives the change, the head, file and line, severity,
confidence, title, description, evidence, and the suggestion if there is one.
The UI copies it with one button, and vuoro writes it to runs/<id>/report.md
and shows the path.

## Acceptance criteria

- [ ] The report lists every kept finding and nothing else, which a test proves
- [ ] One button copies it, and the file in the run holds the same text
