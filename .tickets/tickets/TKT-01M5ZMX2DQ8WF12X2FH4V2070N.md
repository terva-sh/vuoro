---
schema: 3
id: TKT-01M5ZMX2DQ8WF12X2FH4V2070N
title: "Update by rebase: replay commits, with a stop at each conflict"
type: task
status: ready
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
updated_at: 2026-10-27T21:40:36Z
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

- [ ] A clean rebase replays every commit with its author and message kept
- [ ] Each conflicting commit is a stop resolved by its own editor session, accepted before the next commit is replayed
- [ ] A history with a merge commit is refused for rebase, with the reason
