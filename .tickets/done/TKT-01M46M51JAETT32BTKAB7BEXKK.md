---
schema: 3
id: TKT-01M46M51JAETT32BTKAB7BEXKK
title: Inbox refresh reads every change serially and can time out
type: bug
status: done
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
updated_at: 2026-10-05T18:33:59Z
created_by:
  id: agent:claude-code/inbox-refresh-20261005
  name: ""
updated_by:
  id: agent:claude-code/inbox-tuning-20261005
  name: ""
extensions: {}
---

## Description

On 2026-10-05 a GitLab account with 51 open changes took about 50 seconds to refresh: readAccount reads facts one change at a time (about 1s each), and refreshAccount times out at 2 minutes. An account with about 120 changes would always time out. Read changes with bounded concurrency, or reuse unchanged facts by updatedAt as the Open path already can.

## Acceptance criteria

- [x] An account with a few hundred open changes refreshes well inside the timeout

## Notes

**agent:claude-code/inbox-tuning-20261005** at 2026-10-05T18:33:58Z

Reads of one account run six at a time (readWorkers), keeping a stable order. TestChangesAreReadConcurrently reads 60 changes at 50ms each well within 2s, and checks that peak concurrency stays between 2 and 6. Live on 2026-10-05: the owner's 51 GitLab changes refreshed in about 14s, down from about 51s.

## Summary

Change reads run six at a time; the owner's 51-change refresh went from about 51s to about 14s.
