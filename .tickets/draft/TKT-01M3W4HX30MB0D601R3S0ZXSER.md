---
schema: 3
id: TKT-01M3W4HX30MB0D601R3S0ZXSER
title: GitHub adapter for the forge interface
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - forge
  - github
assignees: []
milestone: null
parent: null
origin: null
dependencies:
  - TKT-01M3W4HWTXC6EM9SS84A4WMEPG
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-01T16:25:25Z
updated_at: 2026-10-01T16:25:25Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
extensions: {}
---

## Description

Implement the `forge` interface for GitHub: review requests, pull request
reviews and their states, review threads, and pending reviews, which are the
GitHub equivalent of GitLab draft notes. GraphQL may be the better fit for
threads and resolution, so decide in the plan.

A part of GitHub that does not fit the interface is a finding against the
model in docs/plan.md section 4. Do not leak GitHub detail through the
interface.

## Acceptance criteria

- [ ] GitHub pull requests appear in the inbox with correct turns, tested from fixtures
- [ ] Comment, reply, resolve, approve, request changes, and submit work against a real pull request
