---
schema: 3
id: TKT-01M3Z7WC760Q9864DGDRYAAETJ
title: "Make a large review readable: compact findings, Markdown, folding"
type: task
status: draft
status_reason: null
priority: high
due_on: null
labels:
  - ui
  - review
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-02T21:21:18Z
updated_at: 2026-10-02T21:21:18Z
created_by:
  id: agent:claude/vuoro-mvp-1
  name: ""
updated_by:
  id: agent:claude/vuoro-mvp-1
  name: ""
extensions: {}
---

## Description

A real review (next-gen-compute!144: 47 files, 4 sessions, 15 drafts) is hard
to read. Every finding shows twice, once merged and once from its reviewer.
Every card is fully open, the four session panes push the findings below the
fold, and the description shows raw Markdown.

Make the findings a compact list: one row per finding, with its severity,
confidence, title, place, and decision, opened in place. Hide a merged
finding's sources under it, and keep what was not carried forward in its own
folded list. Show a triage count. Fold the session panes to one line each once
they are idle. Render descriptions, notes, and drafts as Markdown, without
ever parsing them as HTML, in a measure that reads well. Stop the file list
cutting paths off.

## Acceptance criteria

- [ ] Each finding shows once, with its sources folded under it
- [ ] Descriptions, notes, and drafts render as Markdown, and no input reaches the page as HTML, which a test proves
- [ ] Idle session panes fold to one line, and open on a click
