---
schema: 3
id: TKT-01M5ZDR7B83Y7SJCRVDWDQWWC8
title: Derive where a project's change sits, since when, and why
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - forge
  - state
assignees: []
milestone: v1.0.0
parent: TKT-01M5ZDQ3ZB0K6KMW2CB8NM183S
origin: null
dependencies:
  - TKT-01M5ZDR759JGEN663FWTQ707ZG
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-27T19:35:17Z
updated_at: 2026-10-27T20:10:23Z
created_by:
  id: agent:claude-code/project-triage
  name: ""
updated_by:
  id: agent:claude-code/project-triage
  name: ""
extensions: {}
---

## Description

Pure functions in `projects`, from a change's facts to where it sits, since when, and the reasons, as `turn.Derive` does for turn (plan section 6.9). The places are draft, blocked, waiting on the author, waiting on review, no reviewers, and approved; a change sits in the first that holds in that order, and the others are reasons. The time is the event that put it there, and an unknown time stays unknown rather than borrowing a creation or update stamp (section 4.1). Bots and system notes count for nothing.

Also derive the row's summary facts: reviewers and their states, unresolved threads and whose turn each is, and the change's place in a stack. A stack is a chain where a change's target branch is another open change's source branch, plus GitLab's merge request dependencies.

## Acceptance criteria

- [x] Each place, and each precedence between two that hold together, has a fixture test
- [x] An unknown time stays unknown; no creation or update stamp stands in for it
- [x] Bot reviewers and system notes change no place and no time
- [x] Stacks are found from target branches and from GitLab dependencies, and a cycle is refused rather than looped

## Implementation plan

1. `projects/sits.go`, pure functions over a classified `forge.Change`: `Sit` returns where the change sits, since when, its reasons, and who holds it. Waiting on the author and waiting on review reuse `turn.Derive`, run as the author (A1, A2) and as each human reviewer (R1, R2, R2b, R3), so the project view and the inbox agree on every age. Approved reads current reviewer states and the dated verdicts. Draft, no reviewers, and blocked by a pipeline or conflict have no dated event in the model, so their time is unknown and the row shows when the change was opened beside it instead.
2. Precedence: draft, blocked, waiting on the author, waiting on review, no reviewers, approved. A change that matches none (reviewers commented, nothing outstanding) sits in waiting on review, with the reason saying so.
3. `projects/stacks.go`: a change sits on another open change when its target branch is that change's source branch in the same project (forks never match, so `forge.Change` gains `SourceProject`), or when the forge's dependencies name it. Chains are walked with a visited set and a cycle drops its links rather than looping. A change blocked only by the change beneath it says so.
4. `projects/display.go`: `Display` groups rows by place, oldest known first, then unknown by when the change was opened, then by number. A row carries reviewers and states, assignees, labels, pipeline, conflict, thread counts (unresolved, and whose turn each is), and its stack.
5. `GET /api/projects/{key}` and refresh answer the display. Fixture tests for each place, each precedence pair, unknown times, bots, stacks from both sources, and a cycle.

## Summary

projects.Sit derives where a change sits from classified facts, reusing turn.Derive as the author (A1, A2) and as each human reviewer (R1, R2, R2b, R3), so the inbox and the project view agree on ages. Draft, no reviewers, a failed pipeline and a conflict have no dated event in the model, so their age is unknown and rows sort those by when the change was opened. Stacks come from target branches within the project (forks never match; forge.Change gained SourceProject from GraphQL sourceProject) and from forge dependencies; cycles are broken by a depth-first search. GET /api/projects/{key} and refresh answer projects.Display.
