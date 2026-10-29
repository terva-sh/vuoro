---
schema: 3
id: TKT-01M64K2PVC1YW0JRB1HE693JNS
title: Update vuoro itself from the latest GitHub release
type: task
status: done
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
updated_at: 2026-10-29T19:59:25Z
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

- [x] vuoro self-update replaces an installed release with the latest one, after checking its checksum
- [x] A build that is not a release, or an archive whose checksum does not match, is refused and nothing is replaced
- [x] vuoro self-update -check says whether a newer release exists, and changes nothing
- [x] The window says when a newer release exists, and applies it only on the person's action

## Implementation plan

Port git-ticket's cli/selfupdate.go into an update package (Latest, Newer, AssetName, Apply with checksum before any swap, write-beside-and-rename, Windows moves the running binary aside). vuoro self-update [-check] [-json] in selfupdate_cmd.go with review's exit statuses. The window: GET and POST /api/self-update in app/selfupdate.go behind the guard, asking GitHub at most once a day (an hour after a failure), applying only on POST; a header notice in web/src/selfupdate.tsx.

## Summary

vuoro updates itself from the latest GitHub release (D74). The update package reads the release, refuses a build that is not a release (dev or a pseudo-version) and never offers a prerelease or a downgrade, downloads this platform's archive, checks it against checksums.txt before anything moves, and swaps the binary by writing beside it and renaming (moving the running one aside on Windows). vuoro self-update applies it, -check only asks (exit 1 when newer), -json prints an envelope. The window asks at most once a day, shows '↑ Update to vuoro vX' in the header, applies it only on the person's click, and then says to quit and reopen. GET and POST /api/self-update are behind the window guard. Tested with a fake GitHub in update_test.go, the command in selfupdate_cmd_test.go, the routes and guard in app/selfupdate_test.go, and the notice in selfupdate.test.tsx; the checksum, the version comparison, the daily cache and the newer-only rule each fail their tests when removed. Not yet run against a real published release, since none has archives until the first tag goes through the new workflow.
