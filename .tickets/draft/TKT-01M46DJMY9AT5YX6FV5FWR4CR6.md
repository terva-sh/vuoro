---
schema: 3
id: TKT-01M46DJMY9AT5YX6FV5FWR4CR6
title: An expanded finding squeezes its title into a narrow column
type: bug
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - ui
  - review
assignees: []
milestone: v0.1.0
parent: null
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-05T16:15:31Z
updated_at: 2026-10-05T16:15:31Z
created_by:
  id: agent:claude-code/review-demo-20261005
  name: ""
updated_by:
  id: agent:claude-code/review-demo-20261005
  name: ""
extensions: {}
---

## Description

On the Review tab, expanding a finding (seen on drewshort/vuoro-sandbox!2, 2026-10-05, 1400px wide) lays the summary row out as a column beside the detail. The title wraps one word per line, and the file:line label overlaps it ("ByName" over "users.go:17"). Collapsed rows and the Changes tab's inline findings render correctly.

## Acceptance criteria

- [ ] An expanded finding keeps its one-line header above the detail, with no overlap, at 1400px and at narrow widths
