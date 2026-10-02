---
schema: 3
id: TKT-01M3YMSVGFW0CNC7YQ8DHBJBAZ
title: Record finding dispositions as git-ticket notes
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - git-ticket
  - review
assignees: []
milestone: v1.0.0
parent: TKT-01M3YMSTYMFRDFXD4QSVWHMM8P
origin: null
dependencies:
  - TKT-01M3W4HX1PHPQ98KX0PE5HW60E
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

The v1.0.0 half of the git-ticket integration. The reviewer records each
finding's disposition (accepted, declined, deferred, with a reason) as a note on
the linked ticket. The note is written to the working tree of the person's
checkout and never committed. The editor sees the linked ticket as context too.

## Acceptance criteria

- [ ] A disposition note is written with an attributed actor, and git status shows it uncommitted
- [ ] The editor's prompt includes the linked ticket
