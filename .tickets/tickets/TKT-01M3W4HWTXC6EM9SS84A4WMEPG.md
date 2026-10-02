---
schema: 3
id: TKT-01M3W4HWTXC6EM9SS84A4WMEPG
title: Comment, reply, resolve, approve, and submit on GitLab
type: task
status: blocked
status_reason: The live round-trip needs the owner's GitLab token in the age store, the config file, and a merge request the owner can write to
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
updated_at: 2026-10-02T20:04:15Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-mvp-1
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
- [x] Submit publishes every pending draft note as one batch
- [ ] Approve and revoke work, and the inbox turn updates after each
- [x] No write is reachable without the loopback protection from the loopback-auth ticket

## Implementation plan

forge.Writer, with GitLab's implementation in forge/gitlab/write.go: draft notes (create, update, delete), bulk_publish, resolve, approve, and unapprove. app/write.go serves them under /api/changes/*, each answering with the change re-read, plus keep and drop for an agent's draft. Inline positions come from patch.Line, which names an unchanged line on both sides as GitLab requires.

## Notes

**agent:claude/vuoro-mvp-1** at 2026-10-02T20:04:06Z

Not yet run against a real GitLab change. It needs the owner's token, and a
merge request the owner can write to. The recorder test checks every request's
method, path, and position body. The end-to-end app test runs an agent,
then keeps, drops, and submits against a fake GitLab.

Criterion 3's inbox half waits for the inbox. Approve and revoke are wired,
but nothing derives the turn yet (TKT-01M3W4HWP).

Writes need a configured forge: an anonymous client refuses every write
before any request leaves.

**agent:claude/vuoro-mvp-1** at 2026-10-02T20:04:15Z

in-progress to blocked: The live round-trip needs the owner's GitLab token in the age store, the config file, and a merge request the owner can write to
