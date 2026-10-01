---
schema: 3
id: TKT-01M3W4HWF70KRCRRXZQT2M9C7Y
title: Add a vuoro accent to terva-sh/design and release it
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - ui
  - cross-repo
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-01T16:25:25Z
updated_at: 2026-10-01T16:25:25Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
extensions: {}
---

## Description

terva-sh/design v0.1.0 has accents for terva, lampi, ketju, and
git-ticket-canvas, and none for vuoro. `design-tokens export -app vuoro` cannot
work until one exists.

This is a change in the design repository, not here: add `vuoro` under
`accents` in `tokens.json`, run `just generate` and `just ci` there (the
contrast test checks both schemes), and tag a release. Pick a hue that is
distinct from the existing four. lampi and ketju are both blue.

Record the design release here when it lands.

## Acceptance criteria

- [ ] terva-sh/design has a vuoro accent that passes its contrast test in both schemes
- [ ] A design release containing it is tagged, and the version is recorded in a note here
