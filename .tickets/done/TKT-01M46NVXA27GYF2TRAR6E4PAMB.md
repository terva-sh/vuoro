---
schema: 3
id: TKT-01M46NVXA27GYF2TRAR6E4PAMB
title: Opening a change with an empty file crashes the request
type: bug
status: done
status_reason: null
priority: high
due_on: null
labels:
  - gitlab
  - forge
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-05T18:40:23Z
updated_at: 2026-10-05T18:41:08Z
created_by:
  id: agent:claude-code/empty-file-20261005
  name: ""
updated_by:
  id: agent:claude-code/empty-file-20261005
  name: ""
extensions: {}
---

## Description

On 2026-10-05 the owner could not open flywheel-io/infrastructure/terraform!2487. GitLab sends no diff for an empty file, so the adapter rebuilds it from both versions, and patch.Unified("", "") panicked: Myers' search read v[off+1] with zero lines on both sides, which is out of range. net/http recovered the panic and dropped the connection, so the window showed only a failed load.

## Acceptance criteria

- [x] patch.Unified of two empty files returns no hunks
- [x] terraform!2487 opens
- [x] A panicking handler answers with a JSON error, not a dropped connection, and the panic's value stays out of the answer

## Notes

**agent:claude-code/empty-file-20261005** at 2026-10-05T18:41:07Z

forge/patch: Unified returns "" for identical inputs, and diff returns nil when both sides have no lines; TestEmptyFiles panics on the old code. server: a recovered handler behind the guard turns a panic into a 500 with a fixed JSON error, and logs the value and stack only to vuoro's log (TestAPanicAnswersWithAnError). Live: terraform!2487 opens in about 5s with 263 files and 61 threads.

## Summary

Two empty file versions diff to nothing instead of panicking, so terraform!2487 opens. Any future panic in a request now reaches the window as a readable error.
