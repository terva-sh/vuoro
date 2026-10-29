---
schema: 3
id: TKT-01M647F9BJR6N627ZPABGW3VTD
title: Let a headless review follow an earlier run
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - review
assignees: []
milestone: v1.0.0
parent: TKT-01M3YMSTYMFRDFXD4QSVWHMM8P
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-29T16:21:44Z
updated_at: 2026-10-29T16:51:26Z
created_by:
  id: agent:claude-code/review-rounds
  name: ""
updated_by:
  id: agent:claude-code/review-rounds
  name: ""
extensions: {}
---

## Description

`vuoro review` starts every run from nothing. The vuoro-review skill runs it
once per round, so each round re-reviews the whole range with no idea what the
last round raised, and the reviewers re-derive findings that were already
fixed or set aside. In the getting-started self-review on 2026-10-29, three
rounds of the same eleven commits cost about $9.50 each.

The window already solves this with "Review again" (D50,
TKT-01M4BVRFX). `Manager.Follow` gives the reviewers the earlier findings
with where each stands, and links the two runs. The headless command should
reach the same path:

- `vuoro review -follows <run id>` starts the run with `Manager.Follow`. The
  id is the `run.id` of an earlier report. A run that does not exist, is not
  a review, or belongs to another change is refused with exit status 2 and
  the reason.
- The JSON report names the run it follows, and each finding that restates
  an earlier one names it, so a script can tell new findings from old ones.
- The Markdown report says which run it follows and marks restated findings.

Reviewing only the commits since the earlier head is
TKT-01M3YMSV82VVCF6E0CBDKPCQK0 (Re-anchor drafts, and review only the new
commits), and is out of scope here. This ticket gives that work its
headless entry point.

## Acceptance criteria

- [x] vuoro review -follows <run id> starts the run with Manager.Follow, so reviewers get the earlier findings with where each stands
- [x] An unknown run, a run that is not a review, or a run of another change is refused with exit status 2 and the reason
- [x] The JSON report names the run it follows, and each restated finding names the earlier finding
- [x] The Markdown report names the run it follows and marks restated findings
- [x] Usage text and plan section 6.7 describe -follows

## Implementation plan

Validate -follows in review_cmd.go before any session starts: LoadFinished reads the record another process wrote, then Get, and refuse a missing run, a non-review mode, or another Kind/Ref as a usage error. Start the run with Manager.Follow. Add follows and earlier to the -json envelope; in the Markdown report say how many earlier findings were raised again and mark each restated finding, through an extra-bullets parameter on writeFinding. Plan section 6.7.

## Notes

**agent:claude-code/review-rounds** at 2026-10-29T16:28:28Z

Not done here: the vuoro-review skill's vuoro-round script (in the dotfiles repo, outside this store) still starts every round fresh. Once a vuoro with -follows is installed, it should pass the previous round's run.id. A local range must come from the same checkout path, since a local run's ref is named for that path.

**agent:claude-code/review-rounds** at 2026-10-29T16:45:55Z

vuoro review round 1 (run 20261029-164009-31d7cf): -follows used LoadFinished, so the deferred Shutdown re-saved every finished run from a stale copy, which could undo a window's decisions (high). Now LoadRun reads only the followed run; TestReviewFollowsAnEarlierRun checks that a bystander record is left byte-for-byte, and fails with LoadFinished back. The Markdown restated line now says where the earlier finding stood (medium), and plan 6.7 names the JSON fields (low).

**agent:claude-code/review-rounds** at 2026-10-29T16:51:26Z

vuoro review round 2 (run 20261029-164606-3d95ef): Shutdown still re-saved the followed run itself from its start-up copy (high), the sibling of round 1's finding that the sweep missed. The command now Forgets that run once Follow has written the forward link; TestReviewFollowsAnEarlierRun edits the record mid-run as a window would and checks the edit survives. Also: the raised-again count counts distinct earlier findings, and the refusal says change or checkout. Plan 6.7 no longer claims more than the code does.

## Summary

vuoro review -follows <run id> starts the run with Manager.Follow, so the reviewers get the earlier run's findings with where each stands, and the earlier run links forward. A missing run, a non-review, or a run of another change or checkout is a usage error (exit 2) and starts nothing. The JSON envelope gains run.follows and earlier; the Markdown report says how many earlier findings were raised again and marks each one. Tested in TestReviewFollowsAnEarlierRun and TestReviewRefusesARunItCannotFollow, and each check was confirmed to fail those tests when removed. Plan 6.7 describes it.
