---
schema: 3
id: TKT-01M49AZ6YJT0C4EBSV33VSJ9Z5
title: Share one TextEditor and EditForm across every text box
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
updated_at: 2026-10-06T19:54:45Z
created_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
updated_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
extensions: {}
---

## Description

Composer (write.tsx), ReplyCard's edit form (triage.tsx), Talk and PushDialog (run.tsx), and HandOff's report each build their own textarea, state, keyboard handling, and CSS rule. Add a TextEditor primitive (autosize, Cmd/Ctrl+Enter to submit, a monospace option, a read-only select-all mode) and an EditForm wrapper (busy, error, Save, Cancel), and build those five on them.

## Acceptance criteria

- [x] TextEditor and EditForm exist with their own tests
- [x] Composer, ReplyCard, Talk, PushDialog, and HandOff use them
- [x] One CSS rule styles every text box

## Summary

textedit.tsx holds TextEditor (grows with its text up to 60vh, Cmd/Ctrl+Enter or Enter-alone submits the enclosing form, a mono option, read-only select-on-focus) and EditForm (fields, Save and Cancel, error, note, and a guard so nothing saves while busy or empty). The editor submits through its form rather than a callback, so the form's guards hold however the person saves. Composer, ReplyCard, Talk, PushDialog's description, and HandOff's report use them; textarea now appears only in TextEditor. One .text-editor rule, with .mono, replaces the four per-site textarea rules; .talk keeps only a flex layout rule. Small differences: Composer's Cancel is disabled while saving, and ReplyCard's Save reads 'Saving…' while busy. Tests cover the keys, read-only select, and the form's guards.
