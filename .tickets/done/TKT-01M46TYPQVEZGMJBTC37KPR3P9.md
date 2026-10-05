---
schema: 3
id: TKT-01M46TYPQVEZGMJBTC37KPR3P9
title: Fix a review's kept findings in an editor session
type: task
status: done
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
updated_at: 2026-10-05T21:02:52Z
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

- [x] Fix these in a branch starts one editor on a run with kept findings whose reviewed head is still the change's head, and refuses otherwise
- [x] The editor works in the chosen sandbox, in its own worktree at the reviewed head, with no forge credential, and a test proves it
- [x] It commits once per finding it addresses and reports the ones it left alone, with reasons
- [x] The run shows the editor's commits as a diff beside their findings, and Discard removes the branch and worktree without touching the forge

## Notes

**agent:claude-code/editor-20261005** at 2026-10-05T21:02:51Z

Built and proven on branch tickets/editor.

- `review/editor.go`: StartEditor, Editor (the detail), DiscardEditor, the prompt and the first message (the hand-off report plus a finding-to-id map). The editor's repository is a fresh shallow repo seeded from vuoro's clone through `refs/vuoro/editor-base/<run>`; vuoro never runs git in it and reads commits back with `FetchEditor`. Closing the run discards the editor too.
- Driver (`agent/claude`): editor mode adds Edit, Write, Bash under `--restricted`, the sandbox settings, and an environment built from an allowlist.
- Found live: Claude Code's sandbox sets git config itself and drops GIT_CONFIG_GLOBAL, so git reads ~/.gitconfig and is refused. The driver now writes the editor's variables to a file and sets CLAUDE_ENV_FILE; overriding HOME instead breaks the login. Recorded in docs/editor-sandbox.md.
- `TestLiveEditor` (VUORO_LIVE_CLAUDE=1) ran the real claude: home canary refused by Read and cat, env canary absent, gitlab.com refused, fix committed with the person's identity. Passed 2026-10-05.
- Commits per finding and "left alone" reasons are asked for by the prompt and read back from the `Vuoro-Finding` trailer; the panel names every kept finding no commit answers. Nothing enforces one commit per finding beyond the prompt.
- Not built: the srt mode for an API key, filed as TKT-01M46Y0E9VBNZDCAN9P866YZBD. The UI panel has unit tests but was not looked at in the window against a real run.

## Summary

Fix these in a branch starts one sandboxed editor in its own repository on vuoro/fix-<n>-<run>; the run panel shows its commits beside their findings with the diff, and Discard deletes the repository and refs. Proven by review and app tests and a live run under Claude Code's sandbox. srt mode follows in TKT-01M46Y0E9VBNZDCAN9P866YZBD; pushing in TKT-01M46TYPS9WFBT865TP52ZH46X.
