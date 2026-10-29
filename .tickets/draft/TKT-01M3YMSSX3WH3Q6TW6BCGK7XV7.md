---
schema: 3
id: TKT-01M3YMSSX3WH3Q6TW6BCGK7XV7
title: Keep review runs until the change closes, then 30 days
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - state
assignees: []
milestone: v1.0.0
parent: TKT-01M3YMSTYMFRDFXD4QSVWHMM8P
origin: null
dependencies:
  - TKT-01M3YK0CM0SWMYXMM7A7M7J2QX
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-02T15:47:50Z
updated_at: 2026-10-29T18:49:01Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude-code/release-0.1.0
  name: ""
extensions: {}
---

## Description

Review runs, their transcripts, and their draft history live in the state
directory. Keep everything while the change is open, so that a session can
resume and the person can check what each agent said. After the change merges
or closes, keep it 30 days. Then delete the transcripts and keep one line per
run: tools, models, cost, and finding counts. The period is configurable.

## Acceptance criteria

- [ ] A closed change's transcripts are deleted after the configured period, and a test proves it
- [ ] The one-line record per run survives the deletion

## Notes

**agent:claude-code/release-0.1.0** at 2026-10-29T18:49:01Z

Moved from the v0.1.0 epic to v1.0.0 by the owner on 2026-10-29 (D73).
