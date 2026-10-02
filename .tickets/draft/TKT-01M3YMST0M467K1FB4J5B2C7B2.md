---
schema: 3
id: TKT-01M3YMST0M467K1FB4J5B2C7B2
title: Move vuoro to terva-sh/vuoro on Forgejo once the skeleton lands
type: chore
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - ci
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies:
  - TKT-01M3W4HWCNR50KXMCX741QD9XQ
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-02T15:47:50Z
updated_at: 2026-10-02T15:47:50Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
extensions: {}
---

## Description

Decided on 2026-10-02: the repository moves to the internal Forgejo once the
walking skeleton lands, so that its CI and the terva-sh review process apply
early. There is no public mirror until v1.0.0.

The owner creates the repository and pushes. An agent prepares the branch and
the Forgejo workflow and stops there.

## Acceptance criteria

- [ ] terva-sh/vuoro exists on Forgejo, created and pushed by the owner
- [ ] The Forgejo CI workflow runs just ci on a pull request
