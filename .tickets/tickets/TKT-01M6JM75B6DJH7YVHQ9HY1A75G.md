---
schema: 3
id: TKT-01M6JM75B6DJH7YVHQ9HY1A75G
title: Verify the GitHub release process after syncing main
type: task
status: in-progress
status_reason: null
priority: normal
due_on: null
labels:
  - release
  - ci
  - docs
assignees: []
milestone: null
parent: null
origin: null
dependencies: []
blocks_on: none
references: []
claim:
  actor: agent:codex/github-release-process
  branch: t3/github-release-process
  worktree: /home/sothr/.t3/worktrees/vuoro/t3-afae8ffa
  commit: c2ec1c06945ae25ac662a07b889d65956cd49ce5
  session: null
  claimed_at: 2026-11-04T06:34:32Z
  expires_at: null
archive: null
created_at: 2026-11-04T06:33:52Z
updated_at: 2026-11-04T06:34:33Z
created_by:
  id: agent:codex/github-release-process
  name: ""
updated_by:
  id: agent:codex/github-release-process
  name: ""
extensions: {}
---

## Description

The owner requested the latest GitHub changes be pulled and synced between the forges, then a binary release process matching the other terva projects. GitHub already contained the GoReleaser workflow and release runbook (D74); origin/main was fast-forwarded to github/main at c2ec1c06945ae25ac662a07b889d65956cd49ce5, and the clean task branch was updated to the same commit.

Validate the imported tooling, correct the runbook's publication command for this repository's configured origin (Forgejo) and github remotes, and rehearse packaging without publishing. The existing release ticket TKT-01M3YMSVKKGJ3R8EXZ8QZAMQAJ (Release for macOS, Linux, and Windows, with self-update) still owns the first live publication and remains claimed by its existing actor. GitHub currently reports no releases or release workflow runs; v0.1.0 and v0.1.1 precede the tooling. No new version or tag push is authorized by this verification task.

## Acceptance criteria

- [ ] The release runbook uses the configured GitHub remote for publication
- [ ] The complete local CI gate passes on the synced source
- [ ] A tagged rehearsal verifies all five archives without publishing

## Implementation plan

The requested sync is complete at c2ec1c0. Read and compare the imported GoReleaser config, tag-only GitHub workflow and release verifier with git-ticket-canvas. Correct the runbook to name the github publisher explicitly and keep the two main branches synchronized before a release. Run just ci and just release-check, then rehearse the committed source in a disposable tagged clone under the required agent scratch directory, with TMPDIR inherited by all build and verification tools. Record local results and the outstanding first hosted publication without moving old tags or creating a new remote tag. Reuse the imported D74 process rather than adding a competing publisher or re-opening signing decisions.
