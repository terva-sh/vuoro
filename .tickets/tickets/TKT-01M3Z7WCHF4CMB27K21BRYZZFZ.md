---
schema: 3
id: TKT-01M3Z7WCHF4CMB27K21BRYZZFZ
title: Verify each finding, and mark its confidence
type: task
status: in-progress
status_reason: null
priority: high
due_on: null
labels:
  - agent
  - review
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies: []
blocks_on: none
references: []
claim:
  actor: agent:claude/vuoro-mvp-1
  branch: tickets/triage
  worktree: /Users/drewshort/Workspace/github.com/terva-sh/vuoro
  commit: 84de04eee1b88331586ecd130b74ecaa26e971bc
  session: null
  claimed_at: 2026-10-02T21:29:38Z
  expires_at: null
archive: null
created_at: 2026-10-02T21:21:18Z
updated_at: 2026-10-02T21:29:38Z
created_by:
  id: agent:claude/vuoro-mvp-1
  name: ""
updated_by:
  id: agent:claude/vuoro-mvp-1
  name: ""
extensions: {}
---

## Description

D31. After the synthesizer, or after the reviewers when there is none, a
read-only verifier session reads every finding the person works from and
checks it against the worktree. It marks each one confirmed, plausible, or
doubtful with a one-line reason, through a drafts tool that only the verifier
role has. The person can override a mark in the UI. Marks show as ✅ ❔ ⛈ beside
a word, never by colour alone.

## Acceptance criteria

- [ ] A verifier marks every finding the person works from, with a reason
- [ ] Only the verifier role can rate, and a test proves the tool lists per role
- [ ] The person can override a mark, and the override is kept
