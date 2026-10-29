---
schema: 3
id: TKT-01M63Z778B87DTP765W8JWSMPN
title: "Getting started: teach a new person to use vuoro in the window"
type: epic
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - ui
  - docs
assignees: []
milestone: v1.0.0
parent: null
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-29T13:57:32Z
updated_at: 2026-10-29T13:57:32Z
created_by:
  id: agent:claude-code/getting-started-tutorial
  name: ""
updated_by:
  id: agent:claude-code/getting-started-tutorial
  name: ""
extensions: {}
---

## Description

vuoro's first run (`FirstRun`, web/src/setup.tsx) configures the app: a forge, its token, the agent tools, and the background settings. Nothing then shows a new person how to use it: the inbox and whose turn it is, opening a change, running a preset, talking to reviewers, editing drafts, and submitting. docs/plan.md section 10.2 asks for "documentation for somebody new" with the v1.0.0 release. This epic is that work, inside the window.

It is built in three levels, cheapest first:

1. Guidance on each empty or first-time screen, saying what to do next.
2. A step-through tour after setup, anchored to the real interface.
3. A practice review on a generated local repository. This is a spike first, because it spends real agent money and needs an agent tool installed.

### Constraints the whole epic keeps

- **An agent drafts. A person posts.** No step of a tutorial submits anything to a forge, and no step simulates a submit that could reach one.
- **The loopback server is attack surface.** Any route the tutorial adds goes through the window-origin protection, with no exception.
- **The inbox works with every agent tool absent and with no ticket store.** So does every tutorial step that does not need them, and the ones that do say why and skip.
- **The forge is the record.** Tutorial progress is the person's own annotation, kept in `prefs.json` beside `tab` and `waived`, never as review state.
