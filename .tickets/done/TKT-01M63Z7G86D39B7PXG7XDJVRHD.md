---
schema: 3
id: TKT-01M63Z7G86D39B7PXG7XDJVRHD
title: "Spike: a practice review on a generated local repository"
type: spike
status: done
status_reason: null
priority: low
due_on: null
labels:
  - sandbox
  - agent
  - review
assignees: []
milestone: v1.0.0
parent: TKT-01M63Z778B87DTP765W8JWSMPN
origin: null
dependencies:
  - TKT-01M63Z7G66X679ZJEHFAGK7FD1
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-29T13:57:41Z
updated_at: 2026-10-29T16:29:55Z
created_by:
  id: agent:claude-code/getting-started-tutorial
  name: ""
updated_by:
  id: agent:claude-code/getting-started
  name: ""
extensions: {}
---

## Description

The deepest way to learn the window is to run a real review. A forge change is the wrong place to practise, so this spike decides whether vuoro can generate a small git repository with a planted bug in its state directory and offer it on the Local tab as a practice review, exercising the reviewers, the synthesizer, and the drafts with no forge involved.

Questions to answer:

- Cost: what the cheapest preset costs on the practice repository, and how the offer shows that before anything starts. It must be opt-in.
- Absence: what the offer says when no agent tool is installed or signed in.
- The end: the practice stops at the drafts. Submit has nothing to post to and must not pretend to. Decide what the last step shows instead.
- Lifetime: where the repository lives, how it is removed, and that it never appears as one of the person's own projects after the practice ends.
- Whether levels 1 and 2 already make this unnecessary.

## Acceptance criteria

- [x] A written answer to each question in the description, as a note on this ticket
- [x] A recommendation to build, defer, or drop it, for the owner to decide

## Notes

**agent:claude-code/getting-started** at 2026-10-29T14:13:02Z

### Answers, from reading the code after levels 1 and 2 landed

Nothing here was measured with a real agent run. The cost figures are an estimate that a build would have to confirm.

**Cost.** The shipped `quick` preset is one `code` reviewer, plus the verifier every run gets (D31), so two sessions and no synthesizer (config.Builtin). A practice repository of a few files keeps each session's reading small, so the cost is mostly each session's fixed start: the profile prompt and the tool's own context. That is probably cents, not dollars, but it depends on the person's model and must be measured on a real run before the offer quotes a number. Showing the synthesizer, which the tutorial is meant to teach, needs `standard` (two reviewers, a synthesizer and a verifier: four sessions), so roughly twice the cost. The offer must be opt-in and name the preset, the session count, and "costs money on your <tool> account" before anything starts, the same as the change-no-runs hint says now.

**Absence.** The offer would reuse NeedsTool from web/src/guide.tsx. With no agent tool answering, it names the tools vuoro knows and points at Settings > Agent tools, and offers no start button. That is the same sentence the change-no-runs and local-empty hints give today.

**The end.** This is the real problem. A practice repository has no forge, so it runs as a local review (section 6.7, 6.8). A local review is the person's own work (D42): its findings are picked for the editor or dismissed, and there is no Keep, no pending comment and no Submit. So the practice would teach the local flow, not the review-someone-else flow that the tour and the first-drafts hint describe. To teach Keep and Submit, the practice would need a third mode of the findings view, with forge-style drafts, Keep and Edit that write nowhere, and a last step that says "on a real change, Submit posts these". Building that is the simulated submit that D70 rejected. Without it, the last step can only be the local flow's own end: the findings, and an offer to hand them to the editor, which in turn needs the editor's sandbox.

**Lifetime.** The repository would live in the state directory, under something like `practice/<id>/`, not in `local.json`, so it never appears among the person's added projects or scan roots. The run is kept under `runs/<id>/` with normal retention (section 8), and its bare clone under `repos/local/...` like any local range. Ending the practice deletes the practice directory and closes the run, which removes its worktree. The bare clone would need its own removal, because nothing removes local-range clones today. RunList would show the closed run, with no forge link, until retention removes it, unless the run is marked as practice and hidden.

**Do levels 1 and 2 make it unnecessary?** Mostly. The hints say on each empty screen what a preset is, that a run costs money, that drafts are drafts, and that only Submit posts. The tour points at each surface. The one thing they cannot give is the feel of a run in progress: the panes filling and talking to a reviewer. A person gets that on their first real change anyway, with the change-no-runs and first-drafts hints beside it, and a review of their own local branch is already a low-stakes first run that posts nothing.

### Recommendation: defer

Defer it, and do not build it for v1.0.0. The practice repository would teach the local flow, not the forge one. Making it teach Keep and Submit means simulating a submit, which D70 rejected. Its remaining value, seeing a run happen, comes from the first real run, which the hints now guide. Reopen it if people who finish the tour still stall at their first run. That would show in feedback, since vuoro has no telemetry. In that case, the smaller build is a "Try a review on your own branch" hint on the Local tab, not a generated repository. The owner decides.

**agent:claude-code/getting-started** at 2026-10-29T14:22:43Z

Owner deferred it on 2026-10-29, recorded as D71; plan 10.3 lists it after v1.0.0.

**agent:claude-code/getting-started** at 2026-10-29T16:29:55Z

Renumbered on landing: another change took D70 first (a conversation is never refused for its size). The getting-started decision is now D71 and the practice-review deferral D72. Earlier notes on this ticket that say D70 and D71 mean D71 and D72.

## Summary

Answered each question in a note. Recommendation: defer. A practice repository runs as a local review, whose findings are the person's own (D42) with no Keep or Submit, so it would teach the local flow rather than the forge one, and teaching Submit there means the simulated submit D70 rejected. Cost is unmeasured: quick is two sessions (reviewer + verifier), standard four with a synthesizer. The owner decides.
