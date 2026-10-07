---
schema: 3
id: TKT-01M4BVRFXPS5PWP5TRP97XGBMC
title: Review a change again carrying the earlier review's findings
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - review
  - agent
  - ui
assignees: []
milestone: null
parent: TKT-01M4BVRFAY1DN7317TACJXXXRA
origin: null
dependencies:
  - TKT-01M4BVRFD32Q0HM8X6NTTZSWSD
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-07T18:59:35Z
updated_at: 2026-10-07T20:24:28Z
created_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
updated_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
extensions: {}
---

## Description

D50, fourth bullet. "Review again" on a moved change starts a run that follows the earlier one (a recorded follows id, linked both ways).

- Reviewers' first message lists the earlier findings with where each stands: fixed (with commit), dismissed, or open.
- propose_draft gains an optional earlier id; a draft that restates an earlier finding names it and comes in with that finding's decision, so a dismissed one arrives dismissed with the earlier dismissal shown.
- The run page shows the run it follows and an "Earlier findings" folded group.

## Acceptance criteria

- [x] Review again starts a run linked to the earlier one, both ways
- [x] Reviewers are given the earlier findings with where each stands
- [x] A draft restating an earlier finding keeps the person's decision on it
- [x] The run page shows the earlier findings in a folded group

## Implementation plan

A run that follows an earlier one (D50, fourth bullet).

- drafts: an `Earlier` finding (id, severity, title, anchor, standing, fix, decision, note) held by the Store with SetEarlier. A Proposal gains an optional `earlier` id, which must name one of them. A reviewer's or synthesizer's draft that names one (a synthesizer's inherits it from its sources) comes in with that finding's decision and note, marked `carried`. The MCP schema offers `earlier` to reviewers and synthesizers.
- review: `Manager.Follow(ch, src, reviewers, synth, verifier, prevID)` checks that the earlier run is a review of the same change. It snapshots the earlier run's shown findings, adds them to the first message with where each stands, and links the runs both ways (`follows`, `followed_by`). Both links and `earlier` are persisted in the record.
- app: POST /api/runs takes `follows`.
- web: the run panel offers "Review again" when the selected run's head is not the change's head. The run page names the run it follows and the runs that follow it, folds "Earlier findings · N", and a draft card says which earlier finding it restates and whether its decision was carried.

## Notes

**agent:claude/frontend-modules-2026-10-06** at 2026-10-07T20:24:28Z

Supersedes the summary's line about the Local tab: "Review local commits" on a branch with a matched review now follows that branch's newest review. POST /api/local/review finds it with matchRuns. Follow accepts any earlier local run, since a local Ref is named for the checkout it was read from. The button's tooltip says it follows. Tested in TestABranchIsReviewedFromTheWindowAndItsCheckoutIsLeftAlone. A headless screenshot of the run page (waves, Fixed and Earlier groups, carried badges, follow links) looked right in Light.

## Summary

"Review again" on a moved change starts a run that follows the earlier one (D50). `Manager.Follow` refuses a run of another change or one that is not a review. Start is Follow with nothing to follow.

The earlier run's shown findings are snapshotted as `drafts.Earlier`, each with a standing: fixed (with its commit), dismissed, kept (a comment posted, not fixed), or open. The snapshot is appended to the first message as `<earlier-finding>` elements marked agent-written, so the reviewers, the synthesizer, and the verifier all see it.

propose_draft and revise_draft take an optional `earlier` id. An unknown id is refused with `earlier.unknown`, and triage refuses the field. A draft that names one takes that finding's decision and note, marked `carried`, and is then locked from agent changes like any decided draft. A synthesizer's merge takes the earlier id from its sources.

The runs are linked both ways (`follows`, `followed_by`), and the links and the earlier findings are persisted in run.json. POST /api/runs takes `follows`.

The run panel offers "Review again" when the shown run's head is not the change's head. The run page links the run it follows and the later review, folds "Earlier findings · N" with standing and "restated as dN" or "not raised again", and marks restated drafts "↺ earlier dN" with a carried decision shown as "earlier" / "in the earlier review".

The Local tab's "Review local commits" does not yet follow the branch's last run; that is left for later. I did not take a screenshot of the panel.
