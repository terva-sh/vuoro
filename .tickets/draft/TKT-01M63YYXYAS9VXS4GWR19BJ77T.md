---
schema: 3
id: TKT-01M63YYXYAS9VXS4GWR19BJ77T
title: Start tests from prompt files and May run code, and list them with Ask
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - ui
  - review
assignees: []
milestone: v1.0.0
parent: TKT-01M61JPA9TYM392M8XBAZ6RQV4
origin: null
dependencies:
  - TKT-01M63YYXR9C2DKP4619KT9Q6KP
  - TKT-01M61JQ2PCA8DRN7GWKX94V16R
  - TKT-01M61JQ2ZEAMQB7Q46QS2YDSKV
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-29T13:53:00Z
updated_at: 2026-10-29T13:53:00Z
created_by:
  id: agent:claude-code/ask-plan
  name: ""
updated_by:
  id: agent:claude-code/ask-plan
  name: ""
extensions: {}
---

## Description

Prompt files take `runs: true` in their front matter, which starts a probe instead of an ask run. vuoro ships `test-compare` (run the tests on base and head and compare) and `deps-impact` (for each changed dependency, find where the code uses it and exercise those paths on base and head). In the Ask tab, a free-form question goes to Ask unless the person ticks "May run code", which says what that allows and which tool runs it. Probes are listed with the change's conversations and on the Asked tab, marked as having run code, with elapsed time and cost.

Design: docs/plan.md section 6.12, D69.

## Acceptance criteria

- [ ] A prompt file with runs: true starts a probe
- [ ] vuoro ships test-compare and deps-impact
- [ ] May run code says what it allows before a free-form question becomes a probe
- [ ] Probes list with the change's conversations and on the Asked tab, marked, with elapsed time and cost
