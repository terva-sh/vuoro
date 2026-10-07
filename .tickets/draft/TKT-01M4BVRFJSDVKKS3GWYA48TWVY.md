---
schema: 3
id: TKT-01M4BVRFJSDVKKS3GWYA48TWVY
title: Fold fixed findings into a Fixed group, linked to their commits
type: task
status: draft
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
updated_at: 2026-10-07T18:59:35Z
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

- [ ] Fixed findings fold into a Fixed group with their commit, linked on the forge
- [ ] A fixed finding shows no reviewed-head warning
- [ ] The warning says when the commits since the review are all vuoro's own fixes
