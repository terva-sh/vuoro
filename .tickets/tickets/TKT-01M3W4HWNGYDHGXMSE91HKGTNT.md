---
schema: 3
id: TKT-01M3W4HWNGYDHGXMSE91HKGTNT
title: "GitLab adapter: discover the changes I review and wrote"
type: task
status: blocked
status_reason: "Personal-account requested-review comparison needs working local glab authentication. Read-only implementation is merged in PR #8 and all offline/platform gates pass."
priority: high
due_on: null
labels:
  - forge
  - gitlab
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies:
  - TKT-01M3W4HWJZ1013S2MPZ69B72AW
  - TKT-01M3W4HWM6V2F20KNT03BSBNZY
  - TKT-01M3YMSSPF763ZMG6P9FAWQYXK
  - TKT-01M3ZQXC5CPRDM3CD937C6PC3Q
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-01T16:25:25Z
updated_at: 2026-10-03T03:47:45Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:codex/inbox-20261002
  name: ""
extensions: {}
---

## Description

Implement the `forge` interface for gitlab.com, read-only: discover changes
where the person is a requested reviewer, an author, or an assignee, and read
each one's diff refs, approvals, discussions, and the person's draft notes.

docs/plan.md section 5 lists the endpoints. Respect rate limits, and fetch
incrementally with `updated_after` once the first full pull is cached.
The adapter must work against any GitLab host, not only gitlab.com.

## Acceptance criteria

- [x] Adapter tests run from recorded fixtures with no network
- [ ] A live run lists the same open changes the GitLab review-requested page shows
- [x] No token appears in any log line or error message, and a test proves it

## Implementation plan

Discover complete open membership through authenticated REST author/reviewer/assignee queries, paginated within the configured host; verify the signed-in username, deduplicate refs/roles, and reject partial pages. Delta-only updated_after discovery was rejected because it cannot safely remove closed changes or removed relationships. Add an optional neutral FactsReader for patch-free inbox reads on each refresh: permissions and mergeability can change without a membership update stamp. Keep Open for diffs and personal drafts when entering a workspace. Harden GraphQL discussion pagination and reject truncated nested factual connections; normalize system review events across mixed threads in chronological order with bot marks and explicit signed-in merge permission. Cover pagination, failures, redirects, cross-host identities, credentials and refresh with offline fixtures. Compare read-only discovery with the owner's GitLab requested-review list once local glab authentication is ready. Record unavailable live evidence honestly.

## Notes

**agent:codex/inbox-20261002** at 2026-10-03T03:40:34Z

Implemented complete role discovery and fresh patch-free facts. Full just ci passes with CGO_ENABLED=0, as does an opt-in anonymous live read of gitlab-org/cli !3991 confirming GraphQL schema compatibility. Offline regressions cover server-clamped/missing-header pagination, late failures, account/host isolation, rate limits, malformed cursors, nested truncation, moving snapshots, mixed system events and token truncation-prefix redaction. Personal-account live comparison remains unverified because glab's /user read cannot authenticate; acceptance criterion 2 remains unchecked. Added an opt-in comparison using glab's authenticated HTTP responses rather than extracting credentials: VUORO_LIVE_GITLAB_DISCOVERY=1 plus username/optional host. No auth file was read. Nested factual connections above 100 entries fail visibly pending separate exhaustive nested pagination support; partial facts never replace the cache.

**agent:codex/inbox-20261002** at 2026-10-03T03:42:23Z

PR #8's first terva-review was clean on fe6ea043ddfba1fd86338807a0c79ce59226af85. A subsequent self-review added explicit MR IID validation and fixed malformed GraphQL timestamps so decoder errors cannot echo credential values. Regression tests cover GraphQL error messages, invalid timestamps and mismatched identities. These substantive hardening changes will receive a fresh review. Personal-account live comparison is still pending local authentication.

**agent:codex/inbox-20261002** at 2026-10-03T03:47:44Z

in-progress to blocked: Personal-account requested-review comparison needs working local glab authentication. Read-only implementation is merged in PR #8 and all offline/platform gates pass.

## Summary

Read-only GitLab discovery and fresh patch-free facts landed in PR #8 after two clean full reviews and passing exact-head internal CI plus Linux/macOS/Windows native checks, including the macOS window and race checks. Final reviewed head: 0abfba143744674a4a7efc3533a3bdabd8850a07. Full local just ci and anonymous live schema/read validation passed. Personal-account discovery parity remains the sole unfinished acceptance criterion and needs the owner's local glab authentication. No credential was extracted or auth file read.
