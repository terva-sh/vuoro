---
schema: 3
id: TKT-01M3YMSVKKGJ3R8EXZ8QZAMQAJ
title: Release for macOS, Linux, and Windows, with self-update
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - release
  - ci
assignees: []
milestone: v1.0.0
parent: TKT-01M3YMSTYMFRDFXD4QSVWHMM8P
origin: null
dependencies:
  - TKT-01M3W4HWB551REE766G3TE2Y7D
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

goreleaser releases, as other terva-sh tools have them, and `vuoro self-update`,
as git-ticket has it. Decide macOS signing and notarization here.

## Acceptance criteria

- [ ] A tagged release publishes archives for macOS, Linux, and Windows
- [ ] vuoro self-update moves an installed release to the next one
- [ ] The macOS signing decision is recorded
