---
schema: 3
id: TKT-01M49H46P8XE4NAVEMBG2S87E4
title: Count only active branches as needing the person in Local
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - state
  - ui
assignees: []
milestone: null
parent: TKT-01M49F68YV694W81S3ESJKVH02
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-06T21:15:16Z
updated_at: 2026-10-06T21:16:47Z
created_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
updated_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
extensions: {}
---

## Description

Section 6.8 counted every branch never reviewed, which made the Local count about 118 on the owner's workspace. A branch with no commit in the last 14 days is quiet: it stays a row, but an unreviewed or moved branch counts as needing the person only while it is active. Undecided findings and a fix waiting to be merged count at any age, since each clears when the person acts. Decided by the owner on 2026-10-06 (D48).

## Acceptance criteria

- [x] A branch with no commit in 14 days is quiet and is not counted for never reviewed or moved
- [x] Undecided findings and a waiting fix branch count at any age
- [x] The view says which branches are quiet and the window it used, and the Local tab shows them apart
- [x] The plan and decisions record the rule

## Summary

A branch whose tip has no commit in 14 days (`quietAfter` in app/local.go) is quiet: listed, with `quiet: true`, but not counted for being never reviewed or moved since its review. Undecided findings and a waiting fix branch count at any age. The view carries `quiet_days`, and the Local panel lists quiet branches as their own line. Recorded as D48 and in docs/plan.md section 6.8; on the owner's workspace this takes the count from about 127 to about 27. The window is a constant, not a setting; a pref is a later change if 14 days proves wrong.
