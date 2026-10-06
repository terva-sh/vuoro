---
schema: 3
id: TKT-01M477XH7NQWFKXCR43STDZ6T4
title: Fix the findings of a review of your own work
type: task
status: done
status_reason: null
priority: high
due_on: null
labels:
  - agent
  - review
assignees: []
milestone: v1.0.0
parent: null
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-05T23:55:51Z
updated_at: 2026-10-06T00:03:30Z
created_by:
  id: agent:claude-code/editor-20261005
  name: ""
updated_by:
  id: agent:claude-code/editor-20261005
  name: ""
extensions: {}
---

## Description

D42. Flow three: the person reviews their own work and has the editor fix what the review found.

- On their own merge request, a review run's findings are picked for the editor or dismissed. Keep is refused there, so nothing is posted. The editor's commits carry `Vuoro-Finding` and push to the merge request's branch, fast-forward only, as D41's do.
- On a local range, the same picks, and the push writes a new branch `vuoro/fix-<run>` into the person's repository, refusing a name that exists, with hooks off, touching nothing else.

## Acceptance criteria

- [x] On the person's own merge request, a review's findings are picked for the editor or dismissed, Keep is refused, and the fixes push to the merge request's branch fast-forward only
- [x] On a local range, the fixes become a new branch in the person's repository, an existing branch is never overwritten, and nothing else in the repository changes
- [x] Tests cover both destinations and the refused Keep

## Summary

A review of the person's own work posts nothing: Keep is refused on their merge request, findings are picked (above low and not doubted, by default) or dismissed locally, and the editor fixes the picks. On a merge request the fixes push to its branch fast-forward; on a local range, which now opens on its own page, they become vuoro/fix-<run> in the person's repository, created with a lease so no branch of theirs moves, with their hooks off. Tested in review, app and web; the local page and picking checked in the window. Local runs from before this record no checkout and must be reviewed again.
