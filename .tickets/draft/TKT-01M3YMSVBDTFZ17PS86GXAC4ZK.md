---
schema: 3
id: TKT-01M3YMSVBDTFZ17PS86GXAC4ZK
title: Tray, background refresh, and quiet turn notifications
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - desktop
  - inbox
assignees: []
milestone: v1.0.0
parent: TKT-01M3YMSTYMFRDFXD4QSVWHMM8P
origin: null
dependencies:
  - TKT-01M3W4HWR5K8TXA6S6HBVXDXX5
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-02T15:47:52Z
updated_at: 2026-10-02T15:47:52Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
extensions: {}
---

## Description

vuoro lives in the tray, using tuohi's tray, notify, and autostart. It refreshes
in the background, and it notifies when a change becomes the person's turn,
with the reason, such as "Mason replied on 2 threads". It also notifies when a
long review run finishes. Notifications are batched and quiet by default.

## Acceptance criteria

- [ ] Closing the window leaves vuoro in the tray, still refreshing
- [ ] A turn change produces one notification with its reason, and a burst of changes is batched
