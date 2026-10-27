---
schema: 3
id: TKT-01M5ZMX2YC1QW1GTBY25G0EDFG
title: Update from the target in the window, with stops and the push
type: task
status: ready
status_reason: null
priority: normal
due_on: null
labels:
  - ui
  - review
assignees: []
milestone: v1.0.0
parent: TKT-01M5ZMW481E943MKBJ7FSJWNCJ
origin: null
dependencies:
  - TKT-01M5ZMX2P51GCH3KM5VFP755CX
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-27T21:40:16Z
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

"Update from the target" in the window (plan section 6.10). The change page and a row of the project view offer Merge (default) or Rebase. The update's page shows the run's status. For each stop, it shows the conflicted files with markers beside the editor's resolution, the editor's account, and the files changed outside the conflict. It offers the stop's session to talk to, Accept stop, and Discard. When the update is ready, it lists what will be pushed, and the push needs one confirmation that names the branch, the strategy, and for a rebase that it rewrites the branch with a lease.

## Acceptance criteria

- [ ] The update can be started from the change page and from the project view
- [ ] Each stop shows the conflicted files beside the resolution, and the person accepts or discards it
- [ ] The push takes one confirmation that names the branch and the strategy
