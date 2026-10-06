---
schema: 3
id: TKT-01M49AZ7C8CPNC5FCT0BGZKGFP
title: Add a shared Modal for the backdrop, Escape, and focus
type: task
status: done
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
updated_at: 2026-10-06T19:52:39Z
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

- [x] Modal exists with a test for Escape, backdrop click, and initial focus
- [x] ConfirmDialog and PushDialog use it

## Summary

modal.tsx holds Modal: the backdrop, a role=dialog named by its title through useId, initial focus on a ref the caller passes, and Escape and a click outside closing it unless locked. Tests cover all four. ConfirmDialog and PushDialog are built on it, and their hard-coded confirm-title and push-title ids are gone. One difference: PushDialog used to refocus Cancel each time busy changed, and now focuses it once on opening.
