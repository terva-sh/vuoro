---
schema: 3
id: TKT-01M4ES1EPBYF4Q2VQAFQ5YK7G0
title: Record a suggested fix once the author applies it
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - review
  - state
assignees: []
milestone: null
parent: TKT-01M4ES1E4BJAZ78GB96H20W1RD
origin: null
dependencies:
  - TKT-01M4ES1EGCHKQXWYS9FY4QYQCC
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-08T22:09:46Z
updated_at: 2026-10-08T22:09:46Z
created_by:
  id: agent:claude/editor-suggestions-2026-10-08
  name: ""
updated_by:
  id: agent:claude/editor-suggestions-2026-10-08
  name: ""
extensions: {}
---

## Description

D59: a suggested fix is detected once the author applies it. "Mark fixed…" and "Review again" already read the change's new head from the cache. When the change has moved, vuoro checks each suggested finding: is every replacement in the file at the new head, with the lines that bordered it? If so, it records the oldest commit since the review after which they are present as FixedIn, with FixedBy "suggestion". The earlier-findings standing then reads fixed. A link made by a suggestion can be taken back like one the person made.

## Acceptance criteria

- [ ] A suggested finding whose replacement is in place at a later head is recorded fixed by the commit that brought it, which a test proves
- [ ] A replacement not in place, or in place without its bordering lines, leaves the finding suggested
