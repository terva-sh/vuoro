---
schema: 3
id: TKT-01M4EMR39KNPGXB34K3FYHM8F4
title: Walk a fresh install through setup one step at a time
type: task
status: in-progress
status_reason: null
priority: normal
due_on: null
labels:
  - ui
assignees: []
milestone: null
parent: null
origin: null
dependencies: []
blocks_on: none
references: []
claim:
  actor: agent:claude/forgejo-parity-20261006
  branch: setup-ux
  worktree: /home/sothr/.t3/worktrees/vuoro/t3code-8cba4816
  commit: f0031cec3fc107525af7ac4621516f33048089c1
  session: null
  claimed_at: 2026-10-08T20:54:58Z
  expires_at: null
archive: null
created_at: 2026-10-08T20:54:45Z
updated_at: 2026-10-08T20:55:08Z
created_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
updated_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
extensions: {}
---

## Description

The first-run setup showed every field at once: forge kind, address, token
store, read command, a Test button numbered as a step, then username, name,
and clone protocol. It filled the left of a wide window, it showed
`pass insert vuoro/HOST` before there was a host, it repeated the read
command as `‹pass› ‹show› ‹vuoro/HOST›`, and it said Claude Code was
"signed in (oauth_token)". The owner asked for a friendlier walk-through and
chose guided steps over one page revealed section by section.

## Acceptance criteria

- [x] A fresh install sees one step at a time, with a step bar: forge, token, agent tools, background
- [x] GitHub, GitLab, and Codeberg are one click; a forge of the person's own asks for its software and address
- [x] The token step links to the forge's token page, shows the store command with a Copy button, and keeps the read command under a disclosure
- [x] A person with no forge can still reach the agent tools and background steps
- [x] Agent tools say how they are signed in, in words

## Implementation plan

Split ForgeEditor's state into a useForgeForm hook shared by the accounts panel and the first run. The first run becomes four steps with a step bar: preset forge cards, then token steps (link to the token page, store as radio pills, store command with Copy, read command under a disclosure, Check the token), then agent tools, then background. The forge saves at the end of the token step. Background autosaves each change so the wizard has no stray Save button. Agent tool login words come from signedIn().

## Notes

**agent:claude/forgejo-parity-20261006** at 2026-10-08T20:55:08Z

Checked in a headless Chromium against vuoro -browser with a scratch config, in both schemes, at 1400x900: every step, the own-forge address, the environment-variable store, and going on without a forge. Not checked in tuohi's webview itself, nor with a real token, since the scratch config has none; the vitest walk mocks the test and save calls. The Background panel in the accounts panel now saves on change too, since one control saving at once and two waiting for Save read as inconsistent.
