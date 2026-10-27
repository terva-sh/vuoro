---
schema: 3
id: TKT-01M5ZMW481E943MKBJ7FSJWNCJ
title: Update a change from its target, with the editor resolving conflicts
type: epic
status: ready
status_reason: null
priority: normal
due_on: null
labels:
  - review
  - agent
assignees: []
milestone: v1.0.0
parent: null
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-27T21:39:45Z
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

Bring a change's target branch into it, by merge (the default) or by rebase, from the change page and the project view. vuoro computes the update itself. When nothing conflicts, no agent runs. Each conflict is a stop: the editor resolves it in its sandbox and commits, vuoro builds the merge or rebased commit, and the person accepts each stop. Then the person confirms the push: a fast-forward for a merge, a force with lease for a rebase, or a draft change onto somebody else's branch for a merge there.

Design: docs/plan.md section 6.10. Decisions: D66 (update by merge or rebase; the editor resolves only conflicts) and D67 (force with lease only for an accepted rebase).

## Acceptance criteria

- [ ] A change is updated from its target by merge or rebase, and a clean update needs no agent
- [ ] Each conflict is resolved by the editor in its sandbox, and the person accepts it before anything is pushed
- [ ] A rebase force-pushes only with a lease against the head the person saw
- [ ] The update is offered on GitLab, GitHub and Forgejo, from the change page and the project view
