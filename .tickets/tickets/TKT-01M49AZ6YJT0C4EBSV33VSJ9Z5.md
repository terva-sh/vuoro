---
schema: 3
id: TKT-01M49AZ6YJT0C4EBSV33VSJ9Z5
title: Share one TextEditor and EditForm across every text box
type: task
status: ready
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
updated_at: 2026-10-06T19:49:52Z
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

- [ ] TextEditor and EditForm exist with their own tests
- [ ] Composer, ReplyCard, Talk, PushDialog, and HandOff use them
- [ ] One CSS rule styles every text box
