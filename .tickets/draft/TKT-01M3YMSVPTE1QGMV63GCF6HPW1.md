---
schema: 3
id: TKT-01M3YMSVPTE1QGMV63GCF6HPW1
title: "Editor on others' changes: findings as suggested patches"
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - agent
  - review
assignees: []
milestone: null
parent: null
origin: null
dependencies:
  - TKT-01M3YMSVEW8443QTX654QMJRX0
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-02T15:47:52Z
updated_at: 2026-10-05T20:09:18Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude-code/editor-20261005
  name: ""
extensions: {}
---

## Description

After v1.0.0. The editor turns a reviewer finding on somebody else's change into
a suggested patch, posted as a forge suggestion block on the person's action.

## Acceptance criteria

- [ ] A finding becomes a suggestion draft that applies cleanly on the forge

## Notes

**agent:claude-code/editor-20261005** at 2026-10-05T20:09:18Z

D37 (reviewers draft suggestion blocks with their findings) and D39 (the editor fixes kept findings in a stacked draft merge request) now cover most of what this ticket asked for. Consider closing it, or narrowing it to whatever is left, when someone picks it up.
