---
schema: 3
id: TKT-01M49F69S5V5859RTJFWEQ6YNE
title: "Build the Local tab: projects, branch rows, and Review"
type: task
status: done
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
  - TKT-01M49KHWH65X2S40G441JXTE3X
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-06T20:41:27Z
updated_at: 2026-10-06T22:28:12Z
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

- [x] Branch rows show ahead count, age, worktree tags with a dirty mark, and the last review
- [x] Review starts a run and opens it
- [x] Ten branches show at a time per project
- [x] A project's base can be changed

## Notes

**agent:claude/frontend-modules-2026-10-06** at 2026-10-06T21:32:53Z

Adding, hiding and removing projects and roots moved to TKT-01M49F69Y (Add, hide, and remove local projects and roots, with the folder chooser) on 2026-10-06, so this ticket now waits on it. Changing a project's base stays here.

## Summary

The Local tab has its full rows (web/src/localproject.tsx).

- **Rows:** one per branch, with what needs the person first, ten at a time with "Show N more", and quiet branches folded behind a toggle. Each row has:
  - its worktree tags, marked "uncommitted";
  - commits ahead and age;
  - the open change with its standing;
  - the last review, which opens its run, with its status;
  - its fix branches, and "Needs you: …".
- **Reviews:**
  - "Review local commits" posts to /api/local/review with the preset picked in the tab's header, then opens the run.
  - A row tied to an open change also offers "Review the change", side by side (D49).
  - With no usable agent tool, Review is disabled, and its title says why.
- **Base:** "Change base" sets the project's base, or returns it to the default.

local.tsx keeps the tab, the Add buttons, and Folders. Tests are in web/src/local.test.tsx. No real review was started from the window in testing, because that runs a paid agent.
