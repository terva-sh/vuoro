---
schema: 3
id: TKT-01M64K2PVC1YW0JRB1HE693JNS
title: Update vuoro itself from the latest GitHub release
type: task
status: ready
status_reason: null
priority: normal
due_on: null
labels:
  - release
  - desktop
assignees: []
milestone: v1.0.0
parent: TKT-01M3YMSTYMFRDFXD4QSVWHMM8P
origin: null
dependencies:
  - TKT-01M3YMSVKKGJ3R8EXZ8QZAMQAJ
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-29T19:44:35Z
updated_at: 2026-10-29T19:44:36Z
created_by:
  id: agent:claude-code/release-tooling
  name: ""
updated_by:
  id: agent:claude-code/release-tooling
  name: ""
extensions: {}
---

## Description

D74. `vuoro self-update` reads the latest release of terva-sh/vuoro on
GitHub, compares it with the running version, downloads the archive for this
platform, checks it against `checksums.txt`, and replaces the running binary.
It follows git-ticket's `self-update` (cli/selfupdate.go): a build that is not
a release (a pseudo-version, or dev) is refused rather than guessed at, and a
prerelease is never offered.

The window then offers it: while open it checks for a newer release at most
once a day, says so in the header, and on the person's say applies it and asks
them to reopen vuoro. It never updates without the person's action.

## Acceptance criteria

- [ ] vuoro self-update replaces an installed release with the latest one, after checking its checksum
- [ ] A build that is not a release, or an archive whose checksum does not match, is refused and nothing is replaced
- [ ] vuoro self-update -check says whether a newer release exists, and changes nothing
- [ ] The window says when a newer release exists, and applies it only on the person's action
