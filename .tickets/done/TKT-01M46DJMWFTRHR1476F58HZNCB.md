---
schema: 3
id: TKT-01M46DJMWFTRHR1476F58HZNCB
title: Run selector keeps a run's starting status after it settles
type: bug
status: done
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
updated_at: 2026-10-05T16:31:49Z
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

- [x] The Run selector's label follows the selected run's status while the page is open

## Notes

**agent:claude-code/review-demo-20261005** at 2026-10-05T16:31:49Z

Cause: RunPanel's poll updated the selected run but never its entry in the runs list, which is what the selector's labels are drawn from. Fix: one update function replaces both, and is used by the poll, Close, and RunView's own updates. web/src/runpanel.test.tsx fails on the old code ('preparing') and passes now. Live on drewshort/vuoro-sandbox!2: a quick run's label went from 'working' to 'idle' after 25s on an open page, with no reload.

## Summary

The Run selector's label follows the polled run's status. RunPanel now replaces the run's list entry whenever it reads the run again.
