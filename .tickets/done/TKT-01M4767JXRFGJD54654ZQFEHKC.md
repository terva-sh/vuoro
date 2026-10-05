---
schema: 3
id: TKT-01M4767JXRFGJD54654ZQFEHKC
title: Section the review tab and list the editor's files beside its diff
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - ui
  - review
assignees: []
milestone: v1.0.0
parent: null
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-05T23:26:23Z
updated_at: 2026-10-05T23:26:24Z
created_by:
  id: agent:claude-code/editor-20261005
  name: ""
updated_by:
  id: agent:claude-code/editor-20261005
  name: ""
extensions: {}
---

## Description

## Summary

The review tab is three labelled cards: Agents, Findings, and Hand back (the report and the editor). The editor's diff uses the change view's file column, now a shared FileList, with its own id prefix so picks open and scroll to the editor's copy. Checked in the window on terraform!2487.
