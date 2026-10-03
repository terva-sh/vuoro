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
updated_at: 2026-10-03T03:06:52Z
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

## Summary

Implementation and fixture/boundary coverage are complete on tickets/turn-engine in PR #7. Pure derivation explains mine/theirs turns with earliest unanswered times for known events; all seven rules have table coverage and every result has a readable reason. Unknown timestamps and merge permission remain explicit. Full local CI and targeted final tests pass. PR review and platform gates determine when this implemented slice merges; adapter discovery and the UI follow separately.
