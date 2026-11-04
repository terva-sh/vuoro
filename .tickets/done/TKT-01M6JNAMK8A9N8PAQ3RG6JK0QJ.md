---
schema: 3
id: TKT-01M6JNAMK8A9N8PAQ3RG6JK0QJ
title: Publish vuoro binaries from a verified GitHub release
type: task
status: done
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
claim: null
archive: null
created_at: 2026-11-04T06:53:14Z
updated_at: 2026-11-04T15:08:34Z
created_by:
  id: agent:codex/release-v0.1.2
  name: ""
updated_by:
  id: agent:codex/release-v0.1.2
  name: ""
extensions: {}
---

## Description

The owner requested a release that publishes downloadable binaries using the D74 GoReleaser/GitHub workflow. Finish the first automated publication as v0.1.3, including all five cgo-free platform archives, checksums, notices and verified public downloads.

The initial v0.1.2 attempt at cb9b0be passed internal CI, model review, the local tagged build, and GitHub's full release gate, packaging and archive checks. Publishing stopped before creating any release because gh rejects --notes-from-tag with --repo. Keep that tag unchanged, fix the command on main, and cut the next patch tag. The earlier clean-machine merge identity blocker was reproduced and fixed in merged PR #39 (Fix merge preparation on machines without a Git identity).

The owner's request authorizes release/tag pushes. No existing tag may be moved, and existing working trees must be preserved. Build and verify the final tag in a disposable full clone under ~/.cache/agent-scratch/vuoro and remove only task-owned scratch artifacts afterward. The existing release-tooling ticket still owns its prior claim; add publication evidence without taking over that claim. No remote settings or credentials need to change.

## Acceptance criteria

- [x] The release candidate passes local checks and a tagged archive verification
- [x] Downloaded public archives pass checksums and provenance verification
- [x] The v0.1.3 tag is present on both forges and GitHub publishes five archives plus checksums

## Implementation plan

Fix the GitHub publishing command before the first downloadable release. v0.1.2 at cb9b0be passed the real GitHub release gate, five builds and archive verification, but gh rejected --notes-from-tag together with --repo before creating any release. Remove --repo from gh release create so the checked-out GitHub repository supplies its context; retain the explicit repository on release edit. Land this through an internal PR with CI and model review, synchronize main, build and verify the next unused tag v0.1.3 in a clean full clone, then push that tag to both forges. Leave v0.1.2 unchanged and unpublished, following the fix-forward runbook. Download all six public assets and verify checksums, documents, clean tag/commit provenance, native version and the updater's check-only command. Record exact hashes, hosted results and limitations; close the earned release criteria only after public verification.

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

**agent:codex/release-v0.1.2** at 2026-11-04T07:09:20Z

PR #39 (Fix merge preparation on machines without a Git identity), https://git.local.sothr.com/terva-sh/vuoro/pulls/39, passed ci / Vet, test and cross-build and terva-review/code on cb9b0be80038a6d197b0cfa777a87874798117c1. Model review run cab8265e-888a-489f-8d5c-2663f99aa2b8 identified no findings; its scope was the diff and issue discussion, not execution. Inspected the review and status before a guarded fast-forward-only merge. The checked SHA is therefore the exact commit on both main branches, rather than an untested merge commit.

Created the annotated v0.1.2 tag at that commit with the prepared release notes and the repository's configured identity. Built the real tag in a new clean full clone under ~/.cache/agent-scratch/vuoro/release-v0.1.2-BWNKdZ/release-source with GoReleaser v2.18.1, CGO_ENABLED=0 and publishing disabled. The verifier passed all five archives, SHA-256 checksums, exact documents, target/module/tag/clean commit provenance, native -version ('vuoro v0.1.2'), and review -h. Pushed the exact tag to origin and github. Both remote tag objects are 90951bc3b6c56d4bb35bf3f85fdc00b1b4eb4343 and peel to cb9b0be80038a6d197b0cfa777a87874798117c1. No existing tag was moved. Awaiting GitHub publication before ticking the public-asset criteria. PR #39 is merged and its watcher has been stopped.

**agent:codex/release-v0.1.2** at 2026-11-04T07:14:53Z

The first live GitHub workflow ran at https://github.com/terva-sh/vuoro/actions/runs/41790234893. The release gate, GoReleaser builds, and archive verification all passed. Publishing then failed before any release/draft was created: gh reported "using `--notes-from-tag` with `--repo` is not supported". Read the failed job's logs with gh run view --log-failed; the GitHub releases API still reports zero releases. This is a publisher argument bug, not a build or artifact-verification failure.

