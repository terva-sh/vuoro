---
schema: 3
id: TKT-01M3W4HX5Q5H6J8JHKHYKYEVMA
title: "vuoro inbox --json: the inbox for scripts, without the window"
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - inbox
assignees: []
milestone: v1.0.0
parent: TKT-01M3YMSTYMFRDFXD4QSVWHMM8P
origin: null
dependencies:
  - TKT-01M3W4HWNGYDHGXMSE91HKGTNT
  - TKT-01M3W4HWPT9NYP4ADW339FJDSC
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-01T16:25:25Z
updated_at: 2026-10-02T15:47:52Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
extensions: {}
---

## Description

A headless subcommand that prints the inbox, with turns, reasons, and ages, as
one JSON envelope in the git-ticket style. Morning and evening ritual scripts
can then ask "what is waiting on me" without scraping a forge. It uses the
same adapters and turn package as the window, and it shares the cache.

## Acceptance criteria

- [ ] vuoro inbox --json prints every change with its turn, reasons, and since, and exits 0
- [ ] The output has a schemaVersion and a kind, and a test holds its shape
