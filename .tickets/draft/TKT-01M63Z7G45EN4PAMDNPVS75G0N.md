---
schema: 3
id: TKT-01M63Z7G45EN4PAMDNPVS75G0N
title: Say what to do next on each empty or first-time screen
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

Each screen a new person meets empty should say what to do next, in place, and be dismissable for good:

- the Reviews inbox with no forges, and with forges but nothing yet
- the Local tab with no folders or roots
- the Projects tab with nothing added
- a change with no runs yet: what a preset is, and that running one costs money
- the first drafts a run produces: that they are drafts, editable, and reach the forge only on Submit

Each hint is short, styled by terva-sh/design, and dismissed hints are kept through the progress store. A hint that needs an agent tool names the missing tool rather than pointing at a button that will fail.

## Acceptance criteria

- [ ] Each screen in the description shows its hint when empty or first met
- [ ] A dismissed hint stays dismissed across restarts
- [ ] With no agent tool and no ticket store, every hint still renders and none points at a control that fails
- [ ] web/dist is rebuilt and committed
