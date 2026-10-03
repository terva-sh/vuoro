---
schema: 3
id: TKT-01M3ZKGRZBBS869259MBA818NQ
title: Open Forgejo PRs through the shared forge boundary
type: task
status: review
status_reason: null
priority: high
due_on: null
labels:
  - forge
  - forgejo
  - review
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-03T00:44:40Z
updated_at: 2026-10-03T01:27:21Z
created_by:
  id: agent:codex/forgejo-20261002
  name: ""
updated_by:
  id: agent:codex/forgejo-20261002
  name: ""
extensions: {}
---

## Description

The owner selected the shared forge boundary plus Forgejo open-by-URL support on 2026-10-02. This brings the first read/review slice forward from v1.0; discovery, the inbox and Forgejo writes stay in TKT-01M3W4HX4CJE62G9R124EV5PT4 (Forgejo adapter for the forge interface).

Use forge-neutral models, checkout metadata, supported actions and errors, with per-forge parsing and API details in adapters. Preserve the existing agent and ticket boundaries rather than a universal integration interface. A second real adapter will expose model mismatches more effectively than speculative abstractions. The slice does not need GitLab live-write credentials and therefore does not depend on that blocked validation.

## Acceptance criteria

- [x] GitLab and Forgejo URLs route through a shared provider boundary without adapter-specific logic in app
- [x] Forgejo metadata, diffs, reviews and discussions normalize from offline fixtures, including pagination and line anchors
- [x] Forgejo PR checkout feeds the existing review pipeline, including fork heads and explicit SSH ports
- [x] Forgejo exposes no write controls or write routes; GitLab human-only writes remain covered
- [x] A read-only live run opens and checks out a real internal Forgejo PR, and the full local CI gate passes

## Implementation plan

Keep the existing forge Ref identity stable, and add neutral provider capabilities, checkout metadata and errors. Put parsers and client factories in forge/adapters; app resolves a configured host before obtaining its credentials and otherwise reads anonymously. Preserve GitLab Writer as a separate optional interface. Forgejo reads paginated files, reviews, review comments and timeline, with a bounded diff parser and no writes. Checkout uses the target repository's PR head ref (also for forks), advertised clone URLs and the target branch. Gate the frontend's write context on supported actions; neutralize reference rendering and UI wording. Test mixed-forge routing, token/redirect isolation, fixtures, anchors, checkout and the human posting boundary; update the plan, append D33, rebuild web/dist and run just ci plus a read-only internal live check.

## Notes

**agent:codex/forgejo-20261002** at 2026-10-03T00:58:09Z

Implemented forge.Provider with optional Writer, neutral capabilities/errors/checkout metadata, and the parser/factory registry. app no longer imports GitLab API types. Existing Ref identity keys stay stable; display and persisted run kind distinguish PRs. Configuration accepts Forgejo and refuses ambiguous accounts on the same host. The Forgejo reader normalizes paginated metadata/reviews/threads/timeline; the separate diff is matched to its inventory, with incomplete/binary content marked omitted. Comments use new/old line fields rather than diff-row offsets. The final metadata read refuses head/target changes during the read. Capability-gated controls and Writer checks keep Forgejo read-only; GitLab posting remains protected by its existing human-action regression test.

Validation: all Go package tests, vet for Linux/macOS/Windows, six cross-builds, 27 frontend tests, TypeScript, generated bundle comparison, design-export check and strict ticket-store check passed through just ci with CGO_ENABLED=0. A fixture-driven fork test removed the source branch and reviewed only refs/pull/7/head; local drafts and Forgejo identity survived reload. The read-only live check opened internal vuoro PR #2, read 16 files (15 visible patches), one discussion and three push events, and checked out head 68ff96c709bf through the advertised SSH URL on port 2222. tea handled API authentication without exposing its config or credential; no remote writes or real agent review were run.

Limits: Forgejo discovery/inbox and posting remain in TKT-01M3W4HX4CJE62G9R124EV5PT4 (Forgejo adapter for the forge interface). This API has no bot marker; configured bot classification must precede correct turn derivation. Native Windows/macOS runtime checks were not run here, and the pre-existing Windows portability ticket remains separate. README, plan and owner decision D33 record the early read/review scope. Changes are on forgejo-open, awaiting the owner's commit/publication instruction under repository AGENTS.md.

