---
schema: 3
id: TKT-01M49F69S5V5859RTJFWEQ6YNE
title: "Build the Local tab: projects, branch rows, and Review"
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - ui
assignees: []
milestone: null
parent: TKT-01M49F68YV694W81S3ESJKVH02
origin: null
dependencies:
  - TKT-01M49F697KRB85MATTJDR3PE42
  - TKT-01M49F69DQA78FC51MYRH9N1TR
  - TKT-01M49F69K6E6NECKP8KCSG977C
  - TKT-01M49F69YPPFF84P65TX6S9Y80
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-06T20:41:27Z
updated_at: 2026-10-06T21:32:53Z
created_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
updated_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
extensions: {}
---

## Description

Section 6.8: each project with its branch rows, worktree tags, last review and what moved since, fix branches under their branch, Review with a preset, the project's base, Refresh, and adding a project or a root by typed path. Uses the shared components (DiffBrowser not needed; EditForm, useAction, usePoll where it polls).

## Acceptance criteria

- [ ] Branch rows show ahead count, age, worktree tags with a dirty mark, and the last review
- [ ] Review starts a run and opens it
- [ ] Ten branches show at a time per project
- [ ] A project's base can be changed

## Notes

**agent:claude/frontend-modules-2026-10-06** at 2026-10-06T21:32:53Z

Adding, hiding and removing projects and roots moved to TKT-01M49F69Y (Add, hide, and remove local projects and roots, with the folder chooser) on 2026-10-06, so this ticket now waits on it. Changing a project's base stays here.
