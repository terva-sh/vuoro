---
schema: 3
id: TKT-01M63Z7G1EX6EC809503VTFT1Z
title: Keep getting-started progress in prefs.json behind the guarded API
type: task
status: done
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
updated_at: 2026-10-29T14:03:02Z
created_by:
  id: agent:claude-code/getting-started-tutorial
  name: ""
updated_by:
  id: agent:claude-code/getting-started
  name: ""
extensions: {}
---

## Description

Both the empty-screen guidance and the tour need to remember what a person has seen and dismissed. app/prefs.go already keeps the home tab and the waived confirmations in `prefs.json`. Add the getting-started progress there: the hints dismissed, and the tour's state (not started, the step reached, finished, or turned off).

The page reads it with the rest of `/api/setup` and writes it through the guarded API. A missing or old `prefs.json` reads as a fresh start, never as an error. Resetting it is how Settings replays the tour.

## Acceptance criteria

- [x] prefs.json holds the dismissed hints and the tour's state, read with /api/setup
- [x] Writes go through the window-origin protection, with a test that a request without it is refused
- [x] A missing or older prefs.json reads as a fresh start, tested

## Implementation plan

Add Hints (dismissed hint IDs, filtered to the known set in app.Hints) and Tour {state: '', on, done, off; step: a known tour step ID} to prefs. Unknown values from an old or edited file read as a fresh start, as Scheme and Tab already do. /api/setup returns hints and tour beside waived. Routes POST /api/prefs/hint {hint, dismissed}, POST /api/prefs/hints/reset, POST /api/prefs/tour {state, step}, all on the same mux behind server.New's guard. Tests: restart round-trip, unknown values refused, an older prefs.json and a missing one read fresh, and every new route refused for missing token, other origin, rebound host, and the agent token.

## Summary

prefs.json holds hints (dismissed IDs, filtered to app.Hints) and tour {state, step}, both returned by /api/setup. POST /api/prefs/hint, /api/prefs/hints/reset and /api/prefs/tour sit on the app mux behind server.New's guard; app/gettingstarted_test.go proves all three refuse a missing token, another origin, a rebound host and the agent token, and write nothing when refused. Missing, older, edited and damaged files read as a fresh start, and dismissing a hint keeps an older file's waived, scheme and tab. web/src/types.ts carries Hint and Tour; both are optional there so a fixture without them reads as a fresh start.
