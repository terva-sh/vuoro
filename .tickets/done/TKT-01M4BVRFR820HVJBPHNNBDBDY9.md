---
schema: 3
id: TKT-01M4BVRFR820HVJBPHNNBDBDY9
title: Start another editor wave on a review from its last push
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
dependencies:
  - TKT-01M4BVRFD32Q0HM8X6NTTZSWSD
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-07T18:59:35Z
updated_at: 2026-10-07T19:27:10Z
created_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
updated_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
extensions: {}
---

## Description

D50, third bullet. After an editor's work is pushed, the person picks more findings and starts another editor on the same review.

- The run keeps a list of waves (EditorState per wave), the newest last; only the newest can be pushed or discarded, and unpushed work blocks a new wave.
- A wave starts from the last pushed head, on the same branch: the own branch (D41/D42 on a merge request), the stacked change's branch (D39), or the local vuoro/fix-<run> branch (D42), which is updated, not opened again.
- StartEditor accepts a change head equal to the reviewed head or to the newest wave's pushed SHA, and refuses one anyone else moved, with "review again".
- Findings fixed in an earlier wave are not offered for picking.

## Acceptance criteria

- [x] After a push, another editor starts on the same review with findings not yet fixed
- [x] A new wave starts from the last pushed head and updates the same branch or stacked change
- [x] A change someone else moved refuses a new wave and asks for a review again
- [x] The run shows each wave with its findings, commits, and push

## Summary

Once an editor's work is pushed, another can start on the same review (D50). StartEditor archives the pushed editor into the run's waves and closes its session. The new editor seeds from the last push (`lastPush`), records it as `continues`, and commits on the same branch. A push then updates the same stacked change, or moves the same local fix branch on, with the lease taken from the prior push.

A change whose head is anything other than the reviewed head or vuoro's own last push refuses with "review it again". Unpushed commits on the editor block a new wave with ErrEditorBusy.

Findings any wave fixed (FixedIn) are not offered again. FixCommits reads every wave's recorded commits. The app's stacked preview compares the target with the reviewed head, and takes Opened and NotDraft from `continues`.

The editor panel folds the earlier waves (branch, push, commits beside their findings). Once the newest wave is all pushed, it offers "Fix … in another wave (N)".

Tests: TestAReviewIsFixedInWavesFromTheLastPush, TestALocalRangesSecondWaveMovesItsOwnBranchOn, the updated stacked editor test, and two editor.test cases. I did not take a screenshot of the panel.
