---
schema: 3
id: TKT-01M61JQ2ZEAMQB7Q46QS2YDSKV
title: Replace the Summary tab with an Ask tab on the change page
type: task
status: done
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
claim: null
archive: null
created_at: 2026-10-28T15:40:31Z
updated_at: 2026-10-29T18:40:20Z
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

- [x] Prompt buttons and a question box start a conversation with the chosen tool and model
- [x] The change's conversations list with prompt, head, cost and a stale mark
- [x] The person follows up and stops in the session pane
- [x] A stale conversation offers the same question on the new head
- [x] Earlier summaries show as explain conversations

## Notes

**agent:claude-code/ask-20261029** at 2026-10-29T14:36:02Z

vuoro review round 2 (run 20261029-143042-7e4c13, d14): criterion 1 says 'the chosen tool and model', but the tab offers a tool only. The model is the tool's default unless a prompt file names one; plan section 6.11 now says so. A model picker is left out deliberately until there is a list of models to pick from; the setup reports tools, not models.

**agent:claude-code/ask-20261029** at 2026-10-29T15:03:04Z

The work has landed (through b2d38a5). The ticket stays in progress until a vuoro self-review round over 474f7f8..HEAD raises nothing above low and nothing in production code; round 4 (run 20261029-145238-5bc642) did not, and its fixes are b2d38a5.

**agent:claude-code/ask-20261029** at 2026-10-29T16:06:33Z

Criterion 1 is left unticked for its model half: the tab picks a tool, and a model comes only from a prompt file's front matter (on the tool it names) or the tool's default. A model picker waits for a list of models to pick from. Supersedes the 15:03 note: vuoro review round 6 raised only lows, fixed in 16f5933.

**agent:claude-code/ask-20261029** at 2026-10-29T18:40:20Z

Criterion 1 is now met in full, superseding the note that left its model half open: the Ask tab has a free-text Model field beside Tool, as the preset editor has, and a model typed there wins over a prompt's (da86e6f; web/src/ask.test.tsx asks with tool claude and model opus). vuoro review rounds over c30f479..2192875 ended clean at round 6 (run 20261029-165827-bd3ef3).

## Summary

Landed in web/src/ask.tsx, replacing the Summary tab in change.tsx, with app/prompts.go (GET /api/prompts) and app/ask.go. The tab offers the shipped and the person's prompts (a file it cannot use is named with the reason), a question box, scope, review and tool; lists the change's conversations newest first with prompt, head, cost and a stale mark; shows the selected one in the session pane with follow-up and Stop; offers a stale one again on the new head with its own prompt, scope and review; and shows a pre-Ask summary as explain, read-only. Tests: web/src/ask.test.tsx (seven), app TestAPromptsScopeDecidesTheRange, TestAPromptsModelGoesOnlyToItsOwnTool. Self-reviewed with vuoro rounds 1-6.
