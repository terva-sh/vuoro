---
schema: 3
id: TKT-01M3YMSVD4Y1ME5QST9BY818XD
title: Decide the editor agent's sandbox
type: spike
status: draft
status_reason: null
priority: high
due_on: null
labels:
  - agent
  - sandbox
assignees: []
milestone: v1.0.0
parent: TKT-01M3YMSTYMFRDFXD4QSVWHMM8P
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-02T15:47:52Z
updated_at: 2026-10-02T15:47:52Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
extensions: {}
---

## Description

The editor agent runs the change's code, so its boundary matters, and it must
hold without cgo. The default position is a container (podman or docker) with
only the editor's worktree mounted, no forge credentials, and network limited
to the model endpoint and package registries. Compare it with each tool's own
sandbox (terva's lock, Claude Code's sandbox) and with what macOS offers
natively, and decide.

Reviewers are unaffected. They stay read-only, always.

## Acceptance criteria

- [ ] docs/plan.md records the sandbox chosen and the alternatives rejected, with reasons
- [ ] A recorded run shows the editor unable to reach a forge or read a credential
