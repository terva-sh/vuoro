---
schema: 3
id: TKT-01M3ZQXC5CPRDM3CD937C6PC3Q
title: Share forge discovery, caching, and refresh for the inbox
type: task
status: draft
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
claim: null
archive: null
created_at: 2026-10-03T02:01:27Z
updated_at: 2026-10-03T02:01:27Z
created_by:
  id: agent:codex/windows-roadmap-20261002
  name: ""
updated_by:
  id: agent:codex/windows-roadmap-20261002
  name: ""
extensions: {}
---

## Description

D34 selects a single GitLab-and-Forgejo inbox as the next product milestone. Add a narrow, optional discovery contract beside the existing forge Provider/Writer contracts, and an account-scoped service that refreshes derived forge state for the inbox. Agents and ticket stores retain their own boundaries.

Keep pagination and API details in adapters; the service owns bounded/cancellable refresh, deduplicated identities and roles, last-success/error information, and cache updates. A failed or rate-limited account must not erase a healthy account’s results or silently look like an empty inbox. Authored snoozes are separate from derived snapshots. Apply configured bot identity information before turn derivation, preserving the model’s exclusion of bot/system activity. Keep existing Ref/run identities stable.

Build with fixture providers first so both adapter tasks can implement the same behavior. This task does not implement either forge’s discovery endpoints, a forge write, the turn rules, or the frontend.

## Acceptance criteria

- [ ] A narrow optional forge discovery contract works with fixture providers without importing agent or tickets.
- [ ] Account results deduplicate change identities and roles while preserving existing Ref identities.
- [ ] A failed, cancelled or rate-limited refresh preserves prior account snapshots and exposes last-success/error state without leaking credentials.
- [ ] Configured bot identities are applied before turn derivation, and authored snoozes survive cache replacement.
- [ ] Tests cover mixed-account refresh and partial failures without network I/O or installed agent tools.
