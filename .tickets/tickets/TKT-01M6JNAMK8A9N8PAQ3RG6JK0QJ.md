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
updated_at: 2026-11-04T06:53:55Z
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
