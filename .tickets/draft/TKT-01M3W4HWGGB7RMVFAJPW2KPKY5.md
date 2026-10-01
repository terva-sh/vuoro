---
schema: 3
id: TKT-01M3W4HWGGB7RMVFAJPW2KPKY5
title: Scaffold the Preact frontend with committed, embedded web/dist
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - ui
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies:
  - TKT-01M3W4HWCNR50KXMCX741QD9XQ
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

Set up `web/` as git-ticket-canvas does: Preact and TypeScript, built with
Vite, assets committed in `web/dist` and embedded with `go:embed`, so a plain
`go build` needs no Node. Copy the canvas's justfile recipes for `web-setup`,
`web-build`, `web-typecheck`, and `web-test`, and its check that the committed
bundle matches the source.

Replace the skeleton's placeholder page with the Preact app.

## Acceptance criteria

- [ ] go build with no Node toolchain produces a binary that serves the Preact app
- [ ] just ci fails when web/dist differs from a fresh build of web/src
- [ ] Frontend typecheck and unit tests run in just ci
