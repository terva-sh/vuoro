---
schema: 3
id: TKT-01M46TYPQVEZGMJBTC37KPR3P9
title: Fix a review's kept findings in an editor session
type: task
status: draft
status_reason: null
priority: high
due_on: null
labels:
  - agent
  - review
  - sandbox
assignees: []
milestone: v1.0.0
parent: null
origin: null
dependencies:
  - TKT-01M3YMSVD4Y1ME5QST9BY818XD
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-05T20:09:18Z
updated_at: 2026-10-05T20:09:18Z
created_by:
  id: agent:claude-code/editor-20261005
  name: ""
updated_by:
  id: agent:claude-code/editor-20261005
  name: ""
extensions: {}
---

## Description

D39 and docs/plan.md section 6.3 cover this. After a review of somebody else's change, its kept findings (the hand-off report) become the task of one editor session. The session runs in the sandbox the spike chooses, in a worktree at the reviewed head on a local branch `vuoro/fix-<number>-<run>`. It commits once per finding, and the run shows its commits as a diff beside the findings they answer.

## Acceptance criteria

- [ ] Fix these in a branch starts one editor on a run with kept findings whose reviewed head is still the change's head, and refuses otherwise
- [ ] The editor works in the chosen sandbox, in its own worktree at the reviewed head, with no forge credential, and a test proves it
- [ ] It commits once per finding it addresses and reports the ones it left alone, with reasons
- [ ] The run shows the editor's commits as a diff beside their findings, and Discard removes the branch and worktree without touching the forge
