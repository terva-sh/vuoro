---
schema: 3
id: TKT-01M3W4HWDY8XV92MXXQWA0AES7
title: Refuse loopback API requests that did not come from vuoro's window
type: task
status: done
status_reason: null
priority: high
due_on: null
labels:
  - desktop
  - credentials
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
updated_at: 2026-10-02T17:00:13Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-mvp-1
  name: ""
extensions: {}
---

## Description

Every process on the machine can reach 127.0.0.1, and this server will carry
the person's forge tokens. docs/plan.md section 5.2 requires the server to
refuse a request that cannot prove it came from vuoro's own window.

Candidates: a per-launch secret delivered through a tuohi binding or the
initial URL, an Origin and Host check, or both. Read tuohi's
`docs/architecture.md` on trusted origins first. `-browser` mode needs an
answer too, such as the secret in the printed URL.

This must land before the first forge write.

## Acceptance criteria

- [x] A request without the per-launch credential gets 403, and a test proves it
- [x] A request with a foreign Origin or Host gets 403, and a test proves it
- [x] The window and -browser mode both work with the protection on
- [x] docs/plan.md section 5.2 records the mechanism chosen

## Implementation plan

An exact Host check, plus Origin and Sec-Fetch-Site checks on every route. A per-launch token in the X-Vuoro-Token header on /api/, compared in constant time. The window gets it from a tuohi binding, which only the trusted origin can call. -browser gets it in the URL fragment, which the page moves into sessionStorage. No cookie, because cookies are not isolated by port.

## Notes

**agent:claude/vuoro-mvp-1** at 2026-10-02T17:00:13Z

Verified on 2026-10-02. server/server_test.go has the tests: 403 for no token, an empty, wrong, or prefix token, a token sent as Authorization, a rebound or localhost Host, another port, a foreign, null, or other-port Origin, and cross-site or same-site fetch metadata. vuoro -check passes in the real window with the guard on. vuoro -browser in headless Chrome loads the API with the printed URL. Without the fragment, the page says to open the printed URL. curl gets 403 without the token and 403 with a foreign Host.

## Summary

Every route checks Host, Origin, and Sec-Fetch-Site, and /api/ needs a per-launch token, which the window reads from a tuohi binding and -browser passes in the URL fragment. docs/plan.md section 5.2 records it, server/guard.go holds it, and tests prove each refusal.
