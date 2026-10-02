---
schema: 3
id: TKT-01M3W4HWCNR50KXMCX741QD9XQ
title: "Walking skeleton: a loopback server shown in a tuohi window"
type: task
status: done
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
updated_at: 2026-10-02T16:55:32Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-mvp-1
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

- [x] vuoro opens a window on macOS that shows a page served by its own loopback server
- [x] vuoro -browser serves the same page and prints a 127.0.0.1 URL
- [x] vuoro -check exits 0 when the page reached the server and a binding, and 1 otherwise
- [x] The server binds 127.0.0.1 only and refuses another address

## Implementation plan

Follow tuohi/examples/loopback. server.Listen accepts 127.0.0.1 only. server.New serves web.Dist(), a placeholder page embedded from web/dist that the Preact ticket replaces, and GET /api/hello. main keeps the UI loop on the main goroutine. It has -browser (print the URL, serve until interrupted) and -check (the page reports through the vuoro.report binding, with a 30s bound). The GUI test runs vuoro in a child test binary, because only TestMain has the main goroutine. It is gated on TUOHI_REQUIRE_GUI=1, with a just test-gui recipe and a step on the GitHub macOS runner.

## Notes

**agent:claude/vuoro-mvp-1** at 2026-10-02T16:55:32Z

Verified on this Mac (macOS, Go 1.26.1, tuohi v0.1.0) on 2026-10-02:
- `vuoro -check` opened the window, and the page reported `ok=true server="dev" binding="dev"`, exit 0. `just test-gui` passes the same check under -race.
- `vuoro -browser` printed http://127.0.0.1:PORT/. The page and /api/hello answered 200, and SIGINT exits 0.
- server.Listen refuses 0.0.0.0, ::, ::1, localhost, 127.0.0.2, a LAN address, a hostname, and an address with no port. TestANonLoopbackAddressIsRefused covers it through the binary's flags.

Two things for the next tickets:
- The tests pass in a docker.io golang:1.27-alpine container without gcompat, which tuohi's own CI installs. If a test that reaches purego fails to start on Alpine later, add gcompat to the Forgejo workflow's apk line.
- tuohi's macOS engine is now proven on this workstation, not only on GitHub's runners. TKT-01M3YMST0M467K1FB4J5B2C7B2 (Move vuoro to terva-sh/vuoro on Forgejo once the skeleton lands) is now unblocked, and it is the owner's push.

## Summary

vuoro opens a tuohi window over its own 127.0.0.1 server, -browser serves the same page and prints the URL, and -check exits 0 once the page has reached both the server and a Go binding. All of it was verified on this Mac. The server refuses any address but 127.0.0.1. The window check runs as just test-gui and on the GitHub macOS runner.
