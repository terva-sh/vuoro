---
schema: 3
id: TKT-01M46DJMWFTRHR1476F58HZNCB
title: Run selector keeps a run's starting status after it settles
type: bug
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - ui
  - review
assignees: []
milestone: v0.1.0
parent: null
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-05T16:15:31Z
updated_at: 2026-10-05T16:15:31Z
created_by:
  id: agent:claude-code/review-demo-20261005
  name: ""
updated_by:
  id: agent:claude-code/review-demo-20261005
  name: ""
extensions: {}
---

## Description

During a standard review of drewshort/vuoro-sandbox!2 on 2026-10-05, the Run dropdown on the Review tab still read "2m ago at 013c7e8f, preparing" after every session and the run itself showed idle, with all findings in. A reload showed "idle". The selector's label is computed from the run list fetched when the change opened, and is not refreshed when the polled run changes.

## Acceptance criteria

- [ ] The Run selector's label follows the selected run's status while the page is open
