---
schema: 3
id: TKT-01M61JQ2PCA8DRN7GWKX94V16R
title: Load canned ask prompts from the config directory beside shipped ones
type: task
status: ready
status_reason: null
priority: normal
due_on: null
labels:
  - review
  - state
assignees: []
milestone: v1.0.0
parent: TKT-01M61JPA9TYM392M8XBAZ6RQV4
origin: null
dependencies: []
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

Canned prompts as files. vuoro ships `explain` (the summary prompt), `risks`, `test`, and `start-here`. The person adds Markdown files under `prompts/` in vuoro's config directory, each an optional front matter of `title`, `scope` (`change` or `since_review`), `tool`, and `model`, then the prompt. As presets do (D61), theirs supplement the shipped ones, one with a shipped name shadows it, and theirs sort first. A file that cannot be read or parsed is listed with its reason and never stops the others. Served at `GET /api/prompts` behind the window guard.

Design: docs/plan.md section 6.11, D68.

## Acceptance criteria

- [ ] vuoro ships explain, risks, test and start-here
- [ ] A prompt file's front matter sets title, scope, tool and model
- [ ] The person's prompts sort first, and one with a shipped name shadows it
- [ ] A bad file is listed with its reason and the rest still load
- [ ] GET /api/prompts is behind the window guard
