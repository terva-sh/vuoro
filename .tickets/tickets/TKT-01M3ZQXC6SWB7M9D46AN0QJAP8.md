---
schema: 3
id: TKT-01M3ZQXC6SWB7M9D46AN0QJAP8
title: Discover open Forgejo changes for the shared inbox
type: task
status: in-progress
status_reason: null
priority: high
due_on: null
labels:
  - forge
  - forgejo
  - inbox
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies:
  - TKT-01M3ZQXC5CPRDM3CD937C6PC3Q
  - TKT-01M3ZKGRZBBS869259MBA818NQ
blocks_on: none
references: []
claim:
  actor: agent:codex/inbox-20261002
  branch: tickets/forgejo-discovery
  worktree: /home/sothr/workspace/git.local.sothr.com/terva-sh/vuoro
  commit: f5fd01bf194c223f7e2a96a876d197a9651a3b6d
  session: null
  claimed_at: 2026-10-03T03:48:37Z
  expires_at: null
archive: null
created_at: 2026-10-03T02:01:28Z
updated_at: 2026-10-03T03:48:39Z
created_by:
  id: agent:codex/windows-roadmap-20261002
  name: ""
updated_by:
  id: agent:codex/inbox-20261002
  name: ""
extensions: {}
---

## Description

D34 brings Forgejo discovery into the MVP so the next inbox includes GitLab and Forgejo together. The open-by-URL reader and PR checkout already landed in PR #3 under TKT-01M3ZKGRZBBS869259MBA818NQ (Open Forgejo PRs through the shared forge boundary). Implement only the remaining read-only discovery slice here.

Discover open PRs where the configured person is the author, requested reviewer or assignee; deduplicate roles and feed normalized current-head reviews, review requests, real pushes and threads to the shared refresh service and turn model. Honor server-clamped pagination, cancellation, limits and host/token isolation. Preserve fork PR refs and advertised SSH ports through the existing reader.

Compare one configured account’s results to the Forgejo web/API view in a read-only live check, with scrubbed fixture coverage. Bot identity classification must precede turn derivation. Posting remains in the separate Forgejo write ticket. GitLab live-write validation is not a dependency of this read-only task.

## Acceptance criteria

- [ ] Open authored, requested-reviewer and assigned PRs are discovered and deduplicated with all account roles retained.
- [ ] Offline fixtures cover pagination, review re-requests, stale/dismissed reviews, real push times, threads and bot classification.
- [ ] A read-only live account comparison matches the Forgejo view; checkout retains PR refs and SSH ports.
- [ ] Discovery uses the shared refresh contract, has no Writer requirement and does not depend on GitLab live-write validation.

## Implementation plan

Use authenticated global issue search with state=open and type=pulls for created, review_requested and assigned relationships, verifying /user matches the configured person. Page through empty responses with bounds and cancellation, validate same-host PR/repository identities, and deduplicate all roles. Also inspect reviewed candidates: retain requested-reviewer membership only with a surviving recorded request relationship, never because someone volunteered a review. Normalize REQUEST_REVIEW records and removed-assignee request timeline entries; keep historical verdicts, but a later stale/dismissed non-comment verdict must not resurrect an old current verdict. Add the optional patch-free FactsReader, excluding pending private drafts and retaining checkout PR refs/SSH ports. Unknown signed-in merge permission stays nil because repository push/admin access is insufficient evidence. Offline fixtures cover full role membership, pagination/failure/isolation, historical requested vs voluntary/removed reviewers, turn rules and bot classification. Compare the already authenticated Forgejo account read-only through tea's transport without extracting credentials, then run full local and platform CI plus PR review before merging.
