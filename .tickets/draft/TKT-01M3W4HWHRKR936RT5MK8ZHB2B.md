---
schema: 3
id: TKT-01M3W4HWHRKR936RT5MK8ZHB2B
title: Load the terva-sh design stylesheet, with the export check in CI
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - ui
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies:
  - TKT-01M3W4HWGGB7RMVFAJPW2KPKY5
  - TKT-01M3W4HWF70KRCRRXZQT2M9C7Y
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

Adopt terva-sh/design as its README describes: `go get` the release that
carries the vuoro accent, export the stylesheet into `web/`, load it before the
app's own styles, and write components against `--ui-*` roles only.

Honour `data-scheme` for light and dark, and leave `data-theme` alone.

## Acceptance criteria

- [ ] design-tokens export -app vuoro -check runs in just ci
- [ ] The app follows the system scheme, and data-scheme on html overrides it both ways
- [ ] No component stylesheet contains a literal colour
