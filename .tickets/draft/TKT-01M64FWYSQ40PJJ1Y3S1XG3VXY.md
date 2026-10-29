---
schema: 3
id: TKT-01M64FWYSQ40PJJ1Y3S1XG3VXY
title: Queue a message sent to a reviewer mid-turn for the turn's end
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - review
  - ui
assignees: []
milestone: v1.0.0
parent: TKT-01M3YMSTYMFRDFXD4QSVWHMM8P
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-29T18:49:01Z
updated_at: 2026-10-29T18:49:01Z
created_by:
  id: agent:claude-code/release-0.1.0
  name: ""
updated_by:
  id: agent:claude-code/release-0.1.0
  name: ""
extensions: {}
---

## Description

A message sent to a reviewer while its turn is running is refused today
("the session is still working; wait, or stop it"). D73 shipped v0.1.0 with
talk between turns, and moved this here.

Queue the message instead, and send it when the turn ends. The pane shows it
as queued, and the person can withdraw it before it is sent. Stopping the
reviewer sends nothing queued.

## Acceptance criteria

- [ ] A message sent during a turn is queued and sent when the turn ends
- [ ] The pane shows a queued message, and the person can withdraw it
- [ ] Stopping the reviewer drops what is queued, and says so
