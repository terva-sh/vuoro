---
schema: 3
id: TKT-01M4EP0RRT9QA7ZQQKQA30E6K4
title: Show on a gauge how far a forge token reaches
type: task
status: in-progress
status_reason: null
priority: normal
due_on: null
labels:
  - ui
  - forge
  - credentials
assignees: []
milestone: null
parent: null
origin: null
dependencies: []
blocks_on: none
references: []
claim:
  actor: agent:claude/forgejo-parity-20261006
  branch: token-gauge
  worktree: /home/sothr/.t3/worktrees/vuoro/t3code-8cba4816
  commit: 5f580d2ee7af161b4bb86ad8ff21a692a9e32d69
  session: null
  claimed_at: 2026-10-08T21:16:58Z
  expires_at: null
archive: null
created_at: 2026-10-08T21:16:58Z
updated_at: 2026-10-08T21:16:58Z
created_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
updated_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
extensions: {}
---

## Description

The owner asked, on 2026-10-09, for setup to vet a token's permissions and
show them on a gauge when the check finishes: read only (local reviews, and
replies the person pastes by hand) and full integration in the green, and
anything with administration pushing the needle into the red, as broader
than vuoro needs. They invited design opinions. D57 records the result.

## Acceptance criteria

- [x] The token check places the token on a gauge: can't read, read only, full, or too broad
- [x] GitHub classic, GitLab, and Forgejo tokens are placed from what the forge reports, with no write sent to learn it
- [x] A token that cannot read reviews is not saved; one broader than needed is saved with a warning naming the extra scopes
- [x] A token the forge does not describe, such as a GitHub fine-grained one, says so and what to check

## Implementation plan

Add forge.Access and the optional forge.AccessReader. GitHub reads X-OAuth-Scopes from GET /user and calls fine-grained tokens unknown. GitLab reads /personal_access_tokens/self and is_admin. Forgejo finds its token in /users/{me}/tokens by its last eight characters, and falls back to 403-versus-404 reads of a repository that does not exist. /api/setup/test returns access. The window shows it as an SVG gauge under the token check, and a level of none blocks the save.

## Notes

**agent:claude/forgejo-parity-20261006** at 2026-10-08T21:16:58Z

Every reader is tested against an in-process fake that fails on any write. The gauge was screenshotted at each level, with the token check's reply stubbed, in headless Chromium in both schemes. No real token has been through it: GitLab's self endpoint version, and Forgejo's list-with-token support on older releases, come from source reading and are not verified live. TestLiveSetupTest now logs the access a real token gets.
