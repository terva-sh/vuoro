---
schema: 3
id: TKT-01M3ZQXC5CPRDM3CD937C6PC3Q
title: Share forge discovery, caching, and refresh for the inbox
type: task
status: in-progress
status_reason: null
priority: high
due_on: null
labels:
  - forge
  - inbox
  - state
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies:
  - TKT-01M3W4HWJZ1013S2MPZ69B72AW
  - TKT-01M3W4HWM6V2F20KNT03BSBNZY
blocks_on: none
references: []
claim:
  actor: agent:codex/inbox-20261002
  branch: tickets/inbox-discovery
  worktree: /home/sothr/workspace/git.local.sothr.com/terva-sh/vuoro
  commit: bd81a4eea8df78cd7d7a9bbe31a3dfa18dc0170d
  session: null
  claimed_at: 2026-10-03T02:39:51Z
  expires_at: null
archive: null
created_at: 2026-10-03T02:01:27Z
updated_at: 2026-10-03T02:44:01Z
created_by:
  id: agent:codex/windows-roadmap-20261002
  name: ""
updated_by:
  id: agent:codex/inbox-20261002
  name: ""
extensions: {}
---

## Description

D34 selects a single GitLab-and-Forgejo inbox as the next product milestone. Add a narrow, optional discovery contract beside the existing forge Provider/Writer contracts, and an account-scoped service that refreshes derived forge state for the inbox. Agents and ticket stores retain their own boundaries.

Keep pagination and API details in adapters; the service owns bounded/cancellable refresh, deduplicated identities and roles, last-success/error information, and cache updates. A failed or rate-limited account must not erase a healthy account’s results or silently look like an empty inbox. Authored snoozes are separate from derived snapshots. Apply configured bot identity information before turn derivation, preserving the model’s exclusion of bot/system activity. Keep existing Ref/run identities stable.

Build with fixture providers first so both adapter tasks can implement the same behavior. This task does not implement either forge’s discovery endpoints, a forge write, the turn rules, or the frontend.

## Acceptance criteria

- [x] A narrow optional forge discovery contract works with fixture providers without importing agent or tickets.
- [x] Account results deduplicate change identities and roles while preserving existing Ref identities.
- [x] A failed, cancelled or rate-limited refresh preserves prior account snapshots and exposes last-success/error state without leaking credentials.
- [x] Configured bot identities are applied before turn derivation, and authored snoozes survive cache replacement.
- [x] Tests cover mixed-account refresh and partial failures without network I/O or installed agent tools.

## Implementation plan

Add optional forge.Discoverer returning complete open membership with roles and forge update stamps. Keep pagination inside adapters. Build an inbox service independent of agents/tickets, bounded to four concurrent accounts with per-account timeouts and cancellation; each account commits an entire new snapshot only after all reads succeed. Reuse unchanged cached facts by update stamp, preserve failed snapshots, and store scrubbed status categories. Use stable Ref keys; persist derived snapshots under cache and authored snoozes under annotations. Apply configured bots to neutral users and reviews before serving facts. Prove duplicate membership, incremental reads, partial failures, cancellation, private persistence, bot exclusion and snooze survival using fixture providers. No routes or background refresh yet.

## Notes

**agent:codex/inbox-20261002** at 2026-10-03T02:42:43Z

Implemented optional forge.Discoverer and inbox.Service without agent/review/ticket imports. Complete membership plus nonzero update-stamp equality enables safe incremental reads; failed discovery or any item read retains the previous complete account snapshot. All displayed errors use fixed categories, never arbitrary provider text. Native/configured bot identities are applied only to independent views of raw cached facts, avoiding sticky configured marks. Snoozes persist separately and survive refresh and cache deletion.

Fixture tests cover both forge kinds, duplicate roles, unchanged/missing/new stamps, rate limits, cancellation, untrusted errors, partial reads, restart and private files. just ci passed with CGO_ENABLED=0 across all cross targets, frontend and ticket checks; a subsequent bot propagation correction passed inbox tests. Waiting for PR review and native CI before closing.

**agent:codex/inbox-20261002** at 2026-10-03T02:44:01Z

PR #6: https://git.local.sothr.com/terva-sh/vuoro/pulls/6. Native macOS CI passed. Corrected the new cache permission test to assert Unix mode bits only on Unix, matching state/state_test.go; Windows does not expose Unix 0600 mode bits. Persistence and bot-removal assertions still run on every platform. The implementation's private state writer is unchanged.
