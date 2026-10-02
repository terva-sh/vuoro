---
schema: 3
id: TKT-01M3YRE0E00TRVKXP8YVASHDGD
title: Prove terva's reviewer lockdown once terva can take a per-run server
type: spike
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - agent
  - cross-repo
assignees: []
milestone: v1.0.0
parent: TKT-01M3YMSTYMFRDFXD4QSVWHMM8P
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-02T16:51:18Z
updated_at: 2026-10-02T16:51:18Z
created_by:
  id: agent:claude/vuoro-mvp-1
  name: ""
updated_by:
  id: agent:claude/vuoro-mvp-1
  name: ""
extensions: {}
---

## Description

terva 0.139.7 cannot run as a locked reviewer (docs/plan.md section 6.6, decision D28). It has no way to take vuoro's drafts server for one run while it keeps the person's login. `--mcp` only narrows the configured servers, `--team-socket` is hard-wired to the Talkoot bridge, and moving `TERVA_HOME` moves `auth.json` with it. The person's config hooks always load, and no flag turns them off.

When a terva release closes both gaps, prove the lockdown the way the Claude Code half of TKT-01M3YK0CFFKAPM2QHZGGZMDHND (Decide the agent driver interface for terva and Claude Code) did. Use the throwaway program in spikes/agentdriver, or its successor in agent/, and record the transcripts. Then update section 6.6. No change to terva is asked for now; D28 records that.

The flags that already hold are in section 6.6: `--tools read,grep,glob --no-ext --no-mcp --no-skill --no-lore --no-memory --no-ticket --portable=strict`, with `--approval auto-edit --rpc-approvals`, or plan mode with a `readOnlyHint` drafts tool.

## Acceptance criteria

- [ ] A terva release is named that supplies one run's MCP server and keeps the person's login
- [ ] The person's terva hooks do not load in a reviewer session, and a recorded canary shows it
- [ ] A recorded transcript shows terva refusing to run a command under the lockdown, and writing a draft through vuoro's server
- [ ] docs/plan.md section 6.6 records terva's lockdown, with the open-question marker removed
