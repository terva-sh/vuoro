---
schema: 3
id: TKT-01M3YMSSYSBT0X4SMB712EA0PQ
title: Triage the threads on my change, and draft replies
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - agent
  - review
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies:
  - TKT-01M3YMSSRC81M0BYHK15TW9VDY
  - TKT-01M3YMSSPF763ZMG6P9FAWQYXK
  - TKT-01M3W4HWSGA04D4VQ8WHBEJC2D
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-02T15:47:50Z
updated_at: 2026-10-03T06:35:59Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:codex/gitlab-live-20261003
  name: ""
extensions: {}
---

## Description

The author's side of the MVP. On a change the person wrote, an agent reads the
open threads, groups them as must fix, discuss, or nit, and drafts replies
through the drafts MCP server. The person edits and posts. Fixing code is the
editor agent's job, at v1.0.0.

## Acceptance criteria

- [x] Open threads on my change are grouped as must fix, discuss, or nit, with the reason for each
- [x] Reply drafts appear on their threads, marked as drafted, and post only on my action

## Implementation plan

Start with a documented interaction sketch and a working Threads tab on changes authored by the configured account. Group actionable human threads into Must fix, Discuss and Nit with visible agent rationale; retain an Untriaged group while work is incomplete. Use a dedicated single reviewer-class triage session through the existing proven driver, worktree and conversation infrastructure, never an embedded agent or forge-writing MCP tool. Extend local drafts with validated thread/category/reason and a server-bound thread snapshot. Persist runs/replies and expose local edit/discard independently of forge write capability; freeze person-edited drafts against later agent revisions. The person accepts a reply into existing pending forge comments and submits through the existing confirmation. Fresh server-side identity/head/thread checks refuse stale, resolved or mismatched replies. Forgejo supports local triage/reply drafting and copy while posting remains unavailable. Show progress, cancellation, absent tools, read-only, errors and changed-snapshot states in the sketch and fixture preview. Cover role separation, injection-resistant context, full thread context, persistence, user-edited drafts, stale/identity checks, protected routes and UI interactions with meaningful tests; rebuild embedded assets and run full CGO_ENABLED=0 CI before PR/review/merge. Do not expand into usage, retention, editor or summaries tickets.

## Notes

**agent:codex/gitlab-live-20261003** at 2026-10-03T06:30:33Z

Implemented the Threads workspace and documented the interaction points in docs/author-triage.md. The configured author starts one reviewer-class triage run, sees Must fix/Discuss/Nit rationale beside full human threads, edits locally, accepts into private pending forge replies after a fresh identity/head/thread check, and publishes only through explicit Submit. Forgejo remains local edit/copy. Person edits freeze against agent revision; acceptance serializes against edits/revision/discard/duplicate writes; exact full-discussion fingerprints catch undated edits. Bot/system-only, author-only and resolved threads are excluded from new work. Previously triaged replies survive changed/resolved/missing threads for copying, with acceptance disabled. Persistence restores context, bindings and edited state.

Used the existing proven read-only driver/worktree/conversation boundary rather than introducing an embedded agent or writable tool. Kept discussion suggestions separate from new review findings: no synthesizer/verifier stage for triage, consistent with D13/D17 and plan 6.4. Classification can omit a reply instead of fabricating fixes, tests or agreement. Exact diff-thread focus and persistent split conversation remain later refinements.

Validation: full CGO_ENABLED=0 just ci passes (39 frontend tests; all Go tests/vet; six cross builds; dist/design/ticket checks). Regression coverage includes role/ownership, duplicate/retarget refusal, immutable snapshots, restart, configured bots without provider mutation, oversized overrides, changed head/thread/change/closed state, local edit/discard and explicit accept/submit boundaries, protected routes, no tools/read-only UI, polling cancellation/failure and an old in-flight poll versus a newer edit. Browser synthetic fixture verifies authored default Threads, all three categories, local save, pending acceptance, Submit confirmation/publication, resolved-thread history, Birch/Tar styling and 390px layout. Fixture-only banner removal avoids its own header overflow; production layout has no overflow. No real forge writes or agent quota used. Live GitLab posting remains separately blocked on writable test MR/configuration.

**agent:codex/gitlab-live-20261003** at 2026-10-03T06:34:14Z

PR #12 (Triage author threads and draft local replies) is open at https://git.local.sothr.com/terva-sh/vuoro/pulls/12. Initial full model review on 5ae3535 found two medium issues; both accepted and fixed:

1. The panel retained selection if a caller reused it for another change. Although the current parent also keys the workspace, the panel now owns a keyed inner workspace using forge change identity. Selection, polling, local state and mutation responses are discarded together on identity change; a displayed run must also match the current reference. A run-list response started before a new mutation cannot replace the newer run. Regression reuses the public panel for another change with the same thread/draft IDs, verifies abort of old polling, resolves a late old local-edit response, and proves subsequent edits target the new run. Another test covers a delayed initial list after explicit triage start.

2. Acceptance-time body overrides could leave persisted/copied wording different from the accepted reply. Removed that triage override behavior: different text is refused before any forge I/O and must be saved through EditReply first. Identical saved text is allowed; acceptance preserves existing edited provenance. Regression refuses unsaved wording, then saves/accepts and reloads to prove the actual forge body matches the persisted local text. Ordinary finding keep behavior is unchanged.

Full CGO_ENABLED=0 just ci passes after these fixes, now including 41 frontend tests. Rebuilt embedded assets. Source changes require a fresh full model review; native checks will run on the new head.

**agent:codex/gitlab-live-20261003** at 2026-10-03T06:35:59Z

Fresh full model review on 36aeb03 is clean: terva-review/code success, run ec55b5e4-7134-4260-ab70-425226687824 (Forgejo Actions #115, id 20335). The reviewer explicitly marks finding-1 (change-scoped state) and finding-2 (saved acceptance wording) resolved, with no new concrete bugs. Both dispositions are posted on PR #12 while open. Native Linux/macOS checks already pass this source head; Windows and Forgejo CI are finishing and remain merge prerequisites. Closing the implementation record with both earned criteria checked; the next commit changes only the ticket store and will carry this clean review. Merge remains guarded by both forges' exact-final-head gates, followed by main mirror synchronization.

## Summary

Delivered the author Threads workspace and interaction sketch in PR #12, https://git.local.sothr.com/terva-sh/vuoro/pulls/12. Authored changes default to Threads. One reviewer-class run groups full human discussions into Must fix/Discuss/Nit with rationale and optional reply drafts. Local edits preserve personal wording and freeze agent overwrites; explicit acceptance validates a fresh exact change/head/thread before creating a private pending reply, and the existing Submit confirmation publishes. Forgejo remains local draft/edit/copy. Changed/resolved/missing discussions retain saved replies without allowing acceptance. Run context, bindings, classification and personal edits survive restart.

Full CGO_ENABLED=0 just ci passes, including 41 frontend tests and six cross builds; synthetic browser flow verified local edits, private acceptance, explicit Submit, history and light/dark/narrow styling. Two initial model-review findings were fixed with regressions and posted dispositions; fresh full review of source head 36aeb03 is clean. Ticket-only closure carries that review; merge and mirror synchronization require final-head gates. Real GitLab posting remains separately unvalidated; no real forge writes or agent quota were needed for this slice. Exact thread focus and a persistent conversation split remain documented future refinements.
