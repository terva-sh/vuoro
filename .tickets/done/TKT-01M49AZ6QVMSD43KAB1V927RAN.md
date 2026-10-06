---
schema: 3
id: TKT-01M49AZ6QVMSD43KAB1V927RAN
title: Split run.tsx and diffview.tsx into modules by concern
type: chore
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
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-06T19:27:40Z
updated_at: 2026-10-06T19:29:28Z
created_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
updated_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
extensions: {}
---

## Description

run.tsx (781 lines) mixes run polling and views, findings and marks, the editor with its diff and push dialog, and session transcripts with chat. diffview.tsx mixes diff rendering with the comment cards. Move code only, with no behaviour change.

## Acceptance criteria

- [x] run.tsx keeps RunPanel, RunView, LocalRun, and the run poll
- [x] findings.tsx holds Drafts, Finding, Marks, HandOff, shownDrafts, and sourceLabel
- [x] editor.tsx holds Editor, EditorDiff, PushBar, and PushDialog
- [x] session.tsx holds SessionPane, EntryView, and Talk
- [x] comments.tsx holds ThreadView, MyDraft, DraftCard, and the keep-text helpers, and diffview.tsx keeps the diff
- [x] just ci passes with no test assertion changed

## Summary

Split by slicing on function boundaries, so the code moved unchanged; only Drafts and HandOff gained export. run.tsx (781 lines) is now run.tsx 234 (RunPanel, RunView, LocalRun, the run poll), findings.tsx 175, editor.tsx 266, and session.tsx 132. diffview.tsx keeps the diff (192), and comments.tsx (210) holds ThreadView, MyDraft, DraftCard, and the keep-text helpers. diffview imports the cards from comments, and the run modules only import downward, so there is no cycle. Tests changed import paths only, and just ci passes. Uncommitted, in the same working tree as the theme picker and the diff contrast pass.
