---
schema: 3
id: TKT-01M49AZ7K2AYBQR1SVPX7BGK13
title: Share one DiffBrowser between the change and the editor's diff
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
updated_at: 2026-10-06T19:56:01Z
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

- [x] DiffBrowser exists, and the change view and the editor's diff use it
- [x] Picking a file opens and scrolls to it in both, as tested today

## Summary

diffview.tsx gains useFilePicker, which keeps the file picked last and scrolls to it, and DiffBrowser, which draws the FileList beside a FileView per file, takes per-file notes (threads, drafts, mine) and badges, and collapses every file past manyFiles. The picked state is a hook rather than DiffBrowser's own, because the change view's picks also come from findings, which switch the tab first. The change view and the editor's diff both use them, and a new test covers listing, marking, and opening a collapsed file on a pick; the existing pick tests for both pass unchanged.
