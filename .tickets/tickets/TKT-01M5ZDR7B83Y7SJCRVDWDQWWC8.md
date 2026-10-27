---
schema: 3
id: TKT-01M5ZDR7B83Y7SJCRVDWDQWWC8
title: Derive where a project's change sits, since when, and why
type: task
status: ready
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

Pure functions in `projects`, from a change's facts to where it sits, since when, and the reasons, as `turn.Derive` does for turn (plan section 6.9). The places are draft, blocked, waiting on the author, waiting on review, no reviewers, and approved; a change sits in the first that holds in that order, and the others are reasons. The time is the event that put it there, and an unknown time stays unknown rather than borrowing a creation or update stamp (section 4.1). Bots and system notes count for nothing.

Also derive the row's summary facts: reviewers and their states, unresolved threads and whose turn each is, and the change's place in a stack. A stack is a chain where a change's target branch is another open change's source branch, plus GitLab's merge request dependencies.

## Acceptance criteria

- [ ] Each place, and each precedence between two that hold together, has a fixture test
- [ ] An unknown time stays unknown; no creation or update stamp stands in for it
- [ ] Bot reviewers and system notes change no place and no time
- [ ] Stacks are found from target branches and from GitLab dependencies, and a cycle is refused rather than looped
