---
schema: 3
id: TKT-01M61JQR72VWJGK5AP67H34NG0
title: Search the reviews list by change, project, title, and finding
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - ui
  - review
assignees: []
milestone: null
parent: null
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-28T15:40:53Z
updated_at: 2026-10-28T15:40:53Z
created_by:
  id: agent:claude-code/ask-plan
  name: ""
updated_by:
  id: agent:claude-code/ask-plan
  name: ""
extensions: {}
---

## Description

The home page's list of review runs (web/src/runs.tsx, one row per change with its earlier runs folded under it) has no search, so finding an old review means scrolling. Add a search box that narrows the list by the change's title, reference, and project, and by the text of its findings, and keep the grouping by change. Local runs match by repository and title. This should share its search behaviour with the Asked tab's search (TKT-01M61JQ38K69DK3B2NW8TT0YR8): a scan of the kept text first, and an FTS5 index only if the scan is slow. Filed for later; not part of the Ask epic.

## Acceptance criteria

- [ ] A search box on the reviews list narrows it by title, reference, project, and finding text
- [ ] Matches keep the grouping by change, with the earlier runs of a matching change still reachable
- [ ] Local runs match by repository and title
- [ ] An empty result says what was searched
