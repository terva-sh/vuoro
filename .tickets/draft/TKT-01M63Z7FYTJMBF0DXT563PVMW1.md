---
schema: 3
id: TKT-01M63Z7FYTJMBF0DXT563PVMW1
title: Record the getting-started design in the plan and decisions
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - docs
assignees: []
milestone: v1.0.0
parent: TKT-01M63Z778B87DTP765W8JWSMPN
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-29T13:57:40Z
updated_at: 2026-10-29T13:57:40Z
created_by:
  id: agent:claude-code/getting-started-tutorial
  name: ""
updated_by:
  id: agent:claude-code/getting-started-tutorial
  name: ""
extensions: {}
---

## Description

A tutorial changes the design, so the plan and the decision record move first, in the same pull request as the first implementation or ahead of it.

- Add a section to docs/plan.md describing the three levels (empty-screen guidance, the tour, the practice review), where progress is kept, and how a person replays or turns them off.
- Record the owner's decision as the next D-number in docs/decisions.md, including what was rejected (for example, a third-party tour library, or a simulated submit).
- Update the README's "Opening changes" section once there is something to point to.

The owner decides the scope. This ticket records it.

## Acceptance criteria

- [ ] docs/plan.md has a section for getting started covering all three levels
- [ ] docs/decisions.md has the owner's decision as the next D-number, with what was rejected
- [ ] The README points a new person at the tutorial
