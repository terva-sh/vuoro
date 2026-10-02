---
schema: 3
id: TKT-01M3W4HX4CJE62G9R124EV5PT4
title: Forgejo adapter for the forge interface
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
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-01T16:25:25Z
updated_at: 2026-10-02T15:47:51Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
extensions: {}
---

## Description

Implement the `forge` interface for Forgejo, which hosts terva-sh's internal
work. Forgejo's API follows Gitea's: pull request reviews, review comments, and
requested reviewers. Check what it records for thread resolution and a
reviewer's last reviewed SHA, because the turn rules depend on both.

## Acceptance criteria

- [ ] Forgejo pull requests appear in the inbox with correct turns, tested from fixtures
- [ ] Comment, reply, approve, request changes, and submit work against a real pull request
