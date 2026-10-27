---
schema: 3
id: TKT-01M5ZMX257Z4EE7BCET05PAR69
title: "Update by merge: vuoro merges, and the editor resolves each conflict"
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
  - TKT-01M5ZMX1VE852WQTVS3N9V507Z
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-27T21:40:15Z
updated_at: 2026-10-27T21:48:27Z
created_by:
  id: agent:claude-code/branch-update
  name: ""
updated_by:
  id: agent:claude-code/branch-update
  name: ""
extensions: {}
---

## Description

The review package gains a run in mode `update` (plan section 6.10). It checks out the change's head and target as a review does, then computes a merge of the target into the head in the run's own worktree. A clean merge becomes the merge commit at once, with the person's identity and no agent. A conflicted merge becomes a stop: the conflicted tree, markers included, is committed as a seed on the head, the editor's repository is seeded there, and one editor session resolves it in the sandbox. Accepting the stop fetches the editor's branch, checks that it descends from the seed and leaves no marker in the conflicted files, and writes the merge commit with `commit-tree` using the editor's tree and the head and target as parents. Discarding deletes the repositories and refs. The run and its stops survive a restart.

## Acceptance criteria

- [x] A clean merge produces the merge commit with no agent session
- [x] A conflicted merge seeds the editor at the conflicted tree, and its resolution is checked for leftover markers and descent before vuoro builds the merge commit
- [x] vuoro never runs git in the editor's repository, and a test holds it
- [x] The update run, its stop and its result survive a restart

## Implementation plan

review/update.go: a run in mode update with Update{Strategy, Stops, Result} in its record. prepare checks out head and target, then prepareUpdate computes. review/update_git.go: mergeStep runs git merge --no-commit with diff3 in the run's worktree. A clean merge is written with commit-tree, using head and target as parents. A conflicted one is staged with add -A and written as a one-parent seed. beginStop reuses SeedEditor and the editor's sandbox spec (editorDir/wt), with the update prompt. AcceptStop fetches the editor's branch with FetchEditor, checks descent and leftover markers, and writes the merge commit from the editor's tree. DiscardUpdate closes the run and drops refs/vuoro/update/<run>/.

## Summary

Update by merge is in review/update.go and update_git.go. vuoro merges in the run's own worktree. A clean merge becomes the merge commit (head and target as parents, the person as author) with no session. A conflict becomes a seed commit of the conflicted tree with diff3 markers, which seeds the sandboxed editor's repository. AcceptStop reads the editor's branch back through FetchEditor and refuses a branch that does not descend from the seed, or that leaves marker lines in a conflicted file. vuoro then writes the merge commit with commit-tree. Tests cover the clean merge, the resolved conflict, refused markers, the restart, the discard, and the refusals. A canary shows that vuoro runs no command the editor's repository configures. The rebase strategy is in the same files, under TKT-01M5ZMX2DQ8WF12X2FH4V2070N.
