---
schema: 3
id: TKT-01M3YMSSPF763ZMG6P9FAWQYXK
title: "Open a GitLab change by URL: metadata, diff refs, and threads"
type: task
status: draft
status_reason: null
priority: high
due_on: null
labels:
  - forge
  - gitlab
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies:
  - TKT-01M3W4HWJZ1013S2MPZ69B72AW
  - TKT-01M3W4HWM6V2F20KNT03BSBNZY
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-02T15:47:50Z
updated_at: 2026-10-02T15:47:50Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
extensions: {}
---

## Description

The first slice of the GitLab adapter, and the orchestrator's entry point:
given a merge request URL on any GitLab host, read its metadata, diff refs
(`base_sha`, `start_sha`, `head_sha`), diff, discussions, approvals, and the
person's draft notes. Discovery of every change the person is involved in is
TKT-01M3W4HWNGYDHGXMSE91HKGTNT (GitLab adapter: discover the changes I review
and wrote), which builds on this.

`~/Workspace/code-reviews/tools/fetch_mr.py` handles GitLab's truncated-diff
fallback, and it is worth reading first.

## Acceptance criteria

- [ ] A merge request URL on gitlab.com opens with its diff and threads
- [ ] A truncated diff falls back to reading the files, and a test proves it from fixtures
- [ ] No token appears in any log line or error message, and a test proves it
