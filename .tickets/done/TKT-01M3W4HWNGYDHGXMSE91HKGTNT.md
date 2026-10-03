---
schema: 3
id: TKT-01M3W4HWNGYDHGXMSE91HKGTNT
title: "GitLab adapter: discover the changes I review and wrote"
type: task
status: done
status_reason: null
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
updated_at: 2026-10-03T05:21:46Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:codex/gitlab-live-20261003
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
- [x] A live run lists the same open changes the GitLab review-requested page shows
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

**agent:codex/gitlab-live-20261003** at 2026-10-03T05:21:45Z

Completed the remaining personal-account comparison on 2026-10-03 at source main d5b43828be7c0e7de4f6c94bf087a931b88193a9. Diagnosed the owner's glab login HTTP 429 temporary-service response separately from missing Secret Service. The current default glab /user request returned 401 while an anonymous public-project API request returned 200. A read-only token supplied by the owner authenticated as WarrickSothr when passed through a non-echoing stdin prompt into process memory and the subprocess environment. No auth file was read and no credential was written to a file, config, argv, fixture, ticket or command output.

CGO_ENABLED=0 GOTOOLCHAIN=local mise exec -- go test -v ./forge/gitlab -run '^TestLiveDiscovery$' -count=1 passed in 2.01s. Vuoro's discovery and an independent paginated GitLab requested-review list both returned zero requested reviews; authored/reviewer/assigned discovery found zero open memberships. This proves authenticated empty-inbox parity for this account, not authenticated nonempty facts, pagination or turn transitions. Positive facts/pagination/turn coverage remains the existing offline fixtures, anonymous live GitLab read and separate nonempty Forgejo live comparison. Mark criterion 2 earned for the observed empty list, clear only this ticket's authentication blocker, and keep the separate GitLab posting validation blocked.

## Summary

Read-only GitLab discovery and fresh patch-free facts landed in PR #8 after two clean full reviews and passing exact-head internal CI plus Linux/macOS/Windows native checks, including macOS window and race checks. Final reviewed implementation head: 0abfba143744674a4a7efc3533a3bdabd8850a07. Full local CI and anonymous live schema/read validation passed.

Remaining personal-account parity criterion passed on 2026-10-03 at main d5b43828be7c0e7de4f6c94bf087a931b88193a9: authenticated as WarrickSothr, both discovery and the independent requested-review query found zero requested reviews; all three role queries found zero open memberships. The live test passed in 2.01s. This is empty-inbox evidence only; authenticated nonempty turn facts and live pagination were not exercised. The supplied token was used only in memory/environment via non-echoing stdin; no auth file was read or credential persisted. GitLab posting remains a separate blocked validation ticket.
