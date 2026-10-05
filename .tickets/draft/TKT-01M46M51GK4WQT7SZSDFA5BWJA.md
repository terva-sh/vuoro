---
schema: 3
id: TKT-01M46M51GK4WQT7SZSDFA5BWJA
title: One unreadable change blanks an account's whole inbox refresh
type: bug
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - inbox
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

inbox.readAccount returns the first ReadFacts error, so one change that cannot be read (a connection limit, a GraphQL error, a permission change) fails the whole account, and the inbox shows only the previous snapshot with a generic message. Seen on 2026-10-05 with agent-api!164 among 51 changes. The cause is fixed separately, but other limits remain (discussion notes over 100, reviewers, assignees or participants over 100). Consider keeping the readable changes, showing the unreadable one by its ref with its own error, and naming the failing change in the account status. Removal safety needs complete membership, which discovery still provides.

## Acceptance criteria

- [ ] A change whose facts cannot be read does not hide the account's other changes
- [ ] The person can see which change failed, and why, without a token or forge body in the message
