---
schema: 3
id: TKT-01M49AZ7ZCCKXMDDRVAT9MFNMD
title: Replace the four polling loops with one usePoll hook
type: task
status: ready
status_reason: null
priority: normal
due_on: null
labels:
  - ui
assignees: []
milestone: null
parent: TKT-01M49AYNBWFRYZSBAD5BBJKTB5
origin: null
dependencies:
  - TKT-01M49AZ6QVMSD43KAB1V927RAN
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-06T19:27:42Z
updated_at: 2026-10-06T19:49:52Z
created_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
updated_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
extensions: {}
---

## Description

run.tsx, summary.tsx, triage.tsx, and inbox.tsx each write a setTimeout loop with its own cadence and stop condition. Add a generic usePoll(fn, interval) hook that stops on unmount, and use it in all four.

## Acceptance criteria

- [ ] A generic usePoll exists with a test for its cadence and its stop on unmount
- [ ] All four loops use it
