---
schema: 3
id: TKT-01M3ZQXC6SWB7M9D46AN0QJAP8
title: Discover open Forgejo changes for the shared inbox
type: task
status: draft
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
claim: null
archive: null
created_at: 2026-10-03T02:01:28Z
updated_at: 2026-10-03T02:01:28Z
created_by:
  id: agent:codex/windows-roadmap-20261002
  name: ""
updated_by:
  id: agent:codex/windows-roadmap-20261002
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
