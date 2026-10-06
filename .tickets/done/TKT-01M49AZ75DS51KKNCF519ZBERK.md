---
schema: 3
id: TKT-01M49AZ75DS51KKNCF519ZBERK
title: Add a useAction hook for busy and error state around a request
type: task
status: done
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
created_at: 2026-10-06T19:27:41Z
updated_at: 2026-10-06T19:51:25Z
created_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
updated_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
extensions: {}
---

## Description

The idiom e instanceof Error ? e.message : String(e) appears 21 times, each beside a hand-rolled busy and error useState pair. Add errorText(e) and a useAction() hook, and use them.

## Acceptance criteria

- [x] errorText and useAction exist with tests
- [x] No component repeats the error idiom

## Summary

action.ts holds errorText and useAction, with tests for busy state, the error and its clearing, and nothing set after unmount. RunPanel's start, the Editor's actions, PushBar's preview, PushDialog's push, and Talk's send use useAction; every other catch uses errorText, so the idiom lives only in errorText. Bare String(e) sites (the hello check, run lists, and the summary and triage loads) stay as they were, because errorText would drop the 'Error:' prefix the person sees today. One timing change: PushDialog now leaves busy after a successful push, just before it closes, where it used to stay busy until it closed.
