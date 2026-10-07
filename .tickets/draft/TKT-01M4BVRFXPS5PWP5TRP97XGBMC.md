---
schema: 3
id: TKT-01M4BVRFXPS5PWP5TRP97XGBMC
title: Review a change again carrying the earlier review's findings
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - review
  - agent
  - ui
assignees: []
milestone: null
parent: TKT-01M4BVRFAY1DN7317TACJXXXRA
origin: null
dependencies:
  - TKT-01M4BVRFD32Q0HM8X6NTTZSWSD
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-07T18:59:35Z
updated_at: 2026-10-07T18:59:35Z
created_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
updated_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
extensions: {}
---

## Description

D50, fourth bullet. "Review again" on a moved change starts a run that follows the earlier one (a recorded follows id, linked both ways).

- Reviewers' first message lists the earlier findings with where each stands: fixed (with commit), dismissed, or open.
- propose_draft gains an optional earlier id; a draft that restates an earlier finding names it and comes in with that finding's decision, so a dismissed one arrives dismissed with the earlier dismissal shown.
- The run page shows the run it follows and an "Earlier findings" folded group.

## Acceptance criteria

- [ ] Review again starts a run linked to the earlier one, both ways
- [ ] Reviewers are given the earlier findings with where each stands
- [ ] A draft restating an earlier finding keeps the person's decision on it
- [ ] The run page shows the earlier findings in a folded group
