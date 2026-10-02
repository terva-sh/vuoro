---
schema: 3
id: TKT-01M3YMSV6BABG7EW1NP8J6W5SR
title: Review a local branch or range with no forge
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - review
  - agent
assignees: []
milestone: v1.0.0
parent: TKT-01M3YMSTYMFRDFXD4QSVWHMM8P
origin: null
dependencies:
  - TKT-01M3YK0CPYG132897XVVZ9G4EF
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

Point vuoro at a checkout and a range, such as `origin/main..HEAD`, before
anything is pushed. The orchestrator, panes, and synthesis are the same as for
a forge change. Findings become a local checklist, and they carry into the
merge request if the person opens one.

## Acceptance criteria

- [ ] A local range runs the same preset as a forge change
- [ ] Its findings carry into the merge request opened from that branch
