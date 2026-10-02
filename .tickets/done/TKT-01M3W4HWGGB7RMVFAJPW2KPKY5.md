---
schema: 3
id: TKT-01M3W4HWGGB7RMVFAJPW2KPKY5
title: Scaffold the Preact frontend with committed, embedded web/dist
type: task
status: done
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
updated_at: 2026-10-02T18:53:04Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-mvp-1
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

- [x] go build with no Node toolchain produces a binary that serves the Preact app
- [x] just ci fails when web/dist differs from a fresh build of web/src
- [x] Frontend typecheck and unit tests run in just ci

## Implementation plan

Copy git-ticket-canvas's layout: package.json and the lockfile at the root, with the same pins minus playwright. Vite's root is web, building into web/dist, and web.go embeds all:dist. web/src/api.ts carries the guard's token, from the binding or the URL fragment. scripts/verify-dist.mjs builds into a temp dir and compares the result with web/dist. The justfile gets web-setup, web-build, web-typecheck, web-test, and dist-verify, and ci runs the last three. Both workflows install Node and run them.

## Notes

**agent:claude/vuoro-mvp-1** at 2026-10-02T18:53:03Z

Verified on 2026-10-02:
- With only go on PATH (no node, npm, or npx), go build produced a binary, and vuoro -check passed in the window with the Preact app.
- Editing web/src/app.tsx without rebuilding made verify:dist fail and name both files. A rebuild matched again.
- vitest covers the token handling: the binding, the fragment taken once and removed from the address bar, the sessionStorage reload, no token, and a malformed fragment.
- npm install of the bare package.json hit an npm arborist bug ('reading edgesOut'), so the lockfile was seeded from git-ticket-canvas's, which has the same pins.

## Summary

web/ is a Preact and TypeScript app built by Vite into the committed, embedded web/dist, so go build needs no Node. just ci typechecks, unit-tests, and fails when web/dist differs from a fresh build. Both workflows run the same steps.
