---
schema: 3
id: TKT-01M3W4HWPT9NYP4ADW339FJDSC
title: Derive the turn for each change, with its reasons
type: task
status: in-progress
status_reason: null
priority: normal
due_on: null
labels:
  - inbox
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies:
  - TKT-01M3W4HWJZ1013S2MPZ69B72AW
blocks_on: none
references: []
claim:
  actor: agent:codex/inbox-20261002
  branch: tickets/turn-engine
  worktree: /home/sothr/workspace/git.local.sothr.com/terva-sh/vuoro
  commit: 9aa847dc09e57d2f6b237daf218aa4f8e24dcf2f
  session: null
  claimed_at: 2026-10-03T02:55:15Z
  expires_at: null
archive: null
created_at: 2026-10-01T16:25:25Z
updated_at: 2026-10-03T02:56:37Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:codex/inbox-20261002
  name: ""
extensions: {}
---

## Description

The `turn` package: pure functions from the decided model to `mine` or
`theirs`, a `since` time, and the reasons. No I/O. The reasons are the product
(docs/plan.md section 2.1), so each is a value a person can read, such as
"Mason replied on 2 threads", not a code.

## Acceptance criteria

- [ ] Every turn rule in docs/plan.md section 4 has a table test, built from recorded fixtures
- [ ] Every result carries at least one human-readable reason, and a test proves it
- [ ] The package imports no forge adapter, server, or network code

## Implementation plan

Implement turn.Derive as a pure function over classified forge.Change, username and discovery roles. Return mine/theirs, readable structured reasons, earliest unanswered since, and last activity. Cover R1/R2/R2b/R3 and A1/A2/A3; use current reviewer state plus dated review/request/push facts, ignore bot/system activity, and keep unknown timestamps explicit rather than inventing ages. Add nullable authenticated-person merge permission to the neutral model; A3 requires true permission and no effective change request. Generate scrubbed normalized turn fixtures from existing recorded GitLab adapter fixtures, then test rule variants, noise, earliest unanswered ages, missing facts, dual roles and inactive changes without adapter imports in turn. Extend boundary tests to include turn and forbid forge adapters/network/server transitively. Update the design for explicit unknown values and merge permission.
