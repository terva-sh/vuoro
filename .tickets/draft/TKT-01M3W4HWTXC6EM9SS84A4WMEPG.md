---
schema: 3
id: TKT-01M3W4HWTXC6EM9SS84A4WMEPG
title: Comment, reply, resolve, approve, and submit on GitLab
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - review
  - gitlab
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies:
  - TKT-01M3W4HWSGA04D4VQ8WHBEJC2D
  - TKT-01M3W4HWDY8XV92MXXQWA0AES7
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-01T16:25:25Z
updated_at: 2026-10-01T16:25:25Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
extensions: {}
---

## Description

The write half of the GitLab adapter, and the controls in the change view.
Inline and general comments go out as GitLab draft notes, and "submit" publishes
them together with `draft_notes/bulk_publish`, so that a review lands as one
event as reviewers expect. Replies, resolve and unresolve, approve, and revoke
approval.

Every write is the person's explicit action. A draft drafted by an agent is
visibly marked until the person edits or accepts it.

## Acceptance criteria

- [ ] An inline comment, a reply, and a resolve each round-trip against a real GitLab change
- [ ] Submit publishes every pending draft note as one batch
- [ ] Approve and revoke work, and the inbox turn updates after each
- [ ] No write is reachable without the loopback protection from the loopback-auth ticket
