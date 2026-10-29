---
schema: 3
id: TKT-01M3YMSVKKGJ3R8EXZ8QZAMQAJ
title: Release for macOS, Linux, and Windows, with self-update
type: task
status: in-progress
status_reason: null
priority: normal
due_on: null
labels:
  - release
  - ci
assignees: []
milestone: v1.0.0
parent: TKT-01M3YMSTYMFRDFXD4QSVWHMM8P
origin: null
dependencies:
  - TKT-01M3W4HWB551REE766G3TE2Y7D
blocks_on: none
references: []
claim:
  actor: agent:claude-code/release-tooling
  branch: tickets/release-tooling
  worktree: /Users/drewshort/.t3/worktrees/vuoro/t3-a510f89e
  commit: efcff537b0fb99428e2a44ad05b061495c96a0f2
  session: null
  claimed_at: 2026-10-29T19:44:36Z
  expires_at: null
archive: null
created_at: 2026-10-02T15:47:52Z
updated_at: 2026-10-29T19:44:40Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude-code/release-tooling
  name: ""
extensions: {}
---

## Description

goreleaser releases, as other terva-sh tools have them, and `vuoro self-update`,
as git-ticket has it. Decide macOS signing and notarization here.

## Acceptance criteria

- [ ] A tagged release publishes archives for macOS, Linux, and Windows
- [x] The macOS signing decision is recorded
- [ ] The GitHub workflow runs on a v* tag alone, and verifies the archives before it publishes them (D74)
- [ ] Each archive names its tag and commit through the build information, and carries LICENSE, THIRD_PARTY_LICENSES and README-release.md

## Notes

**agent:claude-code/release-tooling** at 2026-10-29T19:44:36Z

Owner decisions on 2026-10-29 (D74): GitHub runs release CI/CD only, per-branch CI stays internal; follow the siblings' release conventions; the macOS binary ships unsigned with directions; self-update moves to its own ticket, TKT-01M64K2PVC1YW0JRB1HE693JNS. The self-update criterion is removed here for that reason, and 'The macOS signing decision is recorded' is met by D74.
