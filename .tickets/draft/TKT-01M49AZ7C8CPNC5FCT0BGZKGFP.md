---
schema: 3
id: TKT-01M49AZ7C8CPNC5FCT0BGZKGFP
title: Add a shared Modal for the backdrop, Escape, and focus
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - ui
assignees: []
milestone: null
parent: TKT-01M49AYNBWFRYZSBAD5BBJKTB5
origin: null
dependencies:
  - TKT-01M49AZ6QVMSD43KAB1V927RAN
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-06T19:27:41Z
updated_at: 2026-10-06T19:27:41Z
created_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
updated_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
extensions: {}
---

## Description

ConfirmDialog and PushDialog each build the backdrop, Escape handling, initial focus, and aria wiring. Add a Modal primitive and build both on it, so every later dialog is accessible by default.

## Acceptance criteria

- [ ] Modal exists with a test for Escape, backdrop click, and initial focus
- [ ] ConfirmDialog and PushDialog use it
