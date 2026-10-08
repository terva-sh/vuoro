---
schema: 3
id: TKT-01M3YMSVN79DEXW47ZXB519KVE
title: "Publish vuoro: GitHub mirror, and docs for someone new"
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - docs
  - release
assignees: []
milestone: v1.0.0
parent: TKT-01M3YMSTYMFRDFXD4QSVWHMM8P
origin: null
dependencies:
  - TKT-01M3YMSVKKGJ3R8EXZ8QZAMQAJ
  - TKT-01M3YMSVJ2BTQN940P1XR3CJD6
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-02T15:47:52Z
updated_at: 2026-10-08T03:01:56Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
extensions: {}
---

## Description

The terva-sh release. Add the GitHub mirror, with macOS and Windows CI there as
tuohi has it, and write documentation for somebody who is not the owner:
install, set up, run a first review, and what each agent can and cannot do.

## Acceptance criteria

- [ ] The GitHub mirror exists and its CI runs on main
- [ ] The README takes a new person from install to a first review

## Notes

**agent:claude/forgejo-parity-20261006** at 2026-10-08T03:01:56Z

D52 (2026-10-08) removed .github/workflows/ci.yml. GitHub Actions had stopped starting jobs for an account billing problem, and the owner limited testing to linux on the internal runners. Publishing now has to bring macOS and Windows CI back, with the race detector and window check on macOS, rather than keep it. The criterion 'The GitHub mirror exists and its CI runs on main' needs a new workflow.
