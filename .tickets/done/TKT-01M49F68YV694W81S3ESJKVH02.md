---
schema: 3
id: TKT-01M49F68YV694W81S3ESJKVH02
title: "Local projects in the window: a Local tab, one row per branch"
type: epic
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - ui
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
created_at: 2026-10-06T20:41:26Z
updated_at: 2026-10-06T22:28:29Z
created_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
updated_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
extensions: {}
---

## Description

D47 and docs/plan.md section 6.8. The home page gains a Local tab beside the forge inbox: projects the person adds or vuoro finds under scan roots, one row per branch with commits beyond the base, worktrees as tags, and a Review that runs section 6.7's local range in the window. TKT-01M3YMSV6BABG7EW1NP8J6W5SR (Review a local branch or range with no forge) keeps carrying the findings into the merge request; this epic covers finding and starting the work.

## Summary

The Local tab is built.

- Projects come from folders the person adds, through the native chooser or a typed path, and from roots vuoro scans (TKT-01M49F697, TKT-01M49F69Y).
- The home page has Reviews and Local tabs with needs-you counts, and remembers the last one (TKT-01M49F69K).
- A branch with no commit in 14 days is quiet (D48, TKT-01M49H46P).
- A branch is tied to its open change from the inbox cache (D49, TKT-01M49KHWH).
- Each branch row offers "Review local commits", and "Review the change" when tied. A local review runs in the window, leaves the checkout untouched, and finished CLI runs show on Refresh (TKT-01M49F69D, TKT-01M49F69S).
