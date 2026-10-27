---
schema: 3
id: TKT-01M5ZDR7JF3GD741SREJJGQ8WP
title: Show a project's open changes grouped by where they sit
type: task
status: ready
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
updated_at: 2026-10-27T19:57:04Z
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

- [ ] Changes group by where they sit, oldest first, with ages never shown by colour alone
- [ ] Filters by label, author, reviewer and stack narrow the list
- [ ] A stack draws as one chain, and its top says it waits on the change beneath
- [ ] A row opens the review workspace or the forge, and merge is a link to the forge
- [ ] The view's routes go through the window guard, and a test holds it
