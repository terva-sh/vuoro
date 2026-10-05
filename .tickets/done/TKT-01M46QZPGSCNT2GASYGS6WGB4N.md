---
schema: 3
id: TKT-01M46QZPGSCNT2GASYGS6WGB4N
title: Keep on a large change is slow and shows no progress
type: bug
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - review
  - ui
  - gitlab
assignees: []
milestone: null
parent: null
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-05T19:17:25Z
updated_at: 2026-10-05T19:24:38Z
created_by:
  id: agent:claude-code/keep-collapse-20261005
  name: ""
updated_by:
  id: agent:claude-code/keep-collapse-20261005
  name: ""
extensions: {}
---

## Description

Keep on terraform!2487, which has 263 files, takes a long time and shows nothing while it runs. Opening that change takes about 5s, almost all of it reading patches. Keep reads the whole change to place the comment, writes, reads the whole change again for its reply, and then refreshes the inbox entry. That refresh does a third read and can queue behind a full inbox refresh.

## Acceptance criteria

- [x] Keep reuses patches already read for the same diff version instead of downloading them again
- [x] The inbox refresh after a write does not hold up the reply
- [x] The window shows that a Keep is in progress

## Notes

**agent:claude-code/keep-collapse-20261005** at 2026-10-05T19:24:37Z

Measured on terraform!2487 (263 files), read-only: a full open took 4.4-5.2s, and a second open at the same diff version took about 2s with the patch cache. The remaining 2s is the merge request query, discussions and diff versions, plus the draft notes. Keep still opens the change twice (once to place the comment, once for the reply), so expect about 4-5s instead of 15s or more. The Forgejo adapter has no patch cache yet. The reply's re-open could also be dropped in favour of the window refetching, if this is still too slow.

## Summary

The GitLab client caches patches by diff version (8 changes). The inbox refresh after a write runs in the background. Keep shows Keeping…. On !2487 a cached open is about 2s instead of about 5s.
