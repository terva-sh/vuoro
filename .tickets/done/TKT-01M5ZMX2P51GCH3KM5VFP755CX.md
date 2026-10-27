---
schema: 3
id: TKT-01M5ZMX2P51GCH3KM5VFP755CX
title: "Push an accepted update: fast-forward, lease, or a draft change"
type: task
status: done
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
updated_at: 2026-10-27T21:55:09Z
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

- [x] Only the confirmed push route pushes an update, and a test holds it
- [x] A rebase pushes with a lease against the started head, and a moved branch is refused, not overwritten
- [x] On somebody else's change a merge opens a draft change onto their branch, and a rebase is refused

## Implementation plan

Repos.PushUpdate pushes from the clone with --no-verify, and with a lease only for a rebase. Manager.PushUpdate refuses a stale sha, a moved head, a rebase on another's change and a concurrent push. On another's change it pushes vuoro/update-<n>-<run> and opens a draft via a callback. app/update.go adds start, list, detail, accept, discard and push routes behind the guard. Start refuses forks and sets Own from isAuthor. Push requires confirm=push and reads the forge head first. keepProjectRuns also keeps update runs out of the generic routes, and /api/runs leaves them out.

## Summary

Repos.PushUpdate and Manager.PushUpdate in review/update*.go; routes in app/update.go. An own merge fast-forwards the change's branch, a rebase forces with a lease against the started head, and another's merge goes to vuoro/update-<n>-<run> with a draft change via OpenStacked. A fork is refused by StackTarget at start and at push. Update runs stay out of /api/runs and answer only '', close, sessions and update under /api/runs/{id}. The AST test holds that only pushUpdate calls PushUpdate, and that only pushEditor and pushUpdate call OpenStacked.
