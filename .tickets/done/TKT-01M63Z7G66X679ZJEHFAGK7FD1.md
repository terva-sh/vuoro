---
schema: 3
id: TKT-01M63Z7G66X679ZJEHFAGK7FD1
title: Tour the window's main surfaces after setup finishes
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
updated_at: 2026-10-29T14:40:50Z
created_by:
  id: agent:claude-code/getting-started-tutorial
  name: ""
updated_by:
  id: agent:claude-code/getting-started
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

- [x] FirstRun's finish offers the tour, and declining it is remembered
- [x] Each step anchors to a real element, and a step whose element is absent is skipped
- [x] No step starts a run, opens a change, or writes to a forge, held by a test
- [x] Esc ends the tour at any step, and Settings replays it
- [x] web/dist is rebuilt and committed

## Notes

**agent:claude-code/getting-started** at 2026-10-29T14:40:50Z

Criterion 1 says FirstRun's finish offers the tour. Since the review fixes, the home page offers it until answered, which covers that case and more; declining is still remembered.

## Summary

web/src/tour.tsx: six steps on data-tour markers (home-tabs, open-url, settings; change-tabs, review-tab, submit). The home page offers the tour (TourOffer) until the person answers, whether or not first run ran; the change part waits for a change the person opens. A step whose element is absent or inside [hidden] is skipped, and the tour finishes when nothing after it can show. Keys pressed in the popover, a hint, or the offer stop there, so Enter never reaches the inbox's open-change key. Esc and End the tour set it off, except that an open modal takes Esc first; Finish sets it done. Settings > Getting started > Replay the tour returns to the home page and starts from the first step. tour.test.tsx walks both parts through the real App and ChangeView and asserts the only POST is /api/prefs/tour, and covers Enter, Esc with a modal, the offer without first run, and replay from a change; each was checked to fail without its fix. Two vuoro review rounds drove the key, modal, offer and replay fixes.
