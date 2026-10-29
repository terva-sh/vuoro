---
schema: 3
id: TKT-01M61JQ2FJ4QGFZWPJE2456FQB
title: Serve ask sessions read-only git history and the change over MCP
type: task
status: ready
status_reason: null
priority: normal
due_on: null
labels:
  - agent
  - sandbox
  - review
assignees: []
milestone: v1.0.0
parent: TKT-01M61JPA9TYM392M8XBAZ6RQV4
origin: null
dependencies:
  - TKT-01M61JQ22E7049NSPR545323DW
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-28T15:40:31Z
updated_at: 2026-10-29T13:59:18Z
created_by:
  id: agent:claude-code/ask-plan
  name: ""
updated_by:
  id: agent:claude-code/ask-plan
  name: ""
extensions: {}
---

## Description

vuoro's MCP server `context` for ask sessions, the only MCP server they get. Its tools only read:

- `git_log`, `git_show`, `git_blame`, `git_diff` in vuoro's clone. A revision is a full commit ID of the run or a name vuoro resolves to one; paths are relative and inside the repository; every argument follows `--end-of-options`; diff programs and textconv are off; output is capped and refused past the cap, never truncated silently.
- `change`, `discussion`, `pipeline` answer from the forge read vuoro made when the turn started, and send no request of their own.

What the tools return is untrusted data, and the ask system prompt says so.

Design: docs/plan.md section 6.11, D68.

## Acceptance criteria

- [ ] git_log, git_show, git_blame and git_diff answer from the clone and refuse an option, an outside path, or an unknown revision
- [ ] Output past the cap is refused with the reason
- [ ] change, discussion and pipeline answer from the turn's forge read, and a test proves they send no request
- [ ] No context tool writes to the clone, the worktree, or the forge
