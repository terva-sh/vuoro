---
schema: 3
id: TKT-01M3W4HWR5K8TXA6S6HBVXDXX5
title: "Inbox view: every change, sorted by whose turn and how long"
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - inbox
  - ui
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies:
  - TKT-01M3W4HWGGB7RMVFAJPW2KPKY5
  - TKT-01M3W4HWNGYDHGXMSE91HKGTNT
  - TKT-01M3W4HWPT9NYP4ADW339FJDSC
  - TKT-01M3ZQXC5CPRDM3CD937C6PC3Q
  - TKT-01M3ZQXC6SWB7M9D46AN0QJAP8
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-01T16:25:25Z
updated_at: 2026-10-03T04:15:54Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:codex/inbox-20261002
  name: ""
extensions: {}
---

## Description

D34 selects one inbox for GitLab and Forgejo, ordered by whose turn it is. Integrate the shared discovery/refresh service, both account adapters and the pure turn engine into the main window.

Show two groups, my turn and their turn, each sorted by the earliest unanswered event, oldest first. Each row shows forge/project, title, account role, readable reason and age, and opens the existing change/review workspace. Retain a link to the forge. Present per-account refresh errors and last-success/stale information; a failing account must not masquerade as an empty inbox or erase another account’s results.

Refresh manually and on a bounded interval only while the window is open. Keep local snoozes separate from derived cache state; a snoozed change returns at its date or when a new forge event arrives. The inbox builds and works with all agent tools absent and no ticket store. Human posting refreshes forge facts and derived turns; it never guesses posted state.

## Acceptance criteria

- [x] Opening a row opens the change view, and each row also links to the forge URL
- [x] A snoozed change returns when its date passes or a new event arrives
- [x] Changes from configured GitLab and Forgejo accounts share both turn groups with correct reason, age, account roles and oldest-first ordering.
- [x] One account failing or being rate-limited preserves other results and shows visible stale/error state.
- [x] The inbox works with no agent tools and no ticket store; refresh stays bounded to the open window.

## Implementation plan

Integrate the merged shared service, pure turn engine and both adapters via a light display projection containing account status and mine/theirs/snoozed rows, ordered by oldest known unanswered age with unknown ages last and stable Ref ties. Cached raw facts classify bots before deriving turns; projection omits patches, drafts and tokens. Record a raw event fingerprint with authored snoozes so expiry or new forge facts restores a row without config-only bot changes waking it. Add protected GET snapshot, POST refresh and POST snooze routes, initialized independently of tool probing or ticket stores. Refresh only while the home inbox is mounted, at most one frontend request at once, a minute after the previous request completes, with AbortController cleanup. Preserve stale account facts and visible last-success/errors. Rows open the existing workspace and link to the forge; a collapsed snoozed list restores work. After human writes, perform a serialized targeted fresh fact read so an older in-flight refresh cannot overwrite newer facts, preserving whole-account last-success semantics. Mixed-forge fixture, ordering/snooze/error, cancellation/polling, guard and human-write regressions plus light/dark browser validation precede full CI, review and merge. GitLab's implementation is merged; its personal-account live criterion remains blocked on authentication, so proceed with independent offline integration while preserving that blocker in the roadmap.

## Notes

**agent:codex/inbox-20261002** at 2026-10-03T04:12:26Z

Implemented the guarded lightweight combined inbox, oldest-known unanswered ordering with unknown age last, per-account stale status, serialized mounted-only refresh, authored snooze fingerprints and targeted fresh facts after human writes. Local just ci passed with CGO_ENABLED=0: all backend tests, six cross builds, three-platform vet, typecheck, 31 frontend tests, generated assets/design and strict ticket checks. Regression coverage includes no tools/tickets/runs, protected route rejection, stale mixed results, new undated events, bot configuration stability, cancellation and save failure rollback.

Collaborative browser verified the actual embedded fixture app in dark 1280x800 and light 390x844: mixed reasons/roles/age, unknown age, account rate-limit warning, snooze count 3 to 2 and restore to 3, row opening the existing read-only workspace, and no horizontal overflow at 390px. Browser loopback reached a different host, so a temporary scratch proxy forwarded only synthetic fixture traffic to the guarded local app; no production protection was altered and no real forge credentials were used. GitLab personal-account discovery parity remains unverified and blocked on CLI authentication under its own ticket.

**agent:codex/inbox-20261002** at 2026-10-03T04:14:16Z

PR #10 first review (9acf4341-c760-43e6-bc85-cd5a4f44e9f0, head 63f1bbd) raised one medium finding: refresh all same-host account memberships. The requested multi-account behavior is declined because config.Parse already rejects multiple accounts on a host (config/config.go) and app credential routing intentionally chooses one host account; README documents this supported constraint. Instead hardened inbox.New to enforce the same case-insensitive host uniqueness for direct service callers, with a regression test. This resolves the bypass at the public service boundary without adding ambiguous credential routing or unsupported identity semantics. Request a full review of the changed source.

**agent:codex/inbox-20261002** at 2026-10-03T04:15:54Z

Full review of source head 4a8917e32536c2c7f1731c70954c59d72c85113f is clean: run 25d791b1-f601-4e72-a6c9-92cad13315ce, Actions 20215. The reviewer confirms the same-host finding resolved by the constructor invariant and its regression. Full local just ci passed again after this change. Internal and Linux/macOS/Windows CI for the final ticket-only head must pass before guarded merge; request review carry for that head. No further implementation changes are required. GitLab CLI user check still cannot authenticate; do not claim personal-account live evidence.

## Summary

Implemented in Forgejo PR #10: https://git.local.sothr.com/terva-sh/vuoro/pulls/10. Combined GitLab/Forgejo turn groups, oldest-known ages/reasons/roles, workspace opening and forge links, visible per-account stale status, mounted-only serialized polling/cancellation, authored snooze restoration and serialized fresh turn updates after human writes. Independent of agent tools and ticket stores; protected window routes preserved. Full local CI and browser fixture validation passed; clean full source review run 25d791b1-f601-4e72-a6c9-92cad13315ce. Merge remains gated on exact final head CI/review status. Separate GitLab discovery ticket retains its blocked personal-account live comparison; the implementation does not assert that comparison passed.
