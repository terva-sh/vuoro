---
schema: 3
id: TKT-01M49AZ7K2AYBQR1SVPX7BGK13
title: Share one DiffBrowser between the change and the editor's diff
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

change.tsx and EditorDiff each hold the picked-file state, scroll-into-view, and FileList beside FileView. Extract a DiffBrowser that takes per-file threads, drafts, and badges.

## Acceptance criteria

- [ ] DiffBrowser exists, and the change view and the editor's diff use it
- [ ] Picking a file opens and scrolls to it in both, as tested today
