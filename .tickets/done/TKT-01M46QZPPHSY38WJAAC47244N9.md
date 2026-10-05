---
schema: 3
id: TKT-01M46QZPPHSY38WJAAC47244N9
title: Start a large change's files collapsed
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - review
  - ui
  - desktop
assignees: []
milestone: null
parent: null
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-05T19:17:25Z
updated_at: 2026-10-05T19:24:38Z
created_by:
  id: agent:claude-code/keep-collapse-20261005
  name: ""
updated_by:
  id: agent:claude-code/keep-collapse-20261005
  name: ""
extensions: {}
---

## Description

On a change as large as terraform!2487 (263 files), every file starts expanded, so the Changes tab is very long. The owner asked for files to start collapsed on a large change. Clicking a file in the left list should expand it if it was collapsed, and collapse it again when they move to another file, if they had not expanded it themselves.

## Acceptance criteria

- [x] A change past a file-count threshold opens with every file collapsed
- [x] Picking a file in the list expands it, and collapses it again when another is picked, unless the person expanded it themselves

## Notes

**agent:claude-code/keep-collapse-20261005** at 2026-10-05T19:24:37Z

Threshold: more than 25 files (manyFiles in web/src/diffview.tsx). Also fixed: a picked file's header landed under the sticky top bar, which made the diff look as if it started mid-file. .file now has scroll-margin-top. Checked headless on terraform!2487.

## Summary

A change with more than 25 files opens with every file collapsed. Picking a file from the list or a finding opens it, and the next pick folds it again unless the person toggled it by hand. A picked file now clears the sticky top bar.
