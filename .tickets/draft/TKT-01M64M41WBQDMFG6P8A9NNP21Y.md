---
schema: 3
id: TKT-01M64M41WBQDMFG6P8A9NNP21Y
title: Sign release checksums and verify them on self-update
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - release
assignees: []
milestone: null
parent: null
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-29T20:02:48Z
updated_at: 2026-10-29T20:02:48Z
created_by:
  id: agent:claude-code/release-tooling
  name: ""
updated_by:
  id: agent:claude-code/release-tooling
  name: ""
extensions: {}
---

## Description

Self-update checks a downloaded archive against the release's `checksums.txt`.
That proves the archive is the one the release lists, but whoever can publish
a release can publish a matching `checksums.txt`, so an update is only as
trustworthy as the GitHub release. Raised by the release-tooling review round
(d7) and recorded as a known gap in docs/plan.md section 9.1.

Sign `checksums.txt` in the release workflow (cosign keyless or minisign, to be
chosen with the owner alongside the macOS signing setup), publish the
signature with the release, and have the `update` package verify it against a
key or identity built into vuoro before anything is replaced.

## Acceptance criteria

- [ ] The release workflow signs checksums.txt and publishes the signature
- [ ] Self-update refuses an archive whose checksums signature does not verify
- [ ] docs/plan.md 9.1 no longer lists the gap
