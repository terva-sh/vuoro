---
schema: 3
id: TKT-01M64HWQB50H51QEZNM7J8VDZ6
title: Mark a cost unknown only when a running turn ended without usage
type: bug
status: done
status_reason: null
priority: normal
due_on: null
labels:
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
created_at: 2026-10-29T19:23:51Z
updated_at: 2026-10-29T19:30:19Z
created_by:
  id: agent:claude-code/release-0.1.0
  name: ""
updated_by:
  id: agent:claude-code/v0.1.1
  name: ""
extensions: {}
---

## Description

Found by the v0.1.0 release review, round 2 (run 20261029-185914-d7fdb6),
after v0.1.0 was tagged. For a v0.1.1 patch.

`Session.noteUnreported` marks a session's cost unknown when a turn ends
with an error or a cancel and no usage. The Claude driver also sends such
an event when no turn ran, so nothing was spent:

- a lockdown refusal on `system/init`, before any `result`
  (agent/claude/claude.go, `ErrLockdown`)
- the process exiting while idle, neither closed nor cancelled

The flag is persisted and nothing clears it, so that session, its run, the
runs card and the headless report show "cost unknown" or "at least $x" for
a cost that is fully known.

Fix: set the flag only when a turn was running, by recording the turn's
start on the send path and checking it in `noteUnreported`.

Also from that round: the comments on the run-level flag name only one of
its two causes (review/view.go `RunView.CostUnknown`, review_cmd.go
`reviewResultRun.CostUnknown`, review/findings.go `runCost`,
web/src/types.ts `Run.cost_unknown`), and plan 6.2 still says a pane shows
cost "as the tool reports them".

## Acceptance criteria

- [x] A lockdown refusal or an idle exit, with no turn running, leaves the cost known
- [x] A turn stopped or failed while running, with no usage, still marks it unknown
- [x] The run-level flag's comments and plan 6.2 name both causes

## Implementation plan

Fix it in the driver, which alone knows whether a turn reached the model: claude emits a zero Usage before TurnDone when the lockdown refuses the session on its init line, and when the process exits between turns. noteUnreported then sees usage for that turn and leaves the cost known; a crash mid-turn or a stop still reports none. Document the zero-usage rule on agent.Session.Send and the Usage kind. Reword the run-level flag's comments and plan 6.2.

## Summary

The claude driver now reports zero usage for a turn that never reached the model: a lockdown refusal on the init line, and an exit between turns. The review and summary pumps then leave the cost known, while a turn stopped or failed mid-turn with no usage still marks it unknown. agent.Session documents the rule for any driver. Tested in TestASessionOfferingAnotherToolIsEndedAtOnce and TestAnExitBetweenTurnsSpentNothing (each fails without its zero usage; the fake claude can now exit after its last turn), and a refused case in TestATurnStoppedBeforeItsUsageLeavesTheCostUnknown. The run-level flag's comments and plan 6.2 name both causes.
