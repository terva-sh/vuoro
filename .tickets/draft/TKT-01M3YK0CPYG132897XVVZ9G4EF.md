---
schema: 3
id: TKT-01M3YK0CPYG132897XVVZ9G4EF
title: Run several reviewers on one change, and reconcile their findings
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - agent
  - review
assignees: []
milestone: null
parent: null
origin: null
dependencies:
  - TKT-01M3YK0CM0SWMYXMM7A7M7J2QX
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-02T15:16:29Z
updated_at: 2026-10-02T15:16:29Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
extensions: {}
---

## Description

After v0.1.0. Run more than one reviewer on a change: different profiles,
models, or tools. Reconcile findings that describe the same issue, and say
what was set aside and why. The review action's panels and terva's raati are
prior art for both the shape and the honest outcomes, such as a minority
finding kept and labelled.

## Acceptance criteria

- [ ] Two reviewers on different drivers review one change, and their findings merge with sources shown
- [ ] A finding one reviewer raised and another did not is kept and labelled
