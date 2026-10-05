---
schema: 3
id: TKT-01M46M51JAETT32BTKAB7BEXKK
title: Inbox refresh reads every change serially and can time out
type: bug
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - inbox
  - gitlab
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-05T18:10:25Z
updated_at: 2026-10-05T18:10:25Z
created_by:
  id: agent:claude-code/inbox-refresh-20261005
  name: ""
updated_by:
  id: agent:claude-code/inbox-refresh-20261005
  name: ""
extensions: {}
---

## Description

On 2026-10-05 a GitLab account with 51 open changes took about 50 seconds to refresh: readAccount reads facts one change at a time (about 1s each), and refreshAccount times out at 2 minutes. An account with about 120 changes would always time out. Read changes with bounded concurrency, or reuse unchanged facts by updatedAt as the Open path already can.

## Acceptance criteria

- [ ] An account with a few hundred open changes refreshes well inside the timeout
