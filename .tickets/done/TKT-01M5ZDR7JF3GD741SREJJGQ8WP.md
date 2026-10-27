---
schema: 3
id: TKT-01M5ZDR7JF3GD741SREJJGQ8WP
title: Show a project's open changes grouped by where they sit
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - ui
assignees: []
milestone: v1.0.0
parent: TKT-01M5ZDQ3ZB0K6KMW2CB8NM183S
origin: null
dependencies:
  - TKT-01M5ZDR7B83Y7SJCRVDWDQWWC8
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-27T19:35:17Z
updated_at: 2026-10-27T20:23:14Z
created_by:
  id: agent:claude-code/project-triage
  name: ""
updated_by:
  id: agent:claude-code/project-triage
  name: ""
extensions: {}
---

## Description

The window's view of one project (plan section 6.9). Changes group by where they sit, oldest first in each group. A row shows the age badge (the inbox's colours, always with the number), title and reference, author, reviewers and their states, assignee, labels, unresolved threads, pipeline state, the turn, and its stack. Filters by label, author, reviewer and stack. A stack is drawn as one chain, and a change blocked only by the one beneath it says so.

The selected row fills a detail pane with its reasons and waiting threads. A row opens the existing review workspace or links to the forge; merging links out (D65). Routes sit behind the window guard like every other.

## Acceptance criteria

- [x] Changes group by where they sit, oldest first, with ages never shown by colour alone
- [x] Filters by label, author, reviewer and stack narrow the list
- [x] A stack draws as one chain, and its top says it waits on the change beneath
- [x] A row opens the review workspace or the forge, and merge is a link to the forge
- [x] The view's routes go through the window guard, and a test holds it

## Implementation plan

A third home tab, Projects (web/src/projects.tsx), kept in prefs like the others. Its head lists the person's projects and an Add panel: search every configured account, or paste a project URL. The selected project reads its cached view, then refreshes once on opening, on r or the button, and every five minutes while the tab shows; nothing refreshes while it is hidden. The list groups rows by place in the derivation's order, oldest known wait first, with the inbox's age badge, which always carries a number or ?; an unknown age shows when the change was opened beside it. A summary strip counts each place and filters, and selects narrow by label, author, reviewer and stack. Rows mark reviewer states, pipeline, conflicts, threads and stack with a symbol and a word, never colour alone. The detail pane shows the reasons, who holds it, reviewers, labels, threads, and the stack drawn bottom to top with the selected change marked; a change waiting only on the one beneath it says that reviewing that one unblocks it. Open in vuoro opens the review workspace; merge is a link to the forge. j/k/Enter/o/r as in the inbox. Vitest tests with a mocked api; the guard test for the routes landed with the first ticket.

## Notes

**agent:claude-code/project-triage** at 2026-10-27T20:23:14Z

Checked in the window on 2026-10-27 against flywheel-io/infrastructure/terraform (51 open): all six places, stacks from target branches, pipeline and conflict marks, and the detail pane render. Found there: a nil Go slice (holder, when no reviewer was waiting) encoded as JSON null, the render threw, and Preact left each failed render's DOM in place, so switching tabs stacked copies of the tab. Fixed at the source, with a Go test that no list in the display encodes as null, and a Home test that the tab mounts once. The project and its cache were removed from the person's state afterwards.

## Summary

web/src/projects.tsx: the Projects home tab, adding a project by search or URL, the selected project's changes grouped by place with the inbox's age badge, place chips and label/author/reviewer/stack filters, symbol-and-word marks for reviewer states, pipeline, conflicts, threads and stacks, and a detail pane that draws the stack bottom first and says when reviewing the change beneath unblocks this one. Merge is a link to the forge.
