---
schema: 3
id: TKT-01M4BVRFD32Q0HM8X6NTTZSWSD
title: "Link each fix to its finding: trailer, then ask the editor, then by hand"
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - review
  - agent
assignees: []
milestone: null
parent: TKT-01M4BVRFAY1DN7317TACJXXXRA
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-07T18:59:34Z
updated_at: 2026-10-07T19:12:38Z
created_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
updated_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
extensions: {}
---

## Description

D50, first bullet. A fix is linked to the finding by the commit's trailer, then by asking the editor, then by hand.

- Before a push, compare the picked findings with the commits' Vuoro-Finding / Vuoro-Thread trailers. For each picked finding no commit names, send the editor one turn in its session asking it to amend with the trailer or say why the finding is left alone, and show the person the answer before the push.
- A route lets the person mark a finding fixed by one of the review's pushed commits (a commit vuoro pushed for this run, or one in the change since the reviewed head), recording FixedIn. No MCP tool reaches it; the drafts store's CiteFix stays vuoro's only writer.
- The run view carries each finding's FixedIn with the commit's subject.

## Acceptance criteria

- [x] A push first asks the editor about each picked finding no commit's trailer names
- [x] The person can mark a finding fixed by a commit of the review, and no MCP tool can
- [x] A finding's fixed commit is recorded and shown with its subject

## Summary

Each fix is now linked to its finding, by the trailer, then by asking the editor, then by hand.

- **Push check:** PushEditor (review/editor.go) compares the picked items with the commits' Vuoro-Finding and Vuoro-Thread trailers. For any picked item no commit names and nothing has fixed, it records the item in EditorState.Asked and sends the editor one turn (askAboutTrailers): reword an unpushed commit to add the trailer, or say why the item is left alone. The push then returns AskedError, which is 409. It asks once per item, so the next push goes ahead. The window closes the push dialog and shows the notice while the editor answers.
- **Recorded fix:** a push now records FixedIn, FixedSubject, and FixedBy "editor" in every mode, including stacked changes (D39).
- **By hand:**
  - POST /api/runs/{id}/fixes lists FixCommits: the commits vuoro pushed for the review, plus the change's commits since the reviewed head, fetched into vuoro's clone.
  - POST /api/runs/{id}/drafts/{did}/fixed {sha} records FixedBy "person". An empty sha takes back only a link the person made (Store.ClearFix).
- **UI:** each finding shows "✓ Fixed in <sha> · <subject>" and who linked it, with "Mark fixed…" and "Not fixed".
- **No agent tool:** no MCP tool takes the fields (tested).

Tests are in review/editor_test.go, review/drafts/mcp_test.go, web/src/fixlink.test.tsx, and web/src/editor.test.tsx.
