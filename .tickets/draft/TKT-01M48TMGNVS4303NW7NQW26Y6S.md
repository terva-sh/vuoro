---
schema: 3
id: TKT-01M48TMGNVS4303NW7NQW26Y6S
title: Build reviewer presets in the window
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - ui
assignees: []
milestone: null
parent: TKT-01M3YMSTYMFRDFXD4QSVWHMM8P
origin: null
dependencies:
  - TKT-01M3YMSVJ2BTQN940P1XR3CJD6
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-06T14:42:13Z
updated_at: 2026-10-06T14:42:17Z
created_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
updated_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
extensions: {}
---

## Description

Split from TKT-01M3YMSVJ2BTQN940P1XR3CJD6 (Set up forges and check agent tools in the window), whose first slice covers forges and the tool check only, as the owner chose on 2026-10-06.

Build, edit, and remove reviewer presets in the window: reviewers with a profile, tool and model each, plus an optional synthesizer and verifier, written to config.json through the same safe save as forges. Until then the built-in default, one code reviewer, applies when the config names none.

## Acceptance criteria

- [ ] A preset can be created, edited and removed in the window, and a run can start from it
