---
schema: 3
id: TKT-01M6JNAMK8A9N8PAQ3RG6JK0QJ
title: Publish vuoro v0.1.2 with verified binary archives
type: task
status: in-progress
status_reason: null
priority: normal
due_on: null
labels:
  - release
  - ci
assignees: []
milestone: null
parent: null
origin: null
dependencies: []
blocks_on: none
references: []
claim:
  actor: agent:codex/release-v0.1.2
  branch: t3/github-release-process
  worktree: /home/sothr/.t3/worktrees/vuoro/t3-afae8ffa
  commit: 9cde17fad0ce4d6a8a52715922dc5d546d31a2b4
  session: null
  claimed_at: 2026-11-04T06:53:55Z
  expires_at: null
archive: null
created_at: 2026-11-04T06:53:14Z
updated_at: 2026-11-04T07:00:41Z
created_by:
  id: agent:codex/release-v0.1.2
  name: ""
updated_by:
  id: agent:codex/release-v0.1.2
  name: ""
extensions: {}
---

## Description

The owner requested a release that publishes downloadable binaries. Both remote main branches are at c2ec1c06945ae25ac662a07b889d65956cd49ce5. The latest existing tag is v0.1.1; v0.1.2 is the next unused patch release, with D74 packaging, self-update, and decided findings for follow-up reviews.

Prepare and verify the candidate in a new disposable full clone under ~/.cache/agent-scratch/vuoro, preserve existing working trees, publish the exact annotated tag to both forges, and verify all GitHub artifacts. The owner's request authorizes the release/tag pushes; no existing tag may be moved. The existing release-tooling ticket remains claimed by its previous actor; record the publication evidence without overwriting its claim. Disposable rehearsal clones and generated artifacts may be removed after verification.

The current internal CI status is failure (run 344). The required forgejo-job-log helper exits 3 because the repository is private on Forgejo 15 and no permitted log route exists. Reproduce locally rather than trying another credential or web-UI authentication. No release has been published yet.

## Acceptance criteria

- [ ] The release candidate passes local checks and a tagged archive verification
- [ ] The v0.1.2 tag is present on both forges and GitHub publishes five archives plus checksums
- [ ] Downloaded public archives pass checksums and provenance verification

## Implementation plan

Use the synced main c2ec1c0 as the candidate for the next unused patch v0.1.2. The local just ci gate passes again. Reproduce the internal failure with the documented Go Alpine image, without attempting another log credential. Build from a clean full clone tagged v0.1.2, verify all five archives, and use the existing tag-triggered GitHub publisher. If hosting prevents Actions execution, retain the tagged verified bytes and use the same draft-then-public release protocol rather than bypassing any verification. Download all six public assets and verify checksums, archive documents and binary provenance; smoke-test the native binary and self-update check. Record the exact tag, commit, hashes, hosted results and limitations. Preserve existing tags and working trees, and commit the understood runbook correction and ticket progress.

## Notes

**agent:codex/release-v0.1.2** at 2026-11-04T06:57:17Z

The full workstation just ci gate passed again, but reproduction in the exact container.local.sothr.com/library/golang:1.27-alpine image failed in six merge/update tests. Git merge --no-commit still requires a committer identity, and mergeStep called gitIn with nil environment before finish supplied the selected identity. A configured workstation masks this real clean-machine bug. Fixed mergeStep to pass identityEnv(who) to merge preparation. Strengthened TestACleanMergeNeedsNoAgent to disable global/system Git config and verify both author and committer names/emails. Configuring a global identity in CI was rejected because that would hide the product bug instead of fixing it.

The existing synced main c2ec1c0 is therefore not releaseable unchanged. The fix and the understood runbook correction will land through an internal pull request before tagging v0.1.2. The release will use the resulting green main commit; the proposed notes include the merge identity fix. No new remote tag has been pushed. The owner requested cutting the release, which authorizes publication; existing tags remain untouched.

**agent:codex/release-v0.1.2** at 2026-11-04T06:58:47Z

The regression now explicitly sets user.useConfigOnly=true as well as disabling system/global config, since on some workstations Git can infer a usable email from the hostname. With the production fix removed in the disposable clone, TestACleanMergeNeedsNoAgent fails with 'no email was given and auto-detection is disabled'. With identityEnv(who) supplied during merge preparation, the same test passes. The complete original Alpine gate is being rerun with the fix; the baseline failed six tests.

Proposed v0.1.2 notes: first downloadable cgo-free binaries for macOS/Linux amd64+arm64 and Windows amd64, SHA-256 checksums and notices; CLI and explicit window self-update; decided findings passed into follow-up reviews; release version/commit read from Go build metadata; merge preparation uses the selected committer on a clean machine. Known limits remain unsigned/unnotarized macOS binaries, unsigned checksums, and desktop tray/notification validation outstanding. No recorded owner decision is reopened.

**agent:codex/release-v0.1.2** at 2026-11-04T07:00:41Z

The complete gate now passes in the exact internal Go 1.27.1 Alpine image, with no configured user identity: all three vet targets, gofmt, Go tests, race tests, six cross builds, design check, npm ci, frontend typecheck and all 225 tests in 35 files, committed dist parity, generated notices, and strict ticket checks. The successful log is ~/.cache/agent-scratch/vuoro/release-v0.1.2-BWNKdZ/internal-gate-fixed.log. The baseline reproduction log beside it contains the original six failures; neither contains credentials.

A clean tagged release rehearsal of committed source baaeb4b also passed: five archives, SHA-256 checksums, exact notices/documents, target/tag/commit provenance, CGO_ENABLED=0, native -version and review -h. The disposable rehearsal clone and artifacts were removed by the helper. The v0.1.2 notes are prepared in ~/.cache/agent-scratch/vuoro/release-v0.1.2-BWNKdZ/release-notes.md and summarized in the preceding note. Next: internal PR and model review, merge the green fix, sync GitHub main, then create/push the annotated v0.1.2 tag at that merged commit and validate every public asset. Existing v0.1.0 and v0.1.1 are left untouched.
