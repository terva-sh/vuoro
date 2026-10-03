---
schema: 3
id: TKT-01M3YMSSVNA1TJVQ425EBHSWZ7
title: Pin review runs to their head commit, and flag them when it moves
type: task
status: in-progress
status_reason: null
priority: normal
due_on: null
labels:
  - review
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies:
  - TKT-01M3YK0CM0SWMYXMM7A7M7J2QX
blocks_on: none
references: []
claim:
  actor: agent:codex/head-summary-20261003
  branch: tickets/moving-heads
  worktree: /home/sothr/workspace/git.local.sothr.com/terva-sh/vuoro
  commit: 1879c644752aa3754756f52400f1723cc2db7f02
  session: null
  claimed_at: 2026-10-03T06:45:19Z
  expires_at: null
archive: null
created_at: 2026-10-02T15:47:50Z
updated_at: 2026-10-03T06:54:54Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:codex/head-summary-20261003
  name: ""
extensions: {}
---

## Description

A review run reviews one head SHA. When the change gains commits, vuoro flags
the run and each of its drafts as "reviewed at abc123, 2 commits behind". It
never re-runs on its own. Re-anchoring drafts and reviewing only the new
commits come at v1.0.0.

## Acceptance criteria

- [x] A run records the head SHA it reviewed
- [x] After a push, the run and its drafts show how many commits behind they are
- [x] Nothing re-runs automatically

## Implementation plan

Finish D15's moving-head behavior before the selected summary ticket. Keep immutable reviewed head/base and existing personal drafts. Add a protected explicit head-comparison endpoint that fresh-reads the exact forge change, fetches its refs with the existing git transport, verifies full object IDs and ancestry, and returns derived same/ahead/rewritten/unavailable status bound to both commits. Count reachable commits only when reviewed head is an ancestor; missing objects, canceled/failed fetches and rewritten history never invent a count. Share the badge across the selected review, all its findings/source/withdrawn cards, diff-side drafts and author triage. Isolate old anchored drafts from today's diff and disable stale anchored acceptance; the server independently checks fresh identity/head. Checking status never launches an agent, modifies a draft or automatically reruns. Cover linear/merge histories, rewrites, unavailable/cancelled data, protection/identity, persistence and frontend response races/placement. Update the plan/roadmap, rebuild embedded assets, run full CGO_ENABLED=0 CI and native gates, then review/fix/merge and sync before implementing summaries. No live posting validation or re-anchoring/incremental review in this slice.

## Notes

**agent:codex/head-summary-20261003** at 2026-10-03T06:53:34Z

Implemented protected exact-pair head comparison with cancellable repository fetch, verified ancestry and reachable commit counts. Rewrites/unavailable history have no invented count. Old anchored drafts keep their wording/locations and appear separately from today's diff; fresh server identity/head checks refuse acceptance. No review reruns or draft mutation from status reads. Full just ci passed with CGO_ENABLED=0: Go tests/vet/cross-builds, 44 frontend tests, bundle/design matching and strict tickets. Browser synthetic fixture confirmed the run/finding badges and separate old-draft section; no live forge writes or model involved. Native and model review gates follow on the PR.

**agent:codex/head-summary-20261003** at 2026-10-03T06:54:54Z

PR #13 review found a valid frontend race: the fresh comparison may return a current head beyond the displayed workspace snapshot. Fixed completed mismatched responses to show an explicit refresh-to-compare state instead of indefinite checking, without using the count against a different diff. Added a regression test and posted the disposition while the PR is open. Full gate and fresh model review follow.
