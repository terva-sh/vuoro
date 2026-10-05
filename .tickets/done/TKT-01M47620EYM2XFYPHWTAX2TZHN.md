---
schema: 3
id: TKT-01M47620EYM2XFYPHWTAX2TZHN
title: "Drop Draft: from the reviewed title in a stacked MR's title"
type: chore
status: done
status_reason: null
priority: low
due_on: null
labels:
  - review
  - gitlab
assignees: []
milestone: v1.0.0
parent: null
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-05T23:23:20Z
updated_at: 2026-10-05T23:24:15Z
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

StackTitle drops Draft:, WIP:, [Draft] and (Draft) from the reviewed title, since GitLab adds its own Draft: prefix; found on !2487, whose title carried one mid-title.
