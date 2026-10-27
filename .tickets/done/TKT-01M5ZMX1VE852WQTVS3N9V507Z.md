---
schema: 3
id: TKT-01M5ZMX1VE852WQTVS3N9V507Z
title: Record branch updates in the plan (D66, D67) and file their tickets
type: chore
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - docs
assignees: []
milestone: v1.0.0
parent: TKT-01M5ZMW481E943MKBJ7FSJWNCJ
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-27T21:40:15Z
updated_at: 2026-10-27T21:40:37Z
created_by:
  id: agent:claude-code/branch-update
  name: ""
updated_by:
  id: agent:claude-code/branch-update
  name: ""
extensions: {}
---

## Description

Section 6.10 of docs/plan.md, D66 and D67 in docs/decisions.md, and the amendment to section 6.3's rule against forcing.

## Acceptance criteria

- [x] docs/plan.md section 6.10 describes the update, its stops, and the push
- [x] D66 and D67 are recorded

## Summary

Recorded D66 (update by merge or rebase; vuoro computes it, and the editor resolves only conflicts) and D67 (force with lease only for an accepted rebase on the person's own change). Added docs/plan.md section 6.10 and amended section 6.3's rule against forcing, sections 2 and 10.2. Filed the epic and five tickets.
