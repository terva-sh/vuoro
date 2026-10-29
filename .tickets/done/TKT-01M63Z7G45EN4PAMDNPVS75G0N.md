---
schema: 3
id: TKT-01M63Z7G45EN4PAMDNPVS75G0N
title: Say what to do next on each empty or first-time screen
type: task
status: done
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
updated_at: 2026-10-29T14:12:26Z
created_by:
  id: agent:claude-code/getting-started-tutorial
  name: ""
updated_by:
  id: agent:claude-code/getting-started
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

- [x] Each screen in the description shows its hint when empty or first met
- [x] A dismissed hint stays dismissed across restarts
- [x] With no agent tool and no ticket store, every hint still renders and none points at a control that fails
- [x] web/dist is rebuilt and committed

## Summary

web/src/guide.tsx holds GuideContext, the Hint component (Got it dismisses for good, optional fallback text once dismissed) and NeedsTool, which names the agent tools vuoro knows when none answered. Six hints: reviews-no-forge, reviews-quiet (inbox), local-empty, projects-empty, change-no-runs (run panel, waits for the runs list so it does not flash), first-drafts (findings; a separate text for the person's own work). App keeps them in setup through useGuideState; before setup loads none shows. Settings > Getting started brings dismissed hints back. guide.test.tsx covers each screen showing, dismissing, staying dismissed after a restart, and rendering with no agent tool without offering Start a review. app/gettingstarted_test.go keeps the window's hint and step lists equal to app.Hints and app.TourSteps. web/dist rebuilt; just ci passes.