Removed --repo from gh release create; --notes-from-tag needs the local checkout, whose origin is this GitHub repository under the workflow's GitHub-only server guard. Retained --repo on gh release edit, which has no incompatible tag-notes option. This preserves annotated tag notes and the draft-then-public protocol. Moving or recreating the remote v0.1.2 tag was rejected by the runbook; v0.1.3 will be the first published binaries after the fix lands. Publishing local v0.1.2 artifacts manually was also set aside so the first release proves the complete automated workflow. The previous checked local candidate remains valid evidence, but criterion 1 is reset until the final v0.1.3 tag is verified. New annotated notes are prepared as release-notes-v0.1.3.md under the same task scratch root.

**agent:codex/release-v0.1.2** at 2026-11-04T07:23:15Z

PR #40 (Fix GitHub release publishing with annotated tag notes), https://git.local.sothr.com/terva-sh/vuoro/pulls/40, received a clean terva-review/code result on e24b61382e0f7bff7c27cfca440f34a3332ca71b (review run 0e7d6a75-036e-4e20-b296-eac01e6fe8c7). No findings required a disposition. The review specifically asked that the checked-out repository context be verified; running gh repo view in the fresh GitHub-origin release checkout returned terva-sh/vuoro, and git confirmed the release tag is annotated.

Hosted CI run 349 then failed after 4m30s. Invoked the required forgejo-job-log helper on its exact job URL first; it exited 3 because this private Forgejo 15 repository has no permitted log route. No alternate credential or web-UI route was attempted. Reproduced the entire gate in a fresh full clone of e24b613 in the documented Go Alpine container, digest sha256:a6c2a4b91494a9f838334a6343d05f541f9aea9f5583b1f72fdb2b901d42f877. Every step passed: vet on three OSes, gofmt, normal/race tests, six cross builds, design, frontend install/typecheck/all 225 tests, committed dist parity, notices and strict ticket checks. The task-owned log is ~/.cache/agent-scratch/vuoro/release-v0.1.2-BWNKdZ/internal-gate-e24b613.log.

The hosted failure has not been reproduced or attributed. No speculative product/test changes are warranted by the available evidence. Commit this evidence as a ticket-only change to trigger a new hosted run, and carry the existing clean review to the new head. Do not merge until the hosted gate is green. The first public release remains v0.1.3 after merge; v0.1.2 remains unchanged and unpublished.

**agent:codex/release-v0.1.2** at 2026-11-04T07:31:49Z

PR #40 (Fix GitHub release publishing with annotated tag notes) passed hosted CI on 14c6daf062d2683b7c1cc196d13381419f926dc4 in run 352, and the clean model review was carried to that head because the later commit changed only .tickets/. The first failed CI run 349 remains unexplained; the exact fresh-clone container gate and the hosted retry both passed without product changes. Inspected the status and review result, then fast-forward merged PR #40 with a head-SHA guard. Both main branches now point to the exact reviewed/checked SHA, and the merged PR watcher was stopped.

Created annotated v0.1.3 on 14c6daf062d2683b7c1cc196d13381419f926dc4 and built it from a new clean full clone using GoReleaser v2.18.1 with CGO_ENABLED=0 and publishing disabled. All five archives passed SHA-256, documents/notices, module/tag/commit/target provenance, clean-tree, native -version ('vuoro v0.1.3') and review -h verification. Pushed v0.1.3 to both remotes; both tag objects are bc4adff201183f149317a02b00363a1a25deca27, peeling to 14c6daf062d2683b7c1cc196d13381419f926dc4. v0.1.2 was not moved. Awaiting the corrected GitHub workflow and anonymous asset verification before checking the publication criteria.

**agent:codex/release-v0.1.2** at 2026-11-04T07:35:18Z

Published v0.1.3 successfully through the corrected tag-triggered GitHub workflow: https://github.com/terva-sh/vuoro/actions/runs/41792822749. Every step passed, including the release gate, five builds, archive verification and draft-to-public publication. Public stable release: https://github.com/terva-sh/vuoro/releases/tag/v0.1.3.

Anonymous downloads of all six assets passed size/API digest checks and scripts/verify-release.py against the clean v0.1.3 checkout: SHA-256 checksums, exact LICENSE/THIRD_PARTY_LICENSES/README-release.md, module/target/tag/commit metadata, clean provenance and CGO_ENABLED=0. The extracted public linux_amd64 binary reports 'vuoro v0.1.3'; self-update -check -json returns current/latest v0.1.3, newer=false and applied=false. This exercises discovery without modifying the executable.

Both remote main branches remain 14c6daf062d2683b7c1cc196d13381419f926dc4. Both remote v0.1.3 tag objects are bc4adff201183f149317a02b00363a1a25deca27 and peel to that exact checked/reviewed commit. Rechecked via git ls-remote after publication. Existing v0.1.2 stays unchanged and unpublished; it exposed the gh argument bug repaired through PR #40.

### Public asset evidence