**agent:codex/forgejo-20261002** at 2026-10-03T01:07:44Z

The owner authorized committing, pushing, opening the internal PR, obtaining and addressing reviews, and merging after validation. The read/review scope remains unchanged; this supersedes the earlier publication hold. Forgejo CI and terva-review will gate the PR, with the branch also pushed to GitHub for native platform checks.

**agent:codex/forgejo-20261002** at 2026-10-03T01:11:42Z

PR #3 is open at 4114fbd, linked to this thread, and pushed to both remotes for hosted checks. Linux and macOS passed (including the native window test). Windows run 37084894940 found one new fixture failure: core.autocrlf rewrote the recorded Forgejo HTTP diff to CRLF. Add a targeted -text attribute for recorded .diff fixtures, preserving API response bytes rather than normalizing production diffs (which could erase meaningful carriage returns in source content). The other four failures match the existing Windows portability ticket TKT-01M3ZHY78HXTVPBC8J1QY77BER (Make the Windows CI tests portable across modes and newlines). Forgejo CI run 16 and terva-review run 17 remain queued; the dispatch did create the run despite tea reporting an empty JSON response.

**agent:codex/forgejo-20261002** at 2026-10-03T01:18:39Z

Addressed both medium findings from terva-review run 17 (review 2017):

- Preserve outstanding review requests: reconcile currently requested reviewers against the latest timeline request and qualifying current-head review. A same-SHA re-request stays Unreviewed while the historical approval remains in Reviews; a review after the request answers it. Missing timestamps conservatively preserve the explicit request, removed requests do not resurrect, and stale reviews do not answer requests. Five full adapter regression cases cover these paths, including unordered request events. The synthetic thread fixture now supplies its earlier request timestamp.
- Retain existing attributes: restored both ticket Markdown rules (LF and merge=gitticket) and the Go LF rule, then appended only the targeted -text diff fixture exception. The earlier attribute overwrite was an implementation mistake.

Targeted Forgejo tests pass with CGO_ENABLED=0. Full local CI and fresh hosted CI/review are next. The four baseline Windows portability failures remain under TKT-01M3ZHY78HXTVPBC8J1QY77BER (Make the Windows CI tests portable across modes and newlines); this change addresses the fixture and formatting regressions introduced here.

**agent:codex/forgejo-20261002** at 2026-10-03T01:27:21Z

Fresh terva-review run 21 (review 2019) confirms both original findings resolved, and identifies a further medium issue: excluded reviews could erase a prior qualifying verdict. Addressed by leaving accumulated state unchanged for dismissed, stale or different-head reviews; they remain in historical Reviews. A reviewer without a valid review is already initialized as Reviewed or Unreviewed, so no extra state assignment is necessary. Six full adapter regression cases cover valid approvals and change requests followed by each kind of excluded comment. Targeted Forgejo tests pass.

Hosted checks on 81ef9b3: internal Forgejo CI run 19 passed; GitHub Linux/macOS (including native window) passed. Windows run 37085635496 reports exactly the four baseline portability failures under TKT-01M3ZHY78HXTVPBC8J1QY77BER (Make the Windows CI tests portable across modes and newlines). Both PR-introduced Windows regressions are resolved. A fresh core.autocrlf=true checkout independently verifies LF Go/tickets and unchanged recorded diff bytes. This fix will receive another full gate and model review before merge.

## Summary

Implemented the shared forge boundary and Forgejo open-by-URL read/review slice on forgejo-open. GitLab continues through the same contracts; Forgejo opens diffs/discussions and supplies validated checkout metadata for the existing review runner, with posting unavailable. Offline, fork-checkout, human-action and UI boundary coverage plus the complete local CI gate pass. A read-only internal PR check confirms the real API and SSH checkout. Source and regenerated web/dist are ready for review; commit and publication await the owner's instruction. Discovery/inbox and Forgejo writes remain a separate draft.
