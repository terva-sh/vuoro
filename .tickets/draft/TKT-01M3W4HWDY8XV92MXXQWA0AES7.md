---
schema: 3
id: TKT-01M3W4HWDY8XV92MXXQWA0AES7
title: Refuse loopback API requests that did not come from vuoro's window
type: task
status: draft
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

Every process on the machine can reach 127.0.0.1, and this server will carry
the person's forge tokens. docs/plan.md section 5.2 requires the server to
refuse a request that cannot prove it came from vuoro's own window.

Candidates: a per-launch secret delivered through a tuohi binding or the
initial URL, an Origin and Host check, or both. Read tuohi's
`docs/architecture.md` on trusted origins first. `-browser` mode needs an
answer too, such as the secret in the printed URL.

This must land before the first forge write.

## Acceptance criteria

- [ ] A request without the per-launch credential gets 403, and a test proves it
- [ ] A request with a foreign Origin or Host gets 403, and a test proves it
- [ ] The window and -browser mode both work with the protection on
- [ ] docs/plan.md section 5.2 records the mechanism chosen
