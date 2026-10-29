---
schema: 3
id: TKT-01M63Z7G66X679ZJEHFAGK7FD1
title: Tour the window's main surfaces after setup finishes
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - ui
assignees: []
milestone: v1.0.0
parent: TKT-01M63Z778B87DTP765W8JWSMPN
origin: null
dependencies:
  - TKT-01M63Z7G1EX6EC809503VTFT1Z
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-29T13:57:41Z
updated_at: 2026-10-29T13:57:41Z
created_by:
  id: agent:claude-code/getting-started-tutorial
  name: ""
updated_by:
  id: agent:claude-code/getting-started-tutorial
  name: ""
extensions: {}
---

## Description

After `FirstRun` finishes, offer a short tour of the window: the home tabs and whose turn it is, opening a change by URL, the run panel and presets, talking to a reviewer, the drafts and the synthesizer, and Submit as the person's own action.

- A small Preact popover component, no new dependency, styled by terva-sh/design. Steps attach to `data-tour` markers on the real elements, and a step whose element is not on screen is skipped rather than shown floating.
- The tour only shows the interface. It never starts a run, opens a change, or writes to a forge on the person's behalf.
- Esc and a visible control end it at any step, and the ending is remembered.
- Settings (or About) replays it.
- `just web-build` and the committed `web/dist` come with the change.

## Acceptance criteria

- [ ] FirstRun's finish offers the tour, and declining it is remembered
- [ ] Each step anchors to a real element, and a step whose element is absent is skipped
- [ ] No step starts a run, opens a change, or writes to a forge, held by a test
- [ ] Esc ends the tour at any step, and Settings replays it
- [ ] web/dist is rebuilt and committed
