---
schema: 3
id: TKT-01M49F69YPPFF84P65TX6S9Y80
title: Add a project or root with the native folder chooser
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - desktop
assignees: []
milestone: null
parent: TKT-01M49F68YV694W81S3ESJKVH02
origin: null
dependencies:
  - TKT-01M49F69S5V5859RTJFWEQ6YNE
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

Bind tuohi's dialog.Open with TypeDirectory on the window's UI thread, and use it from the Local tab's Add buttons. -browser mode keeps the typed path.

## Acceptance criteria

- [ ] Add opens the native directory chooser in the window
- [ ] A cancelled chooser adds nothing
- [ ] -browser mode still takes a typed path
