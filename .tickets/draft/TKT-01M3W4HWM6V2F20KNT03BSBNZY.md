---
schema: 3
id: TKT-01M3W4HWM6V2F20KNT03BSBNZY
title: Decide credential handling and the local state format
type: spike
status: draft
status_reason: null
priority: high
due_on: null
labels:
  - credentials
  - state
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

docs/plan.md sections 5.1 and 8 leave two decisions open.

Credentials. The starting position is a configured `token_command`, run
without a shell and read from stdout, with an environment variable as fallback.
A token never goes in a config file, argv, or a log. Decide whether a keychain
is worth it for v0.1 without cgo, which means `security` on macOS and Secret
Service over D-Bus on Linux.

State. Decide the cache format. JSON files may be enough for one person's
inbox. `modernc.org/sqlite` is the cgo-free option if queries need it. Keep
derived state (the cache) separate from authored state (snoozes, notes), so
deleting the cache loses nothing the person wrote.

Decide the config file's location and shape: forges, hosts, usernames, token
sources.

## Acceptance criteria

- [ ] docs/plan.md sections 5.1 and 8 record the decisions, with the open-question markers removed
- [ ] The config file format is documented with an example that contains no secret
