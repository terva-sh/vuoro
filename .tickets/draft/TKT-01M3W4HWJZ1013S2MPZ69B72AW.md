---
schema: 3
id: TKT-01M3W4HWJZ1013S2MPZ69B72AW
title: Decide the forge model and the turn rules
type: spike
status: draft
status_reason: null
priority: high
due_on: null
labels:
  - forge
  - inbox
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-01T16:25:25Z
updated_at: 2026-10-01T16:25:25Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
extensions: {}
---

## Description

docs/plan.md section 4 is a starting position: change, role, review, thread,
draft, and turn, with turn rules for a reviewer and for an author. Test it
against real GitLab, GitHub, and Forgejo API responses before any adapter is
written. It is cheaper to find a forge that does not fit now than after the
first adapter has shaped the interface.

Questions to answer:

- Does each forge expose what the turn rules need: last review SHA per
  reviewer, note times per thread, re-requested reviews?
- How are a GitLab approval, a "changes requested" verdict, which GitLab does
  not have natively, and a dismissed review represented?
- Is "my last note in the thread" enough to decide an author's turn, or does
  a thread need to know who is expected to answer?
- What does `since` mean when several reasons apply?

`~/Workspace/code-reviews/tools/` holds working GitLab fetch code, in Python.
It is prior art for the API calls, and its responses are real shapes.

## Acceptance criteria

- [ ] docs/plan.md section 4 is rewritten as the decided model, with the open-question marker removed
- [ ] Each turn rule names the forge fields it reads, for GitLab, GitHub, and Forgejo
- [ ] Recorded API fixtures for at least one GitLab change exist for the turn tests to use
