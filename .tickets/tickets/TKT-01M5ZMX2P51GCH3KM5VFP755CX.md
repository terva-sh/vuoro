---
schema: 3
id: TKT-01M5ZMX2P51GCH3KM5VFP755CX
title: "Push an accepted update: fast-forward, lease, or a draft change"
type: task
status: ready
status_reason: null
priority: normal
due_on: null
labels:
  - review
  - forge
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

The push route for a ready update (plan section 6.10, D66, D67), called only from the person's confirmed action. On the person's own change, a merge is pushed to the change's branch as a fast-forward, and a rebase is pushed with `--force-with-lease` against the head the update started from. Either is refused when the forge's head has moved. On somebody else's change, a merge goes to `vuoro/update-<number>-<run>` and is opened as a draft change onto their branch through `forge.Stacker`, and a rebase is refused at the start. A fork's branch is refused. App routes start, read, accept a stop, discard, and push the update, all behind the window guard.

## Acceptance criteria

- [ ] Only the confirmed push route pushes an update, and a test holds it
- [ ] A rebase pushes with a lease against the started head, and a moved branch is refused, not overwritten
- [ ] On somebody else's change a merge opens a draft change onto their branch, and a rebase is refused
