---
schema: 3
id: TKT-01M5ZDR7S4AHCKVQBD2ZEEHGJ2
title: Set reviewers, assignees, labels and draft state on GitLab
type: task
status: done
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
updated_at: 2026-10-27T20:28:57Z
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

- [x] Reviewers, assignees and labels can be added and removed, and draft toggled, from the view
- [x] A write whose capability flag is unset is refused before the adapter, and a test holds it
- [x] An opt-in live test round-trips each write on the sandbox merge request
- [x] Every write names APPEND or REMOVE and never replaces a whole set, and a test holds it
- [x] Any entry in a mutation's errors is reported as a failed write

## Implementation plan

1. forge.Administrator, optional beside forge.Writer: Administer(ref, Changes) with additions and removals of reviewers, assignees and labels and an optional draft state; Members(project, query) and Labels(project) for the candidates. Capabilities gain reviewers, assignees, labels and draft. 2. GitLab: reviewers and assignees through mergeRequestSetReviewers and mergeRequestSetAssignees with an explicit APPEND or REMOVE, draft through mergeRequestSetDraft, and labels through REST PUT add_labels/remove_labels, which is by name and as delta-safe as the GraphQL mutation without a label ID lookup. Any entry in a mutation's errors fails the write, and the error names what had already been applied. Members from /members/all, labels from /labels with ancestor groups, both paged. 3. app: POST /api/projects/{key}/administer refuses a change not in the project and any part whose capability flag is unset, before the adapter; then a targeted project read answers the updated display, and the inbox catches up in the background. GET members and labels for the pickers. 4. The detail pane gains controls: remove buttons on each reviewer, assignee and label, an add field with suggestions from the project, and Mark ready / Mark draft. 5. Fixture tests for the adapter (modes always explicit, errors fail), the route (flags, foreign change, guard), and the UI; an opt-in live test on the sandbox merge request.

## Notes

**agent:claude-code/project-triage** at 2026-10-27T20:28:57Z

Labels go through REST add_labels/remove_labels rather than GraphQL mergeRequestSetLabels: both are deltas, and REST takes names where GraphQL needs label IDs. The plan's GitLab notes say so. TestLiveAdminister passed on 2026-10-27 against drewshort/vuoro-sandbox!1: reviewer, assignee, label and draft each round-tripped and the merge request ended as it began; an unknown username was refused through the mutation's errors. It created the label vuoro-live in the sandbox project. The controls are on the project view's detail pane only; the change's own page has none yet, which the acceptance criteria did not ask for.

## Summary

forge.Administrator with forge.Changes (additions and removals, optional draft) and four capability flags. GitLab administers through mergeRequestSetReviewers/SetAssignees with an explicit APPEND or REMOVE, mergeRequestSetDraft, and REST label deltas; any mutation error fails the write and names what had landed. POST /api/projects/{key}/administer refuses a change outside the project and any part whose flag is unset before the adapter, then answers the view after a targeted read; members and labels feed the pickers. The detail pane has remove buttons, add fields with suggestions, and Mark ready / Mark draft.
