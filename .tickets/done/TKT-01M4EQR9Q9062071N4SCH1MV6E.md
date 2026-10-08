---
schema: 3
id: TKT-01M4EQR9Q9062071N4SCH1MV6E
title: Give vuoro a settings page, with each forge's token gauge
type: task
status: done
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
claim: null
archive: null
created_at: 2026-10-08T21:47:18Z
updated_at: 2026-10-08T21:55:33Z
created_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
updated_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
extensions: {}
---

## Description

Configuration after the first run lived in a collapsed "Accounts and review tools" panel at the bottom of the home screen, and a configured forge's token could not be checked again. The owner asked for a configuration page, with the token gauge (D57) on it.

## Acceptance criteria

- [x] A Settings button in the header opens a page with sections for forges, agent tools, background, appearance, confirmations, and about
- [x] Each configured forge shows how far its token reaches, through the client vuoro already holds, and can be checked again
- [x] The home screen's accounts panel is gone, and the colour scheme moves into Settings

## Implementation plan

Add POST /api/setup/access, which asks a configured forge's held client for its Access. Add web/src/settings.tsx with a sticky section nav. Forges become cards with a compact gauge each. A header Settings button replaces the colour scheme picker, which moves into Appearance. Remove the home screen's details panel.

## Notes

**agent:claude/forgejo-parity-20261006** at 2026-10-08T21:47:18Z

Checked in headless Chromium with two forges in a scratch config and stubbed access replies, in both schemes. Opening Settings asks each forge at once; that may resolve a token the inbox had not yet resolved, such as a pass entry that prompts.

## Summary

Merged in #35 (a9c1c19). Settings opens from the header with sections for forges, agent tools, background, appearance, confirmations, and about. Each forge card shows its token's gauge through POST /api/setup/access, which reuses the held client. Review found a double configuration read, fixed there and in forgeFor.
