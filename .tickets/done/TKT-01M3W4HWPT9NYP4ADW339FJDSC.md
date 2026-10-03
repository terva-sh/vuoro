---
schema: 3
id: TKT-01M3W4HWPT9NYP4ADW339FJDSC
title: Derive the turn for each change, with its reasons
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - inbox
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies:
  - TKT-01M3W4HWJZ1013S2MPZ69B72AW
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-01T16:25:25Z
updated_at: 2026-10-03T03:25:35Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:codex/inbox-20261002
  name: ""
extensions: {}
---

## Description

The `turn` package: pure functions from the decided model to `mine` or
`theirs`, a `since` time, and the reasons. No I/O. The reasons are the product
(docs/plan.md section 2.1), so each is a value a person can read, such as
"Mason replied on 2 threads", not a code.

## Acceptance criteria

- [x] Every turn rule in docs/plan.md section 4 has a table test, built from recorded fixtures
- [x] Every result carries at least one human-readable reason, and a test proves it
- [x] The package imports no forge adapter, server, or network code

## Implementation plan

Implement turn.Derive as a pure function over classified forge.Change, username and discovery roles. Return mine/theirs, readable structured reasons, earliest unanswered since, and last activity. Cover R1/R2/R2b/R3 and A1/A2/A3; use current reviewer state plus dated review/request/push facts, ignore bot/system activity, and keep unknown timestamps explicit rather than inventing ages. Add nullable authenticated-person merge permission to the neutral model; A3 requires true permission and no effective change request. Generate scrubbed normalized turn fixtures from existing recorded GitLab adapter fixtures, then test rule variants, noise, earliest unanswered ages, missing facts, dual roles and inactive changes without adapter imports in turn. Extend boundary tests to include turn and forbid forge adapters/network/server transitively. Update the design for explicit unknown values and merge permission.

## Notes

**agent:codex/inbox-20261002** at 2026-10-03T03:00:21Z

PR #7: https://git.local.sothr.com/terva-sh/vuoro/pulls/7. Implemented pure turn.Derive with every R/A rule, structured human-readable reasons, waiting ages and nullable event times. Added nullable authenticated-person CanMerge; unknown permission never enables A3. Current reviewer state supplements dated history. Resolved threads and bot/system noise do not demand a response. A repeated change request without an author push retains the earlier unanswered age.

Normalized fixtures were generated locally from the already recorded/scrubbed public GitLab fixtures, then synthetic variants cover every rule and missing/stale/dismissed facts. turn imports no forge adapter, HTTP, app, server, agent or ticket code. just ci passed, and the final repeated-change-request refinement passed turn and boundary tests. Review and native CI are requested before merge. Real adapter population of CanMerge follows in the adapter slices; until then merge permission remains safely unknown.

**agent:codex/inbox-20261002** at 2026-10-03T03:06:52Z

PR #7 review dispositions at f2dd08f7a22e320185b7ba38c110451eec129062:
- Preserve undated current change requests: accepted. Explicit GitLab RequestedChanges state now yields A2 with unknown since when no dated verdict was recorded. Its reason states that the age is unknown rather than asserting a dated push comparison. A regression checks Mine/A2/nil age, not merely the absence of A3.
- Mixed dated/undated review requests: accepted. Request derivation tracks whether a later dated request exists separately from its age; undated matching history makes the earliest age unknown without suppressing a known re-request action. Regressions cover initial requests and re-requests mixed with undated history.
The full gate is running before a fresh review. These correct missing-fact behavior; no timestamps are invented.

**agent:codex/inbox-20261002** at 2026-10-03T03:12:21Z

PR #7 second review at e56cd67ff28c1f689bbb77363684f618e9f50eb4 confirms the first two findings resolved. New finding (repeated undated change-request age): accepted. Human verdict history now sorts only when every relevant timestamp is present; otherwise it retains adapter chronological order. Bot/comment noise is filtered before choosing that order. Each effective verdict walks its uninterrupted run back to the first unanswered verdict; an undated member makes the age unknown. Opposing verdicts, dismissals/staleness, head mismatch, or an author push answering a change request end that run. This also keeps repeated approvals from resetting age and lets a latest undated verdict supersede prior dated history without inventing a timestamp. Regression covers mixed change-request times, an opposing verdict ending the unknown run, and a latest undated approval. Running full CI and requesting a fresh review.

**agent:codex/inbox-20261002** at 2026-10-03T03:15:21Z

PR #7 third review at 7a36cb7ece8f10207ff568f921f9a9f502ab5145 confirms the repeated-undated-verdict finding resolved. New finding (R2 age with incomplete push metadata): accepted. R2 now separates proof of a dated post-review push from uncertainty about earliest age. Any undated push in history keeps age unknown once a new-push action is proven by dates or stale/head evidence; review SHA and push Head are not required to preserve that uncertainty. The regression table covers complete metadata, missing review SHA and missing push Head. No action is invented from an undated push alone.

**agent:codex/inbox-20261002** at 2026-10-03T03:18:48Z

PR #7 review at f49a8aa20ee2257774882088c1a0508bd9ca4d2e confirms the incomplete-push age finding resolved. New finding (current GitLab verdict with invalid matching history): accepted. Current state now replaces matching dated history when it is dismissed, stale, or explicitly attached to an old head, leaving the current verdict actionable with unknown age. A three-case regression proves Mine/A2/nil age for those invalid historical facts. Known valid GitLab change-request timestamps still compare against recorded pushes, so the recorded author-push handover remains intact.

**agent:codex/inbox-20261002** at 2026-10-03T03:21:22Z

PR #7 review at 4202d608c0bf7f132ad6a91dbc749b31155b6021 confirms current GitLab state reconciliation resolved. New finding (mixed thread timestamp ages): accepted. A1, R3 and waiting-thread ages now inspect all relevant unanswered human notes. Any undated relevant note makes age unknown; R3 counts only author replies, and bot/system noise is excluded before age calculation. Regression table covers author/reviewer/waiting threads with dated first notes and undated follow-ups, plus undated bot/system noise retaining the known age. This makes unknown-time handling uniform across requests, pushes, verdicts and threads.

**agent:codex/inbox-20261002** at 2026-10-03T03:25:35Z

PR #7 review at f63ef349e206688379378a82fcf6a8620e9fd7a5 confirms mixed thread ages resolved. New finding (non-GitLab current change requests without history): accepted. Current reviewer state reconciliation is now forge-neutral, including both explicit RequestedChanges and Approved states when history is missing or invalid. Unreviewed/Reviewed current states do not resurrect older verdicts, while an absent current state leaves valid history usable. A matrix covers change-request and approval fallbacks on GitLab, Forgejo and a future forge kind; every missing-history result has unknown age. This removes forge-kind knowledge from turn derivation.

## Summary

Implementation and fixture/boundary coverage are complete on tickets/turn-engine in PR #7. Pure derivation explains mine/theirs turns with earliest unanswered times for known events; all seven rules have table coverage and every result has a readable reason. Unknown timestamps and merge permission remain explicit. Full local CI and targeted final tests pass. PR review and platform gates determine when this implemented slice merges; adapter discovery and the UI follow separately.
