---
schema: 3
id: TKT-01M3Z7WCQF2NTTV9312ZZQYTRS
title: Ask a reviewer for a suggestion block on a finding
type: task
status: draft
status_reason: null
priority: high
due_on: null
labels:
  - gitlab
  - review
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-02T21:21:18Z
updated_at: 2026-10-02T21:21:18Z
created_by:
  id: agent:claude/vuoro-mvp-1
  name: ""
updated_by:
  id: agent:claude/vuoro-mvp-1
  name: ""
extensions: {}
---

## Description

D31. On a pending finding with a head-side anchor, the person asks for a
suggestion. vuoro asks the session that drafted it to revise the draft with
replacement text for exactly the anchored lines. The card shows the
suggestion, and keeping it writes a GitLab suggestion block, sized to the
range, into the comment. The person can keep it without the suggestion.

## Acceptance criteria

- [ ] Asking for a suggestion gets replacement lines from the drafting session
- [ ] A kept finding with a suggestion posts a suggestion block that covers its range, which a test proves
- [ ] A suggestion is only ever on a head-side anchor, and the rule says so
