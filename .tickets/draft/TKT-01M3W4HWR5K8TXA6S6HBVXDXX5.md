---
schema: 3
id: TKT-01M3W4HWR5K8TXA6S6HBVXDXX5
title: "Inbox view: every change, sorted by whose turn and how long"
type: task
status: draft
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

D34 selects one inbox for GitLab and Forgejo, ordered by whose turn it is. Integrate the shared discovery/refresh service, both account adapters and the pure turn engine into the main window.

Show two groups, my turn and their turn, each sorted by the earliest unanswered event, oldest first. Each row shows forge/project, title, account role, readable reason and age, and opens the existing change/review workspace. Retain a link to the forge. Present per-account refresh errors and last-success/stale information; a failing account must not masquerade as an empty inbox or erase another account’s results.

Refresh manually and on a bounded interval only while the window is open. Keep local snoozes separate from derived cache state; a snoozed change returns at its date or when a new forge event arrives. The inbox builds and works with all agent tools absent and no ticket store. Human posting refreshes forge facts and derived turns; it never guesses posted state.

## Acceptance criteria

- [ ] Opening a row opens the change view, and each row also links to the forge URL
- [ ] A snoozed change returns when its date passes or a new event arrives
- [ ] Changes from configured GitLab and Forgejo accounts share both turn groups with correct reason, age, account roles and oldest-first ordering.
- [ ] One account failing or being rate-limited preserves other results and shows visible stale/error state.
- [ ] The inbox works with no agent tools and no ticket store; refresh stays bounded to the open window.