| Asset | Bytes | SHA-256 |
| --- | ---: | --- |
| checksums.txt | 486 | e7046448b8a636651df4dae9cfbe0de221900608f3290315c92aa25eb1ba0e23 |
| vuoro_0.1.3_darwin_amd64.tar.gz | 4756037 | 12032e2bdee64a80571b4609c21b286029dc89aa261c72b73d612af36dcb04d1 |
| vuoro_0.1.3_darwin_arm64.tar.gz | 4435582 | 8278c936e9f0840faa4a43bd33635fcdf6ea4ab45e05ea40e7f8abf651730f20 |
| vuoro_0.1.3_linux_amd64.tar.gz | 5366606 | 48de662ba47ce0a5e8e376a977de5ec652cc91631d733b3b040ccc5185a53963 |
| vuoro_0.1.3_linux_arm64.tar.gz | 4883655 | 14b0d03ceb9a2b626c55e234b043467cb616cd259dd6e0a10470df69f7656f62 |
| vuoro_0.1.3_windows_amd64.zip | 4723415 | 69e7b3d2d0fa535799bf24bab6f246519fe04ec2e7a77e67986eb3922ed9bab5 |

### Limits and record

macOS binaries remain unsigned and unnotarized under D74; checksums are unsigned. Native execution was verified on Linux amd64; the remaining targets were cross-built and their embedded provenance verified, without claiming native GUI validation. Desktop tray/notification live validation remains outside this release verification, as disclosed in the annotated release notes. The public release and these hashes are the durable evidence; local scratch files are disposable. Add the earned publication evidence to TKT-01M3YMSVKKGJ3R8EXZ8QZAMQAJ (Release for macOS, Linux, and Windows, with self-update) without taking over its existing claim.

**agent:codex/release-v0.1.2** at 2026-11-04T15:06:50Z

The owner supplied the previously inaccessible output from Forgejo CI run #349: go test -race failed only in TestAFreshInstallReachesAFirstReviewWithoutEditingAFile with TempDir RemoveAll cleanup: directory not empty. This supersedes the earlier notes that left run #349 unexplained. It was a test lifecycle failure during cleanup, without a Go race-detector report.

Reproduced the same cleanup error in 8 of 100 focused race runs at 9a846a2 using Go 1.27.3 on Linux amd64. The fresh-install test never shut down its review.Manager. review.pump publishes Idle under the run lock before saving its record, so observing Idle does not join its disk writes; moreover Idle can occur between a verifier's first turn and its follow-up nudge. The existing Settled method accounts for that pending nudge. The Manager.Shutdown join already exists (TKT-01M4CPAZX0WNTCCV7R3HJ5MAJP, Wait for a review's goroutines in Shutdown; a local-review test flaked), but this newer test omitted it.

Changed only app/firstrun_test.go: defer Runs.Shutdown while both fake servers are still alive, and wait/assert Settled as well as Idle before examining the review. A shutdown-only first attempt exposed the fake session sending to a channel closed during the pending verifier nudge; waiting for Settled fixes the test's premature completion assumption too. The final focused race run passes 100/100 (11.843s); the baseline had failed 8/100 (9.128s). Logs are task-owned under ~/.cache/agent-scratch/vuoro/first-review-cleanup-xGCRov. Full gate verification follows before submission.

Adding sleeps, retrying RemoveAll and suppressing cleanup errors were rejected because they mask the unjoined writer. No product lifecycle or release artifact changes are needed: v0.1.3 remains published and unchanged. This is follow-up evidence and test maintenance for the completed release investigation, not a claim that the prior full-gate reproductions failed.

**agent:codex/release-v0.1.2** at 2026-11-04T15:08:34Z

The full workstation just ci gate passed on the first-review cleanup fix: vet for native/darwin/windows, gofmt, normal Go tests, the entire race suite, all six cgo-free cross builds, frontend typecheck and all 225 tests across 35 files, committed web/dist parity, shared design and third-party notices checks, and strict ticket validation. The final source changes are confined to app/firstrun_test.go; the release record remains done. This validates the regression fix beyond the focused 100/100 race run. Submit the test and evidence through a new internal PR for hosted CI and model review, without changing v0.1.3.

## Summary

Published and anonymously verified v0.1.3 at https://github.com/terva-sh/vuoro/releases/tag/v0.1.3 from checked commit 14c6daf062d2683b7c1cc196d13381419f926dc4. The tag-triggered GitHub workflow passed end to end and publishes five cgo-free archives plus checksums. All public downloads passed checksum, documents and provenance verification; the native Linux binary reports v0.1.3 and self-update correctly reports it current. Main and the annotated tag match on both forges. PR #39 fixed clean-machine merge identity; PR #40 fixed tag-note publishing. v0.1.2 remains unchanged and unpublished. Known unsigned macOS/checksum and GUI validation limits remain documented.
