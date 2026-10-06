---
schema: 3
id: TKT-01M49KHWH65X2S40G441JXTE3X
title: Tie a local branch to its open change from the inbox cache
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - state
  - inbox
  - ui
assignees: []
milestone: null
parent: TKT-01M49F68YV694W81S3ESJKVH02
origin: null
dependencies:
  - TKT-01M49F697KRB85MATTJDR3PE42
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-06T21:57:41Z
updated_at: 2026-10-06T21:57:41Z
created_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
updated_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
extensions: {}
---

## Description

Section 6.8, D49. `local` reads each remote's URL as a host and project path, and each branch's upstream. The app ties a branch to an open change in the inbox's cache when the change's host and project are one of the checkout's remotes and its source branch is the branch's upstream there (or the branch's name, with no upstream). No forge call.

The view gives a tied branch its change (ref, kind, title, web URL, head) and how the branch stands against the change's head: in step, N not pushed, behind, or diverged. A tied branch counts only for commits not pushed ("N commits not pushed to !2"), undecided findings, and a waiting fix branch; never reviewed and moved since do not count for it. The minimal Local panel links the change to its page; the full row's two Review buttons are TKT-01M49F69S.

The owner's example: crdb-access's extends-and-viewers, tracking origin/extends-and-viewers, is gitlab.com/flywheel-io/self/drewshort/crdb-access!2 in the cache.

## Acceptance criteria

- [ ] Remotes in SSH and HTTPS form are read as a host and project path, and each branch's upstream as remote and branch
- [ ] A branch is tied to the cached open change from its upstream on a matching remote, with no forge call
- [ ] A tied row names its change, opens its page, and says in step, N not pushed, behind, or diverged
- [ ] A tied branch counts only for commits not pushed, undecided findings, and a waiting fix branch
