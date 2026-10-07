---
schema: 3
id: TKT-01M4BVRFJSDVKKS3GWYA48TWVY
title: Fold fixed findings into a Fixed group, linked to their commits
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - ui
  - review
assignees: []
milestone: null
parent: TKT-01M4BVRFAY1DN7317TACJXXXRA
origin: null
dependencies:
  - TKT-01M4BVRFD32Q0HM8X6NTTZSWSD
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-07T18:59:35Z
updated_at: 2026-10-07T19:15:19Z
created_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
updated_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
extensions: {}
---

## Description

D50, second bullet. Fixed findings fold into a "Fixed · N" group on the review page, each with "✓ fixed in <commit>" linked to the commit on the forge (GitLab /-/commit/<sha>, Forgejo /commit/<sha>; a local run shows the short hash), and without the reviewed-head warning. The warning on the others says when every commit since the review is vuoro's own push for it ("2 commits behind, both vuoro's fixes"), instead of reading as the change having moved.

## Acceptance criteria

- [x] Fixed findings fold into a Fixed group with their commit, linked on the forge
- [x] A fixed finding shows no reviewed-head warning
- [x] The warning says when the commits since the review are all vuoro's own fixes

## Summary

Fixed findings now fold into a folded "Fixed · N" group below the open ones (web/src/findings.tsx).

- **Commit link:** each fixed finding's "✓ Fixed in <sha>" links to its commit on the forge, through commitURL (web/src/markdown.tsx): GitLab /-/commit/<sha>, otherwise /commit/<sha>. A local run, with no forge, shows the bare hash.
- **No warning on fixed findings:** neither the row nor its card shows the reviewed-head warning.
- **Summary:** the findings line counts "✓ N fixed" apart, and "to decide" no longer counts fixed findings.
- **vuoro's own commits:**
  - useRunHead takes the editor's newest pushed sha. When the change's head is that push, HeadState.ours is set, and the badge reads "N commits behind, both vuoro's fixes" (or "vuoro's fix", or "all vuoro's fixes"), without the warning mark.
  - Anyone else's push on top shows the warning as before.

Tests are in web/src/editor.test.tsx and web/src/head.test.tsx.
