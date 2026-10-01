---
schema: 3
id: TKT-01M3W4HWW7FHHC6VD5M0S9MV90
title: Decide how vuoro embeds terva for agent actions
type: spike
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - agent
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

docs/plan.md section 6 describes two in-process entry points, the SDK
(`packages/agent/sdk`) and the engine (`packages/core` with
`packages/provider`). Pick one, and say why.

Read before deciding:

- `terva/docs/embedding.md`, `examples/sdk`, and `examples/harness`
- `terva/.api/packages.txt`. Import only what it lists as stable.
- terva's `review-crew` personas, under
  `packages/agent/persona/personas/builtin/review-crew`
- `terva-sh/terva-action-code-review`, which already runs terva as a pull
  request reviewer. It is private, and the bootstrap session could not read it.

Decide: where provider credentials come from (terva's own config or vuoro's),
the permission gate, which read-only tools an agent gets, how streamed output
reaches the window, and the import-boundary test that keeps terva inside
`assist`.

## Acceptance criteria

- [ ] docs/plan.md section 6 records the chosen entry point and why, with the open-question marker removed
- [ ] A throwaway program streams one terva completion over a recorded diff on this Mac
- [ ] The tool list for agents is written down, and it contains no forge write
