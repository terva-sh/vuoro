---
schema: 3
id: TKT-01M3W4HWR5K8TXA6S6HBVXDXX5
title: "Inbox view: every change, sorted by whose turn and how long"
type: task
status: in-progress
status_reason: null
priority: normal
due_on: null
labels:
  - inbox
  - ui
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies:
  - TKT-01M3W4HWGGB7RMVFAJPW2KPKY5
  - TKT-01M3W4HWNGYDHGXMSE91HKGTNT
  - TKT-01M3W4HWPT9NYP4ADW339FJDSC
  - TKT-01M3ZQXC5CPRDM3CD937C6PC3Q
  - TKT-01M3ZQXC6SWB7M9D46AN0QJAP8
blocks_on: none
references: []
claim:
  actor: agent:codex/inbox-20261002
  branch: tickets/inbox-view
  worktree: /home/sothr/workspace/git.local.sothr.com/terva-sh/vuoro
  commit: 857dc6e37efa612c6901cca20ed5d1368050411b
  session: null
  claimed_at: 2026-10-03T04:02:42Z
  expires_at: null
archive: null
created_at: 2026-10-01T16:25:25Z
updated_at: 2026-10-03T04:03:25Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:codex/inbox-20261002
  name: ""
extensions: {}
---

## Description

D34 selects one inbox for GitLab and Forgejo, ordered by whose turn it is. Integrate the shared discovery/refresh service, both account adapters and the pure turn engine into the main window.

Show two groups, my turn and their turn, each sorted by the earliest unanswered event, oldest first. Each row shows forge/project, title, account role, readable reason and age, and opens the existing change/review workspace. Retain a link to the forge. Present per-account refresh errors and last-success/stale information; a failing account must not masquerade as an empty inbox or erase another account’s results.

Refresh manually and on a bounded interval only while the window is open. Keep local snoozes separate from derived cache state; a snoozed change returns at its date or when a new forge event arrives. The inbox builds and works with all agent tools absent and no ticket store. Human posting refreshes forge facts and derived turns; it never guesses posted state.

## Acceptance criteria

- [ ] Opening a row opens the change view, and each row also links to the forge URL
- [ ] A snoozed change returns when its date passes or a new event arrives
- [ ] Changes from configured GitLab and Forgejo accounts share both turn groups with correct reason, age, account roles and oldest-first ordering.
- [ ] One account failing or being rate-limited preserves other results and shows visible stale/error state.
- [ ] The inbox works with no agent tools and no ticket store; refresh stays bounded to the open window.

## Implementation plan

Integrate the merged shared service, pure turn engine and both adapters via a light display projection containing account status and mine/theirs/snoozed rows, ordered by oldest known unanswered age with unknown ages last and stable Ref ties. Cached raw facts classify bots before deriving turns; projection omits patches, drafts and tokens. Record a raw event fingerprint with authored snoozes so expiry or new forge facts restores a row without config-only bot changes waking it. Add protected GET snapshot, POST refresh and POST snooze routes, initialized independently of tool probing or ticket stores. Refresh only while the home inbox is mounted, at most one frontend request at once, a minute after the previous request completes, with AbortController cleanup. Preserve stale account facts and visible last-success/errors. Rows open the existing workspace and link to the forge; a collapsed snoozed list restores work. After human writes, perform a serialized targeted fresh fact read so an older in-flight refresh cannot overwrite newer facts, preserving whole-account last-success semantics. Mixed-forge fixture, ordering/snooze/error, cancellation/polling, guard and human-write regressions plus light/dark browser validation precede full CI, review and merge. GitLab's implementation is merged; its personal-account live criterion remains blocked on authentication, so proceed with independent offline integration while preserving that blocker in the roadmap.
