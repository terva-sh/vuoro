---
schema: 3
id: TKT-01M3W4HWCNR50KXMCX741QD9XQ
title: "Walking skeleton: a loopback server shown in a tuohi window"
type: task
status: draft
status_reason: null
priority: high
due_on: null
labels:
  - desktop
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies:
  - TKT-01M3W4HWB551REE766G3TE2Y7D
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

`vuoro` opens a tuohi window over a `net/http` server on `127.0.0.1`, and
`vuoro -browser` serves the same thing and prints the URL. Follow
`tuohi/examples/loopback`: the server runs in a goroutine, and main keeps the
UI loop, which macOS requires to be on the main thread.

Serve a placeholder page and one JSON endpoint. That proves the window, the
server, and a tuohi binding end to end on this Mac before any forge code
exists.

A `-check` mode that loads the page, confirms it reached Go, and exits 0 or 1
gives CI a GUI smoke test, as the tuohi example does.

## Acceptance criteria

- [ ] vuoro opens a window on macOS that shows a page served by its own loopback server
- [ ] vuoro -browser serves the same page and prints a 127.0.0.1 URL
- [ ] vuoro -check exits 0 when the page reached the server and a binding, and 1 otherwise
- [ ] The server binds 127.0.0.1 only and refuses another address
