---
schema: 3
id: TKT-01M6JM75B6DJH7YVHQ9HY1A75G
title: Verify the GitHub release process after syncing main
type: task
status: done
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
claim: null
archive: null
created_at: 2026-11-04T06:33:52Z
updated_at: 2026-11-04T06:36:29Z
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

- [x] The release runbook uses the configured GitHub remote for publication
- [x] The complete local CI gate passes on the synced source
- [x] A tagged rehearsal verifies all five archives without publishing

## Implementation plan

The requested sync is complete at c2ec1c0. Read and compare the imported GoReleaser config, tag-only GitHub workflow and release verifier with git-ticket-canvas. Correct the runbook to name the github publisher explicitly and keep the two main branches synchronized before a release. Run just ci and just release-check, then rehearse the committed source in a disposable tagged clone under the required agent scratch directory, with TMPDIR inherited by all build and verification tools. Record local results and the outstanding first hosted publication without moving old tags or creating a new remote tag. Reuse the imported D74 process rather than adding a competing publisher or re-opening signing decisions.

## Notes

**agent:codex/github-release-process** at 2026-11-04T06:36:28Z

Synchronized origin/main (Forgejo) from f891815 to github/main c2ec1c06945ae25ac662a07b889d65956cd49ce5 by the existing just sync-github --yes recipe, and fast-forwarded this clean task branch. The other existing working trees were left untouched. Both remote main refs were verified equal with git ls-remote.

The imported D74 tooling already follows git-ticket-canvas: GoReleaser packages five cgo-free targets, verifies them before a draft release is published, and ships notices plus checksums. Compared the sibling's current docs/releasing.md and public release workflow over anonymous Forgejo HTTPS. Reusing the imported implementation avoids competing publishers and preserves the owner's unsigned-macOS decision. Corrected docs/releasing.md to explain origin versus github, sync main before tagging, and send the exact tag to github for publication; the previous git push origin main vX.Y.Z would reach only Forgejo. The runbook edit remains uncommitted for review under the repository's commit-on-request rule; ticket-store changes are committed as required by the machine instructions.

Validation on Linux amd64 with the installed mise Go 1.27.3 and GoReleaser v2.18.1: just ci passed (vet for linux/darwin/windows, Go tests and race tests, six cross builds, frontend typecheck and 225 tests across 35 files, committed dist parity, design and notices checks, strict ticket check). just release-check passed. just release-rehearse passed from committed HEAD 632578d: all five archives, SHA-256 checksums, exact documents, clean tag/commit provenance and CGO_ENABLED=0 were verified; the native binary reported vuoro v0.0.0-rehearsal and its review command answered. TMPDIR was set to a mktemp directory under ~/.cache/agent-scratch/vuoro for all build/test/rehearsal runs. The disposable clone, tag and archives were removed by the rehearsal.

GitHub Actions is enabled; all four pinned action SHAs match their documented upstream tags. GitHub's releases API reports zero releases and the release workflow has zero runs. Existing v0.1.0 and v0.1.1 tags precede the workflow. No remote tags were pushed and no version was chosen or published. The existing release ticket retains its current claim and its unticked live-publication criterion; the first real tag push and artifact download must still establish hosted publication. No remote settings or credentials were changed.

## Summary

Pulled GitHub's existing D74 binary release tooling and fast-forwarded Forgejo main to the same c2ec1c0 source. Verified the local full CI gate, release configuration and all five archives in a clean tagged rehearsal. Corrected the release runbook's remote names and publication sequence; that documentation edit is left uncommitted for review. Ticket-store records are committed separately. First hosted publication remains on TKT-01M3YMSVKKGJ3R8EXZ8QZAMQAJ (Release for macOS, Linux, and Windows, with self-update), after the person chooses and approves a new tag; no release or tag push was performed here.
