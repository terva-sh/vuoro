---
schema: 3
id: TKT-01M46M50RP9J0N5QAKE5P63PEN
title: GitLab refresh fails on a merge request pushed over 100 times
type: bug
status: done
status_reason: null
priority: high
due_on: null
labels:
  - gitlab
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
updated_at: 2026-10-05T18:12:35Z
created_by:
  id: agent:claude-code/inbox-refresh-20261005
  name: ""
updated_by:
  id: agent:claude-code/inbox-refresh-20261005
  name: ""
extensions: {}
---

## Description

On 2026-10-05 the owner's GitLab inbox showed "No successful refresh yet" and "Could not refresh this account". Discovery found 51 open changes, but reading the facts of flywheel-io/architecture/agent/agent-api!164 failed with "gitlab: review facts exceed connection limit": it has 142 diff versions, and the query read the first 100 and refused the rest. One unreadable change aborts the whole account's refresh, so the inbox stayed empty.

## Acceptance criteria

- [x] A merge request with more than 100 diff versions is read with every version as a push
- [x] The owner's live GitLab account refreshes

## Notes

**agent:claude-code/inbox-refresh-20261005** at 2026-10-05T18:12:35Z

Fix: forge/gitlab reads diff versions past the first hundred with a second query (diffs.graphql), following endCursor for up to 49 pages, and refuses the read if the merge request's updatedAt or iid changes between pages. Reviewers, assignees and participants keep their 100 cap. diffpages_test.go covers 230 versions over three pages, oldest-first ordering, head on the newest push, and the moved-between-pages refusal. Live on 2026-10-05: all 51 of the owner's discovered changes read, including agent-api!164 with 142 versions, and the window's inbox refreshed in 51s with 21 changes on the owner's turn. This is also the first authenticated nonempty GitLab inbox seen live. Two follow-ups are filed: TKT-01M46M51GK4WQT7SZSDFA5BWJA (one unreadable change blanks the account) and TKT-01M46M51JAETT32BTKAB7BEXKK (serial reads can time out).

## Summary

GitLab merge requests with more than 100 diff versions now read in full, so the owner's inbox refreshes. Verified live with 51 changes.
