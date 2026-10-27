---
schema: 3
id: TKT-01M5ZDR7S4AHCKVQBD2ZEEHGJ2
title: Set reviewers, assignees, labels and draft state on GitLab
type: task
status: ready
status_reason: null
priority: normal
due_on: null
labels:
  - gitlab
  - forge
  - ui
assignees: []
milestone: v1.0.0
parent: TKT-01M5ZDQ3ZB0K6KMW2CB8NM183S
origin: null
dependencies:
  - TKT-01M5ZDR7JF3GD741SREJJGQ8WP
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

`forge.Administrator`, optional beside `forge.Writer`, with the capability flags `reviewers`, `assignees`, `labels` and `draft` (plan section 6.9, D65), and the GitLab adapter for it. Candidates come from the project: `GET /projects/:id/members/all` (including inherited and invited members, filtered with `query`) for reviewers and assignees, and `GET /projects/:id/labels` (including ancestor groups' labels) for labels.

A write is a list of additions and removals, never a replacement set. On GitLab that is GraphQL `mergeRequestSetReviewers`, `mergeRequestSetAssignees` and `mergeRequestSetLabels`, always with an explicit `operationMode` of `APPEND` or `REMOVE`, because the default is `REPLACE`. Any entry in a mutation's `errors`, such as a username matching no visible user, is a failed write. Draft state is `mergeRequestSetDraft`. REST's `reviewer_ids` and `assignee_ids` replace the whole set and are not used. After the write a targeted read updates the row. A write whose flag is unset is refused before the adapter is called (D44). The controls live on the project view's rows and detail pane, and on the change's own page.

Validate live against the sandbox merge request (`drewshort/vuoro-sandbox`), as `TestLiveWrite` did for review writes.

## Acceptance criteria

- [ ] Reviewers, assignees and labels can be added and removed, and draft toggled, from the view
- [ ] A write whose capability flag is unset is refused before the adapter, and a test holds it
- [ ] An opt-in live test round-trips each write on the sandbox merge request
- [ ] Every write names APPEND or REMOVE and never replaces a whole set, and a test holds it
- [ ] Any entry in a mutation's errors is reported as a failed write
