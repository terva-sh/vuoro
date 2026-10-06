---
schema: 3
id: TKT-01M49F69K6E6NECKP8KCSG977C
title: Split the home page into Reviews and Local tabs
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - ui
assignees: []
milestone: null
parent: TKT-01M49F68YV694W81S3ESJKVH02
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-06T20:41:27Z
updated_at: 2026-10-06T20:41:27Z
created_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
updated_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
extensions: {}
---

## Description

Section 6.8: two tabs at the top of the home page, each with a count of what needs the person, and the last tab used kept in prefs.json.

## Acceptance criteria

- [ ] Reviews holds today's inbox unchanged; Local holds the local view
- [ ] Each tab shows its needs-you count
- [ ] The last tab used is restored after a restart
