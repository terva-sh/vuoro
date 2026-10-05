---
schema: 3
id: TKT-01M46DJMY9AT5YX6FV5FWR4CR6
title: An expanded finding squeezes its title into a narrow column
type: bug
status: done
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
updated_at: 2026-10-05T16:31:49Z
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

- [x] An expanded finding keeps its one-line header above the detail, with no overlap, at 1400px and at narrow widths

## Notes

**agent:claude-code/review-demo-20261005** at 2026-10-05T16:31:49Z

Cause: an undecided finding got the class 'open' (finding ${decision ?? 'open'}), which matched the header's '.open { display: flex }' rule for the URL form. An opened finding then laid out its summary and body side by side. Kept and dropped findings were unaffected. Fix: undecided findings use the class 'undecided', and the header rule is scoped to form.open so the two cannot meet again. The test asserts the class. Live screenshot at 1400px shows the header on one line above the detail.

## Summary

An expanded finding keeps its one-line header. Undecided findings no longer carry the header form's 'open' class, and that rule is scoped to form.open.
