---
schema: 3
id: TKT-01M49AZ7ZCCKXMDDRVAT9MFNMD
title: Replace the four polling loops with one usePoll hook
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
created_at: 2026-10-06T19:27:42Z
updated_at: 2026-10-06T19:58:38Z
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

- [x] A generic usePoll exists with a test for its cadence and its stop on unmount
- [x] All four loops use it

## Summary

poll.ts holds usePoll(tick, deps): tick runs at once and again after the wait it answers, so ticks never overlap; undefined or a throw stops it; a deps change or unmount aborts the signal tick was given, which ticks read to tell whether they may still set state. Tests cover cadence, no overlap, stopping, and abort and restart. The run poll (now useRunPoll, same 1s/3s/5s cadence), the summary and triage polls, and the inbox use it. The inbox's tick reads the cache first, then refreshes a minute after the last request settled, whichever request that was, so a manual refresh or a snooze still pushes the next one back; its requests share one gate and one signal aborted on close. One test changed: the inbox's overlap test flushes a few microtasks after the refresh settles, because the next tick is now scheduled a few hops later than the old inline setTimeout. Its assertions are unchanged.
