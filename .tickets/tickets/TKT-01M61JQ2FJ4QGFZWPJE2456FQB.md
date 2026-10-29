---
schema: 3
id: TKT-01M61JQ2FJ4QGFZWPJE2456FQB
title: Serve ask sessions read-only git history and the change over MCP
type: task
status: in-progress
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
claim:
  actor: agent:claude-code/ask-20261029
  branch: main
  worktree: /Users/drewshort/Workspace/github.com/terva-sh/vuoro
  commit: d548df2ff67c80cd99503caa233553a226e410d5
  session: null
  claimed_at: 2026-10-29T14:36:16Z
  expires_at: null
archive: null
created_at: 2026-10-28T15:40:31Z
updated_at: 2026-10-29T15:03:04Z
created_by:
  id: agent:claude-code/ask-plan
  name: ""
updated_by:
  id: agent:claude-code/ask-20261029
  name: ""
extensions: {}
---

## Description

vuoro's own MCP server, the one reviewers draft through, serves an ask session only these context tools, and the session gets no other MCP server. Its tools only read:

- `git_log`, `git_show`, `git_blame`, `git_diff` in vuoro's clone. A revision is `head`, `base` (the change's forge base), `reviewed` (the earlier review's head, in a since-review conversation only), or a commit ID, whole or abbreviated, optionally with `~N`, and it must be in the history of the change's head or base. Paths are relative and inside the repository, passed as literal pathspecs; revisions follow `--end-of-options` (blame gets a resolved commit ID); diff programs and textconv are off; an answer past 128 KiB is refused with the reason, never truncated.
- `change`, `discussion`, `pipeline` answer from the forge read vuoro made when the conversation started, refreshed before each follow-up, and send no request of their own. `pipeline` gives the status only, since vuoro does not read jobs or logs.

What the tools return is untrusted data, and the ask system prompt says so.

Design: docs/plan.md section 6.11, D68.

## Acceptance criteria

- [ ] git_log, git_show, git_blame and git_diff answer from the clone and refuse an option, an outside path, or an unknown revision
- [ ] Output past the cap is refused with the reason
- [ ] change, discussion and pipeline answer from the turn's forge read, and a test proves they send no request
- [ ] No context tool writes to the clone, the worktree, or the forge

## Notes

**agent:claude-code/ask-20261029** at 2026-10-29T15:03:04Z

The work has landed (through b2d38a5). The ticket stays in progress until a vuoro self-review round over 474f7f8..HEAD raises nothing above low and nothing in production code; round 4 (run 20261029-145238-5bc642) did not, and its fixes are b2d38a5.
