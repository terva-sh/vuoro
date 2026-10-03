---
schema: 3
id: TKT-01M3W4HX4CJE62G9R124EV5PT4
title: Forgejo posting through the shared forge boundary
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - forge
  - forgejo
assignees: []
milestone: v1.0.0
parent: TKT-01M3YMSTYMFRDFXD4QSVWHMM8P
origin: null
dependencies:
  - TKT-01M3W4HWTXC6EM9SS84A4WMEPG
  - TKT-01M3ZKGRZBBS869259MBA818NQ
  - TKT-01M3W4HWR5K8TXA6S6HBVXDXX5
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

Complete human-directed Forgejo posting through the shared forge boundary: comment, reply, approve, request changes and submit drafts, with live round-trip validation and capability-gated controls. Agents still only draft; every remote write requires the person’s protected UI action.

Open-by-URL reading/checkout is complete under TKT-01M3ZKGRZBBS869259MBA818NQ (Open Forgejo PRs through the shared forge boundary), merged as PR #3. D34 moves discovery and the mixed inbox into v0.1.0 under TKT-01M3ZQXC6SWB7M9D46AN0QJAP8 (Discover open Forgejo changes for the shared inbox). This v1.0 ticket now tracks writes only, avoiding an unrelated live-write prerequisite on read-only discovery.

## Acceptance criteria

- [ ] Comment, reply, approve, request changes, and submit work against a real pull request
- [ ] Human-directed writes refresh forge facts and the shared inbox turn; agents have no posting route.

## Notes

**agent:codex/forgejo-20261002** at 2026-10-03T00:44:56Z

The owner brought the open-by-URL read/review slice into v0.1 on 2026-10-02, tracked as TKT-01M3ZKGRZBBS869259MBA818NQ (Open Forgejo PRs through the shared forge boundary). This ticket retains discovery/inbox and human-directed writes for v1.0, including the existing live GitLab-write dependency. The new slice proceeds independently; it does not claim this ticket's inbox or posting criteria.
