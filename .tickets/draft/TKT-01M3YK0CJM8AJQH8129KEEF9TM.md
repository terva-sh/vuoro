---
schema: 3
id: TKT-01M3YK0CJM8AJQH8129KEEF9TM
title: Agent driver for terva over terva rpc
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - agent
assignees: []
milestone: v1.0.0
parent: TKT-01M3YMSTYMFRDFXD4QSVWHMM8P
origin: null
dependencies:
  - TKT-01M3YK0CFFKAPM2QHZGGZMDHND
  - TKT-01M3YRE0E00TRVKXP8YVASHDGD
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-02T15:16:29Z
updated_at: 2026-10-02T16:51:18Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-mvp-1
  name: ""
extensions: {}
---

## Description

Implement the driver interface from the spike for terva: spawn `terva rpc`
with the spike's lockdown flags, negotiate with `hello`, and use
`TERVACORE_RPC_TOKEN` so only vuoro can drive the process. Map `text_delta`,
`tool_call`, `tool_result`, `usage`, and `done` onto vuoro's events. Use
`--session PATH` under vuoro's state directory so a conversation can resume.

vuoro passes no model credential. terva uses its own configuration.

## Acceptance criteria

- [ ] A two-turn conversation streams through the driver against a recorded diff
- [ ] Abort stops the turn, and done arrives with no error
- [ ] Tests run without the network, from a recorded stream
- [ ] A missing or too-old terva binary produces a message that says which and what version is needed

## Notes

**agent:claude/vuoro-mvp-1** at 2026-10-02T16:51:18Z

Moved to v1.0.0 by decision D28 on 2026-10-02. terva 0.139.7 cannot be locked down as a reviewer. This ticket now waits on TKT-01M3YRE0E00TRVKXP8YVASHDGD (Prove terva's reviewer lockdown once terva can take a per-run server), and the MVP tickets that depended on it no longer do.
