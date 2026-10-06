---
schema: 3
id: TKT-01M49E8G4BFWFYNY6B6DFAD2P5
title: Give the home page's runs a folded section of their own
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - ui
  - inbox
assignees: []
milestone: null
parent: null
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-06T20:25:11Z
updated_at: 2026-10-06T20:27:02Z
created_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
updated_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
extensions: {}
---

## Description

The runs sit under the inbox as a bare heading and one line per run, so the list grows with every run and scrolls the page; 19 runs cover about 10 changes. Give them a card of their own, folded by default with a summary in its header, one row per change with its earlier runs folded under it, and a bounded height when open.

## Acceptance criteria

- [x] Runs are a card, folded by default, whose header says how many runs, changes, not closed, and their cost
- [x] One row per change, its latest run's status, drafts, and age, with its earlier runs listed under it on demand
- [x] Open, the list scrolls inside the card rather than the page
- [x] RunList moves to runs.tsx, with tests
- [x] just ci passes

## Summary

runs.tsx holds RunList and groupRuns. The runs are a card under the inbox, folded by default, whose header reads 'Runs N across M changes · K not closed · $total'. Open, it is a table with one row per change (latest status, title and where, run count, latest drafts, the change's total cost, age); a change with earlier runs folds them under a ▸ N toggle, and a local run's earlier runs open by their own id. The table scrolls inside the card, at most min(45vh, 420px), with a sticky header, and the card spans the full width beside the inbox. The fold is not remembered across launches. Tests cover grouping, the folded header, and the earlier runs; the existing run-list tests pass unchanged. Checked in light and dark with sample data in headless Chrome.
