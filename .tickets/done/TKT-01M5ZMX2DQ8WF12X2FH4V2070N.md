---
schema: 3
id: TKT-01M5ZMX2DQ8WF12X2FH4V2070N
title: "Update by rebase: replay commits, with a stop at each conflict"
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - review
  - agent
  - sandbox
assignees: []
milestone: v1.0.0
parent: TKT-01M5ZMW481E943MKBJ7FSJWNCJ
origin: null
dependencies:
  - TKT-01M5ZMX257Z4EE7BCET05PAR69
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-27T21:40:15Z
updated_at: 2026-10-27T21:48:28Z
created_by:
  id: agent:claude-code/branch-update
  name: ""
updated_by:
  id: agent:claude-code/branch-update
  name: ""
extensions: {}
---

## Description

Rebase as a second strategy for the update run (plan section 6.10, D66). vuoro replays the change's own commits, `target..head`, onto the target one at a time, with each commit's author and message kept and the person as committer. A conflict is a stop like the merge's, seeded on the commits replayed so far, with a new repository and editor session for each stop. Accepting a stop writes that commit from the editor's tree with the original author and message, then replays the rest. A change whose history holds a merge commit is refused for rebase, with the reason.

## Acceptance criteria

- [x] A clean rebase replays every commit with its author and message kept
- [x] Each conflicting commit is a stop resolved by its own editor session, accepted before the next commit is replayed
- [x] A history with a merge commit is refused for rebase, with the reason

## Implementation plan

planRebase lists target..head oldest first, and refuses a history with a merge commit. pickStep runs cherry-pick --no-commit in the run's worktree, keeping the original author, author date and message, with the person as committer. A replay that changes nothing is dropped. carryUpdate replays until the next conflict, which becomes a stop with its own seed, repository and session. AcceptStop writes that commit from the editor's tree with its original authorship, then carries on.

## Summary

Rebase replays the change's own commits onto the target in the run's worktree. Each commit keeps its author, author date and message, and the person is the committer. A commit the target already holds is dropped. Each conflicting commit is a stop with its own seed, editor repository and session. The person accepts it, vuoro writes the replayed commit from the editor's tree, and the replay carries on. A history with a merge commit is refused, with the reason. Tests cover a clean rebase, a rebase that stops twice (the second stop's context changes with the first resolution), and the refusal.
