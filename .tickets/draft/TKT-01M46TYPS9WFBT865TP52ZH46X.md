---
schema: 3
id: TKT-01M46TYPS9WFBT865TP52ZH46X
title: Push the editor's fixes and open a draft MR onto the reviewed MR
type: task
status: draft
status_reason: null
priority: high
due_on: null
labels:
  - agent
  - review
  - gitlab
assignees: []
milestone: v1.0.0
parent: null
origin: null
dependencies:
  - TKT-01M46TYPQVEZGMJBTC37KPR3P9
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-05T20:09:18Z
updated_at: 2026-10-05T20:09:18Z
created_by:
  id: agent:claude-code/editor-20261005
  name: ""
updated_by:
  id: agent:claude-code/editor-20261005
  name: ""
extensions: {}
---

## Description

D39 and docs/plan.md section 6.3 cover this. Once the person has reviewed the editor's commits, one confirmed action pushes the fix branch to the change's project with the person's credential (never in argv) and opens a Draft merge request onto the original merge request's source branch. The title and description can be edited first. The description lists each finding addressed, with its commit, and links the original.

## Acceptance criteria

- [ ] Push and open asks first, with the branch, the target, and an editable title and description
- [ ] It pushes with the person's credential kept out of argv, logs, and errors, and opens a Draft MR onto the original source branch
- [ ] A change whose source branch is in a fork is refused with the reason
- [ ] No code path pushes or opens a merge request without the person's confirmation, and a test holds it
