---
schema: 3
id: TKT-01M4ES1EPBYF4Q2VQAFQ5YK7G0
title: Record a suggested fix once the author applies it
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - review
  - state
assignees: []
milestone: null
parent: TKT-01M4ES1E4BJAZ78GB96H20W1RD
origin: null
dependencies:
  - TKT-01M4ES1EGCHKQXWYS9FY4QYQCC
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-08T22:09:46Z
updated_at: 2026-10-08T23:09:45Z
created_by:
  id: agent:claude/editor-suggestions-2026-10-08
  name: ""
updated_by:
  id: agent:claude/editor-suggestions-2026-10-08
  name: ""
extensions: {}
---

## Description

D59: a suggested fix is detected once the author applies it. "Mark fixed…" and "Review again" already read the change's new head from the cache. When the change has moved, vuoro checks each suggested finding: is every replacement in the file at the new head, with the lines that bordered it? If so, it records the oldest commit since the review after which they are present as FixedIn, with FixedBy "suggestion". The earlier-findings standing then reads fixed. A link made by a suggestion can be taken back like one the person made.

## Acceptance criteria

- [x] A suggested finding whose replacement is in place at a later head is recorded fixed by the commit that brought it, which a test proves
- [x] A replacement not in place, or in place without its bordering lines, leaves the finding suggested

## Summary

Implemented as `Manager.landSuggestions` in review/suggest.go. It runs from FixCommits (behind "Mark fixed…") and from Follow ("Review again") on the earlier run, whenever the change's head has moved.

For each kept finding that is suggested and not fixed, it checks that every SuggestedHunk is in place at the new head. "In place" means the replacement lines sit between the head lines that bordered them, `Above` and `Below`. A hunk with no line above must be at the file's start, and one with no line below at its end. When every hunk is in place, vuoro walks the commits since the review, oldest first, and cites the first commit at which they all are in place: `CiteFix`, with `FixedBy` "suggestion". It saves that commit's patch with By "suggestion". ClearFix also takes back a fix linked by a suggestion, and the window shows "its suggested lines are in place" with "Not fixed". Earlier standings read "fixed" through FixedIn, as before.

Tested: TestAnEditorWaveGoesBackAsSuggestions now goes on to have the author apply d1 on the remote, then add d2's text away from its suggested place. d1 is cited to the applying commit, d2 stays suggested, the patch is saved, and the link can be taken back. TestASuggestionIsInPlaceOnlyBetweenItsBorders covers the border and file-edge rules. The Follow path is the same call and has no test of its own.
