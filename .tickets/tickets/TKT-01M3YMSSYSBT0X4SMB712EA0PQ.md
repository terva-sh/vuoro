---
schema: 3
id: TKT-01M3YMSSYSBT0X4SMB712EA0PQ
title: Triage the threads on my change, and draft replies
type: task
status: in-progress
status_reason: null
priority: normal
due_on: null
labels:
  - agent
  - review
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies:
  - TKT-01M3YMSSRC81M0BYHK15TW9VDY
  - TKT-01M3YMSSPF763ZMG6P9FAWQYXK
  - TKT-01M3W4HWSGA04D4VQ8WHBEJC2D
blocks_on: none
references: []
claim:
  actor: agent:codex/gitlab-live-20261003
  branch: tickets/author-triage
  worktree: /home/sothr/workspace/git.local.sothr.com/terva-sh/vuoro
  commit: 74b3e38ca7400c2f316657cca0f9d3fd011387ef
  session: null
  claimed_at: 2026-10-03T06:16:28Z
  expires_at: null
archive: null
created_at: 2026-10-02T15:47:50Z
updated_at: 2026-10-03T06:16:40Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:codex/gitlab-live-20261003
  name: ""
extensions: {}
---

## Description

The author's side of the MVP. On a change the person wrote, an agent reads the
open threads, groups them as must fix, discuss, or nit, and drafts replies
through the drafts MCP server. The person edits and posts. Fixing code is the
editor agent's job, at v1.0.0.

## Acceptance criteria

- [ ] Open threads on my change are grouped as must fix, discuss, or nit, with the reason for each
- [ ] Reply drafts appear on their threads, marked as drafted, and post only on my action

## Implementation plan

Start with a documented interaction sketch and a working Threads tab on changes authored by the configured account. Group actionable human threads into Must fix, Discuss and Nit with visible agent rationale; retain an Untriaged group while work is incomplete. Use a dedicated single reviewer-class triage session through the existing proven driver, worktree and conversation infrastructure, never an embedded agent or forge-writing MCP tool. Extend local drafts with validated thread/category/reason and a server-bound thread snapshot. Persist runs/replies and expose local edit/discard independently of forge write capability; freeze person-edited drafts against later agent revisions. The person accepts a reply into existing pending forge comments and submits through the existing confirmation. Fresh server-side identity/head/thread checks refuse stale, resolved or mismatched replies. Forgejo supports local triage/reply drafting and copy while posting remains unavailable. Show progress, cancellation, absent tools, read-only, errors and changed-snapshot states in the sketch and fixture preview. Cover role separation, injection-resistant context, full thread context, persistence, user-edited drafts, stale/identity checks, protected routes and UI interactions with meaningful tests; rebuild embedded assets and run full CGO_ENABLED=0 CI before PR/review/merge. Do not expand into usage, retention, editor or summaries tickets.
