---
schema: 3
id: TKT-01M4BVRFAY1DN7317TACJXXXRA
title: Fix a review in waves, and review again with its earlier findings
type: epic
status: in-progress
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
claim:
  actor: agent:claude/frontend-modules-2026-10-06
  branch: TKT-01M4BVRFD-fix-links
  worktree: /Users/drewshort/Workspace/github.com/terva-sh/vuoro
  commit: 0b74f4c37e89433d171fac86d1ee3589a3eb9f74
  session: null
  claimed_at: 2026-10-07T19:12:14Z
  expires_at: null
archive: null
created_at: 2026-10-07T18:59:34Z
updated_at: 2026-10-07T19:12:14Z
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
