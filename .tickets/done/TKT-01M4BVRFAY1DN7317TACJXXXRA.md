---
schema: 3
id: TKT-01M4BVRFAY1DN7317TACJXXXRA
title: Fix a review in waves, and review again with its earlier findings
type: epic
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - review
  - agent
  - ui
assignees: []
milestone: null
parent: null
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-07T18:59:34Z
updated_at: 2026-10-07T20:24:28Z
created_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
updated_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
extensions: {}
---

## Description

D50 and docs/plan.md section 6.3, "Fixing in waves, and reviewing again". A review keeps its findings and decisions across several fixes and several reviews: each fix records its commit, fixed findings fold away, a review takes several editor waves while only vuoro moved the change, and reviewing again carries the earlier findings and their decisions.

Found on the owner's own merge request on 2026-10-07 (gitlab.com/flywheel-io/infrastructure/terraform!2893): after the editor pushed, no second editor could start; two fixes whose commits lacked the Vuoro-Finding trailer stayed undecided; and every finding showed "reviewed at eb370b4b, 2 commits behind", both commits vuoro's own.

## Notes

**agent:claude/frontend-modules-2026-10-06** at 2026-10-07T20:24:28Z

The one gap left open, a local review following the branch's last run, is closed; see the latest note on TKT-01M4BVRFX.

## Summary

All four children are done:
- TKT-01M4BVRFD (fix links: trailer, then ask, then by hand)
- TKT-01M4BVRFJ (Fixed group, linked commits, vuoro's own fixes not read as a move)
- TKT-01M4BVRFR (editor waves from the last push)
- TKT-01M4BVRFX (review again carrying the earlier findings and decisions)

Each failure seen on terraform!2893 now has an answer:
- A second editor starts after a push.
- An unnamed fix is asked about, or linked by hand.
- The reviewed-head warning names vuoro's own fixes.

Left open: a Local-tab review of local commits does not yet follow the branch's last run.
