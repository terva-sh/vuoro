---
schema: 3
id: TKT-01M3YMSV4KMDHB5H0KDRD82K52
title: Layer a repository's review checklist and conventions, read at base
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - review
assignees: []
milestone: v1.0.0
parent: TKT-01M3YMSTYMFRDFXD4QSVWHMM8P
origin: null
dependencies:
  - TKT-01M3YK0CM0SWMYXMM7A7M7J2QX
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

A repository can add review instructions, such as a checklist and conventions
in `.vuoro/`, layered on top of the person's preset. They are read at the base
commit, never the head, so that a change cannot steer its own review. The
person always picks the reviewers. A repository only adds instructions.

## Acceptance criteria

- [ ] A repository checklist reaches every reviewer's prompt
- [ ] A change that edits the checklist is reviewed with the base commit's version, and a test proves it
