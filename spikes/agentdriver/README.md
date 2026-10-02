# agentdriver: the driver spike

The throwaway program and the evidence for TKT-01M3YK0CFFKAPM2QHZGGZMDHND
(Decide the agent driver interface for terva and Claude Code). docs/plan.md
section 6 holds what was decided. This directory holds what it was decided
from. Delete it once `agent/claude` and `agent/terva` exist.

The runs used Claude Code 2.1.285 and terva 0.139.7 on 2026-10-02, against a
synthetic two-file repository whose change is `transcripts/fixture.diff`.
The model was Claude Haiku 4.5, because the subject was the lockdown, not the
quality of the review.

## Claude Code

| Transcript | What it shows |
|---|---|
| `claude-review.jsonl`, `claude-review.out` | A three-turn streamed conversation under the lockdown. The session offers `Glob`, `Grep`, `Read`, and `mcp__vuoro__propose_draft`, and its only MCP server is vuoro's. The agent writes a draft through the stub drafts server. Asked to run a command, it has no tool for it. Asked to read `/etc/hosts`, the harness refuses: "outside the working directory". No canary file was created. |
| `claude-undefined-tools.jsonl` | Told to call `Bash`, `Write`, `ToolSearch`, and `Edit` anyway, the agent says it has none of them. A tool that is not defined cannot be called. |
| `claude-dontask-bash.jsonl` | The second layer. `Bash` is defined but not in `--allowedTools`, and under `--permission-mode dontAsk` the harness denies the call. The `result` event lists it in `permission_denials`. |
| `claude-worktree-locked.jsonl`, `claude-worktree-control.jsonl` | The worktree carries a `CLAUDE.md`, an `AGENTS.md`, and a `.claude/settings.json` hook that touches a canary. Under the lockdown none of them reaches the session, and the hook does not run. Without the lockdown, the control run quotes `CLAUDE.md` and the change's hook runs. |

The transcripts are scrubbed: session IDs, UUIDs, socket paths, and home
directories are replaced with placeholders.

## terva

There is no terva transcript. Two things stopped it:

- The workstation's terva login had expired ("Refresh token expired"), and no
  other provider was configured, so no model could answer.
- terva 0.139.7 has no supported way to give one run a server that vuoro
  supplies while terva keeps the person's login, and the person's hooks always
  load. docs/plan.md section 6.6 has the detail, with source paths.

`terva -p x --dump-prompt=sizes` with the candidate flags shows the tool set
offered without a model call: `read`, `grep`, `glob`, and `activate_tools`.
`activate_tools` can only reveal tools that are already registered, so it
cannot bring back `bash` or `write`.

## Rerun

```sh
CGO_ENABLED=0 go build -o /tmp/agentdriver ./spikes/agentdriver
/tmp/agentdriver claude -cwd REPO -record OUT.jsonl -drafts DRAFTS.jsonl "turn one" "turn two"
/tmp/agentdriver terva  -cwd REPO -record OUT.jsonl -drafts DRAFTS.jsonl "turn one" "turn two"
```

The terva command runs the lockdown and the conversation, but it does not wire
up the drafts server, for the reason above.
