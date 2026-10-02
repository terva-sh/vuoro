---
schema: 3
id: TKT-01M3YMSV82VVCF6E0CBDKPCQK0
title: Re-anchor drafts, and review only the new commits
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
  - TKT-01M3YMSSVNA1TJVQ425EBHSWZ7
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

Builds on the MVP's pinning. When a change moves, re-anchor the drafts whose
lines did not change, and offer "review the new commits" as a follow-up run
that sees the earlier findings. It never runs automatically.

## Acceptance criteria

- [ ] A draft on unchanged lines follows the new head
- [ ] An incremental run reviews only the new commits, and it receives the earlier findings as context
