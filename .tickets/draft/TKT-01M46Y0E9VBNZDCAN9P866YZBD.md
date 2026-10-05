---
schema: 3
id: TKT-01M46Y0E9VBNZDCAN9P866YZBD
title: Run the editor under srt when an API key is configured for it
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - agent
  - sandbox
  - credentials
assignees: []
milestone: v1.0.0
parent: null
origin: null
dependencies:
  - TKT-01M46TYPQVEZGMJBTC37KPR3P9
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-05T21:02:40Z
updated_at: 2026-10-05T21:02:51Z
created_by:
  id: agent:claude-code/editor-20261005
  name: ""
updated_by:
  id: agent:claude-code/editor-20261005
  name: ""
extensions: {}
---

## Description

D40 says the editor runs under the sandbox runtime (`srt`) instead of Claude Code's own sandbox when the person configures an API key for the editor. TKT-01M46TYPQVEZGMJBTC37KPR3P9 built the local default only.

### What it needs

- A config setting naming the age-store secret that holds the editor's model credential (an API key, or a `claude setup-token` token), read the way a forge token is read, never in argv or a config file.
- `srt` found and its version checked; vuoro refuses to start an editor when it is missing or its config does not validate, and never falls back to running unsandboxed.
- A per-run Claude config directory (`CLAUDE_CONFIG_DIR`), since Seatbelt hides the Keychain and the person's `~/.claude`.
- The `srt` config from docs/editor-sandbox.md: reads of `~/` denied but the worktree, caches, config dir and Claude install; writes to those only; network to the model endpoint and the registries.
- A live test like `TestLiveEditor`, run inside `srt`, which closes the spike's last unproven criterion.
