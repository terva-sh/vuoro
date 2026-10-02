---
schema: 3
id: TKT-01M3YK0CM0SWMYXMM7A7M7J2QX
title: "Run a review: worktree, review profile, and validated findings"
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - review
  - agent
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies:
  - TKT-01M3YK0CH5RHPJJS7E0DX9SH8X
  - TKT-01M3YK0CJM8AJQH8129KEEF9TM
  - TKT-01M3W4HWNGYDHGXMSE91HKGTNT
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-02T15:16:29Z
updated_at: 2026-10-02T15:16:29Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
extensions: {}
---

## Description

vuoro runs reviews itself. For a change, check out the head in a worktree under
the state directory, compute the diff against the merge base, and start a
session with the person's chosen driver, a review profile, and the diff. The
agent reports findings, each with severity, title, body, evidence, and an
optional anchor (path, side, SHA, line range). vuoro validates every finding,
and it tells the agent which rule a bad one broke, so that the agent can fix it.

Profiles start as code, security, and documentation. A repository can add a
checklist and conventions. Where those live is part of this ticket.

terva-sh/terva-action-code-review is the prior art: its `templates/`, its
finding and anchor shapes in `schemas/review-result.v1.schema.json`, and its
rule that reviewer settings are read at the base commit, so a change cannot
choose its own reviewer. Borrow ideas, not code or schema.

## Acceptance criteria

- [ ] A review of a real GitLab change produces validated findings with either driver
- [ ] An invalid finding goes back to the agent with the rule it broke, and a test proves it
- [ ] A repository's review settings are read at the base commit, never the head
- [ ] The worktree is removed when the review is closed
