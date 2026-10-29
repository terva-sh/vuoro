---
schema: 3
id: TKT-01M61JQ2ZEAMQB7Q46QS2YDSKV
title: Replace the Summary tab with an Ask tab on the change page
type: task
status: in-progress
status_reason: null
priority: normal
due_on: null
labels:
  - ui
  - review
assignees: []
milestone: v1.0.0
parent: TKT-01M61JPA9TYM392M8XBAZ6RQV4
origin: null
dependencies:
  - TKT-01M61JQ22E7049NSPR545323DW
  - TKT-01M61JQ2PCA8DRN7GWKX94V16R
blocks_on: none
references: []
claim:
  actor: agent:claude-code/ask-20261029
  branch: main
  worktree: /Users/drewshort/Workspace/github.com/terva-sh/vuoro
  commit: 864508399b58f37671c0b9be6a987de18e97db14
  session: null
  claimed_at: 2026-10-29T14:24:15Z
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

The change page's Ask tab replaces the Summary tab. A row of prompt buttons from the prompt list, a question box, the tool and model to ask, and the change's conversations newest first, each with its prompt or question, head, cost, and a stale mark. The selected conversation shows in the session pane with a box to follow up and Stop. A stale one says which heads differ and offers to ask the same first question on the new head. A prompt file with an error shows its reason.

Design: docs/plan.md section 6.11, D68.

## Acceptance criteria

- [ ] Prompt buttons and a question box start a conversation with the chosen tool and model
- [ ] The change's conversations list with prompt, head, cost and a stale mark
- [ ] The person follows up and stops in the session pane
- [ ] A stale conversation offers the same question on the new head
- [ ] Earlier summaries show as explain conversations

## Notes

**agent:claude-code/ask-20261029** at 2026-10-29T14:36:02Z

vuoro review round 2 (run 20261029-143042-7e4c13, d14): criterion 1 says 'the chosen tool and model', but the tab offers a tool only. The model is the tool's default unless a prompt file names one; plan section 6.11 now says so. A model picker is left out deliberately until there is a list of models to pick from; the setup reports tools, not models.

**agent:claude-code/ask-20261029** at 2026-10-29T15:03:04Z

The work has landed (through b2d38a5). The ticket stays in progress until a vuoro self-review round over 474f7f8..HEAD raises nothing above low and nothing in production code; round 4 (run 20261029-145238-5bc642) did not, and its fixes are b2d38a5.
