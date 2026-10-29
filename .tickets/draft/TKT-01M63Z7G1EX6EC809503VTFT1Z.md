---
schema: 3
id: TKT-01M63Z7G1EX6EC809503VTFT1Z
title: Keep getting-started progress in prefs.json behind the guarded API
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - state
  - ui
assignees: []
milestone: v1.0.0
parent: TKT-01M63Z778B87DTP765W8JWSMPN
origin: null
dependencies:
  - TKT-01M63Z7FYTJMBF0DXT563PVMW1
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

Both the empty-screen guidance and the tour need to remember what a person has seen and dismissed. app/prefs.go already keeps the home tab and the waived confirmations in `prefs.json`. Add the getting-started progress there: the hints dismissed, and the tour's state (not started, the step reached, finished, or turned off).

The page reads it with the rest of `/api/setup` and writes it through the guarded API. A missing or old `prefs.json` reads as a fresh start, never as an error. Resetting it is how Settings replays the tour.

## Acceptance criteria

- [ ] prefs.json holds the dismissed hints and the tour's state, read with /api/setup
- [ ] Writes go through the window-origin protection, with a test that a request without it is refused
- [ ] A missing or older prefs.json reads as a fresh start, tested
