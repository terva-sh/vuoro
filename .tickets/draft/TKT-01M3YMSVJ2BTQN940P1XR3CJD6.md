---
schema: 3
id: TKT-01M3YMSVJ2BTQN940P1XR3CJD6
title: Set up forges and agent tools in the UI, without editing config
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - ui
  - credentials
assignees: []
milestone: v1.0.0
parent: TKT-01M3YMSTYMFRDFXD4QSVWHMM8P
origin: null
dependencies:
  - TKT-01M3W4HWM6V2F20KNT03BSBNZY
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

For somebody who is not the owner: add a forge and its token source, detect
terva and Claude Code and their versions, and build a first reviewer preset,
all from the window. Hand-edited config keeps working.

## Acceptance criteria

- [ ] A fresh install reaches a first review without editing a file
- [ ] The UI never displays a stored token
