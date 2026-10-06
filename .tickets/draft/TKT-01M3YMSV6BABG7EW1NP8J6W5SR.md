---
schema: 3
id: TKT-01M3YMSV6BABG7EW1NP8J6W5SR
title: Review a local branch or range with no forge
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - review
  - agent
assignees: []
milestone: v1.0.0
parent: TKT-01M3YMSTYMFRDFXD4QSVWHMM8P
origin: null
dependencies:
  - TKT-01M3YK0CPYG132897XVVZ9G4EF
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-02T15:47:52Z
updated_at: 2026-10-06T22:28:29Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
extensions: {}
---

## Description

Point vuoro at a checkout and a range, such as `origin/main..HEAD`, before
anything is pushed. The orchestrator, panes, and synthesis are the same as for
a forge change. Findings become a local checklist, and they carry into the
merge request if the person opens one.

## Acceptance criteria

- [x] A local range runs the same preset as a forge change
- [ ] Its findings carry into the merge request opened from that branch

## Notes

**agent:claude/frontend-modules-2026-10-06** at 2026-10-06T22:28:29Z

Criterion 1 is met by TKT-01M49F69D (Start a local review of a branch from the window): a branch's local range runs through the same Plan and Runs.Start as a forge change, with the preset picked, from the window and from vuoro review. Criterion 2, carrying findings into the merge request, is still open. The Local tab now ties a branch to its open change (TKT-01M49KHWH), which is where that carrying could start.
