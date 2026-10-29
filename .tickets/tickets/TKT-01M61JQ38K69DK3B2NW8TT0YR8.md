---
schema: 3
id: TKT-01M61JQ38K69DK3B2NW8TT0YR8
title: List and search every kept conversation from an Asked tab
type: task
status: ready
status_reason: null
priority: normal
due_on: null
labels:
  - ui
  - state
assignees: []
milestone: v1.0.0
parent: TKT-01M61JPA9TYM392M8XBAZ6RQV4
origin: null
dependencies:
  - TKT-01M61JQ2ZEAMQB7Q46QS2YDSKV
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

An Asked tab on the home page lists every conversation across changes, newest first, searchable over the questions and the answers, and filterable by project, change, and prompt. Each row links to its change's Ask tab with that conversation selected. The search is a scan of the kept text; an FTS5 index (`modernc.org/sqlite`) is added only if the scan proves slow. A conversation can be deleted, after a confirmation, and its run state goes with it. Conversations are left out of the retention of review runs.

Design: docs/plan.md section 6.11, D68.

## Acceptance criteria

- [ ] The Asked tab lists every conversation across changes, newest first
- [ ] Search matches questions and answers; filters narrow by project, change and prompt
- [ ] A row opens its change's Ask tab on that conversation
- [ ] Deleting a conversation asks first and removes its run state
- [ ] Review-run retention never removes a conversation
