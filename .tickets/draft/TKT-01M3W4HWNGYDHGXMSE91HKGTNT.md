---
schema: 3
id: TKT-01M3W4HWNGYDHGXMSE91HKGTNT
title: "GitLab adapter: discover the changes I review and wrote"
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
  - TKT-01M3YMSSPF763ZMG6P9FAWQYXK
  - TKT-01M3ZQXC5CPRDM3CD937C6PC3Q
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-01T16:25:25Z
updated_at: 2026-10-03T02:01:28Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:codex/windows-roadmap-20261002
  name: ""
extensions: {}
---

## Description

Implement the `forge` interface for gitlab.com, read-only: discover changes
where the person is a requested reviewer, an author, or an assignee, and read
each one's diff refs, approvals, discussions, and the person's draft notes.

docs/plan.md section 5 lists the endpoints. Respect rate limits, and fetch
incrementally with `updated_after` once the first full pull is cached.
The adapter must work against any GitLab host, not only gitlab.com.

## Acceptance criteria

- [ ] Adapter tests run from recorded fixtures with no network
- [ ] A live run lists the same open changes the GitLab review-requested page shows
- [ ] No token appears in any log line or error message, and a test proves it
