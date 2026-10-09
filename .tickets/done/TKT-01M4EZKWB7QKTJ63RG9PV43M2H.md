---
schema: 3
id: TKT-01M4EZKWB7QKTJ63RG9PV43M2H
title: Take back a mistaken Keep or Drop
type: bug
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - review
  - ui
assignees: []
milestone: null
parent: null
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-09T00:04:41Z
updated_at: 2026-10-09T00:06:14Z
created_by:
  id: agent:claude/editor-suggestions-2026-10-08
  name: ""
updated_by:
  id: agent:claude/editor-suggestions-2026-10-08
  name: ""
extensions: {}
---

## Description

A mistaken Keep or Drop had no undo. A kept finding became a pending comment on the forge, and nothing in the window took it back, so the person had to delete it on the forge and the run still said kept.

"Take back" on a decided finding deletes its pending comment (DeleteDraft) while that is still pending on the forge, and makes the finding undecided. A submitted comment stays kept. A decision carried from an earlier review, one whose fix was pushed or suggested, or one an unfinished editor wave holds, stands.

## Acceptance criteria

- [x] Take back deletes a still-pending kept comment on the forge and makes the finding undecided, which a test proves
- [x] A submitted comment, a carried decision, a fixed or suggested finding, and one an editor holds keep their decision

## Summary

Added drafts.Store.Undecide (holds the draft like Keep, around a remove callback), Manager.Undecide (refuses a finding an undelivered editor wave holds), and POST /api/runs/{id}/drafts/{did}/undecide. For a kept finding, the route deletes the pending comment, but only when the forge still lists it among the person's pending comments. A dropped finding needs no forge call. DraftCard shows 'Take back' beside kept or dropped. Tested: store refusals (carried, fixed, suggested, failed removal), the app route against the fake GitLab (drop undone, submitted comment refused, pending comment deleted via DELETE draft_notes/77), and vitest.
