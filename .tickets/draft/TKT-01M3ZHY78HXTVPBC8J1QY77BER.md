---
schema: 3
id: TKT-01M3ZHY78HXTVPBC8J1QY77BER
title: Make the Windows CI tests portable across modes and newlines
type: bug
status: draft
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
updated_at: 2026-10-03T00:17:04Z
created_by:
  id: agent:codex/vuoro-design-20261002
  name: ""
updated_by:
  id: agent:codex/vuoro-design-20261002
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

- [ ] The four named tests pass on Windows with CGO_ENABLED=0 and preserve their original behavioral assertions.
- [ ] Linux and macOS CI still pass, including the native-window check on macOS.
- [ ] Fixtures and documentation checks do not depend on inherited Git newline settings or Unix-only path conventions.
