---
schema: 3
id: TKT-01M3ZHY78HXTVPBC8J1QY77BER
title: Make the Windows CI tests portable across modes and newlines
type: bug
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - ci
  - desktop
assignees: []
milestone: v0.1.0
parent: null
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-03T00:17:04Z
updated_at: 2026-10-03T02:06:38Z
created_by:
  id: agent:codex/vuoro-design-20261002
  name: ""
updated_by:
  id: agent:codex/windows-roadmap-20261002
  name: ""
extensions: {}
---

## Description

The GitHub mirror's first styling-branch run passed Linux and macOS (including the native-window test) but failed four existing Go tests on Windows. The failing test files are unchanged between the pre-styling base 6cd5940 and merged styling c874f63.

Evidence: https://github.com/terva-sh/vuoro/actions/runs/37080789715/job/111080918401, head 68ff96c709bfbd9d96709b4b976bf4c77f131148. Related work: TKT-01M3W4HWHRKR936RT5MK8ZHB2B (Load the terva-sh design stylesheet, with the export check in CI), merged as vuoro PR #2 (Adopt the shared violet styling).

### Failures

- agent/claude TestATwoTurnConversationStreams: the MCP configuration test expects Unix mode 0600, while Windows reports -rw-rw-rw-. Preserve the credential-isolation assertions and use meaningful platform-appropriate permission coverage.
- config TestTheDocumentedExampleParses: a verbatim LF string is searched in docs/plan.md, which the Windows checkout gives CRLF line endings. Compare the documented content without depending on the checkout's newline convention.
- forge/patch TestAgreesWithGit: git apply inherits the runner's core.autocrlf setting and produces CRLF, but the expected fixture uses LF. Make the fixture's Git behavior deterministic without modifying a user's Git configuration.
- review TestTheReportIsWrittenInTheRun: the test checks a path suffix built with '/' against a Windows path with backslashes. Assert the path with platform-aware construction while retaining the report-content check.

This is separate portability work discovered by the required hosted checks. The shared design module downloaded and its export check passed on Linux and macOS; the Windows job stopped at go test before reaching the export step.

## Acceptance criteria

- [x] The four named tests pass on Windows with CGO_ENABLED=0 and preserve their original behavioral assertions.
- [x] Linux and macOS CI still pass, including the native-window check on macOS.
- [x] Fixtures and documentation checks do not depend on inherited Git newline settings or Unix-only path conventions.

## Implementation plan

Fix the four portability assumptions without changing production behavior: assert the MCP file is regular and created in the configured directory on every OS, retain Unix 0600 checks where meaningful and retain token/argv and cleanup assertions; normalize documentation CRLF only for comparison; override newline configuration on the fixture git apply command; construct report paths with filepath.Join. Run targeted package tests under inherited core.autocrlf=true/core.eol=crlf, then the complete cgo-free gate and hosted Linux/macOS/Windows CI. Native Windows validation is required before ticking its acceptance criteria. After fixes, audit the design and backlog and prepare a sequenced roadmap proposal with dependencies, completion gates and owner choices; keep unsettled priorities out of the decision log.

## Notes

**agent:codex/windows-roadmap-20261002** at 2026-10-03T01:52:19Z

Implemented the four test portability fixes without changing production code. The Claude MCP test keeps argv token isolation and removal-on-close assertions on all platforms, additionally requires a regular file in the configured directory, and checks Unix 0600 only where os.Stat reports Unix permissions; it does not claim to verify Windows ACLs. The docs example compares CRLF-normalized text, git apply fixture commands set core.autocrlf=false/core.eol=lf, and the report test checks the exact filepath.Join path under the run state directory.

Targeted package tests pass. The patch test also passes uncached while inheriting core.autocrlf=true and core.eol=crlf through command-scoped environment settings, with no user Git configuration changes. The full local just ci gate passes with CGO_ENABLED=0: all package tests, three-platform vet, six cross-builds, 27 frontend tests, TypeScript, bundle/design checks and strict tickets. Native Windows and macOS hosted validation remain required before checking criteria 1 and 2. Continuing the owner-authorized commit/push/PR/review/merge workflow for these fixes.

**agent:codex/windows-roadmap-20261002** at 2026-10-03T01:58:48Z

GitHub run 37087736239 passes all Go tests on Windows, including the four original failures; Linux/macOS pass including the native window check. Windows then reaches the previously unreachable design check and fails because core.autocrlf rewrites the generated stylesheet to CRLF. Preserve LF for the exported design CSS and exact bytes for all generated web/dist assets, retaining the pre-existing ticket/Go/diff attributes.

The bundle checker also invokes npx through execFileSync, which is a .cmd shim requiring a shell on Windows. Run the locked local Vite JS CLI with process.execPath instead, preserving the no-shell argument boundary and exact output comparison. These changes finish the existing cross-platform gate rather than weakening comparisons or regenerating different content per OS. A fresh checkout with core.autocrlf=true will exercise generated-asset handling.

The first model review is clean (run 31, 4af834c1-4ab4-4185-b1db-4c5e2bc84d26). Request another review for these substantive checker/attribute changes. Original-test Windows coverage is earned, but full multi-platform CI acceptance remains unticked until the downstream gates pass.

**agent:codex/windows-roadmap-20261002** at 2026-10-03T01:59:58Z

A scratch core.autocrlf=true checkout reproduced a second generated-asset mismatch: Vite preserves the HTML entry point’s CRLF in generated index.html. Added a targeted LF attribute for web/index.html as a build input, alongside the exported CSS and exact-byte web/dist rules. Rechecked a freshly materialized HTML entry point with the new attributes: both verify:dist and design-check pass in that checkout. The original ticket, Go and recorded-diff rules remain intact. The full local gate also passes after the portable Node/Vite invocation. No generated asset content or production application code changed.

**agent:codex/windows-roadmap-20261002** at 2026-10-03T02:06:37Z

Native GitHub run 37088207840 passes Windows, Linux and macOS completely on d5d3471, including the macOS GUI check and Windows vet, formatting, all Go tests, six cross-builds, design export, frontend typecheck/27 unit tests/bundle comparison and strict tickets. All three acceptance criteria are now earned. The additional generated-byte and Node/Vite fixes removed downstream gate failures exposed once the original four tests passed. No production application behavior changed and no validation was disabled.

Close the implementation record before the final review/CI merge gate. The fresh full review request windows-portability-complete-1 is queued and will read the current PR head; if it has already read the earlier source head, carry its passing verdict over these ticket-only changes. Merge remains contingent on final-head internal CI and clean review.

## Summary

Fixed in Forgejo PR #4: https://git.local.sothr.com/terva-sh/vuoro/pulls/4. Portable tests retain MCP token isolation/cleanup and Unix permission assertions, compare docs independent of CRLF, set exact fixture Git newline behavior and check report paths with filepath.Join. Generated assets retain their bytes, and the bundle verifier launches locked Vite through Node without a platform-specific shim. Full local CI and the complete native Windows/Linux/macOS gate pass; review and internal CI gate the authorized merge.
