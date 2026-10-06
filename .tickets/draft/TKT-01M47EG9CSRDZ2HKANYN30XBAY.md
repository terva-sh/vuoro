---
schema: 3
id: TKT-01M47EG9CSRDZ2HKANYN30XBAY
title: "Carry the person's verdict on submit: comment, approve, request changes"
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - forge
  - review
  - ui
  - gitlab
assignees: []
milestone: v1.0.0
parent: TKT-01M47EFQ400T20P79H9X152NAZ
origin: null
dependencies:
  - TKT-01M47EG9BHW0HBVFBTSBB1915X
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-06T01:50:57Z
updated_at: 2026-10-06T02:00:31Z
created_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
updated_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
extensions: {}
---

## Description

`forge.Writer` knows `Submit` and `Approve(bool)`, but has no "request changes". Forgejo submits a review with a verdict of `COMMENT`, `APPROVED` or `REQUEST_CHANGES`. The Forgejo posting ticket requires request changes, and GitLab has a requested-changes reviewer state as well.

Change the shared boundary so that a submit carries the person's verdict. Implement it on GitLab, and show it in the submit control. Recent GitLab has a GraphQL mutation for this; whether the configured instance supports it has not been checked, and a GitLab without it reports the capability false. The change moves the design: update docs/plan.md (the forge boundary) in the same pull request, and record the choice as the next decision in docs/decisions.md.

Keep the existing guarantee: the verdict is the person's explicit choice in the UI, and agent output never selects or sends it.

## Acceptance criteria

- [ ] Submit takes a verdict through the shared forge boundary, and GitLab implements request changes or reports it unsupported
- [ ] The submit control offers only the verdicts the forge supports
- [ ] docs/plan.md and a new docs/decisions.md entry record the boundary change

## Notes

**agent:claude/forgejo-parity-20261006** at 2026-10-06T02:00:31Z

From the spike: Forgejo submit takes COMMENT, APPROVED or REQUEST_CHANGES. REQUEST_CHANGES requires a body, and COMMENT requires a body or an inline comment. The author cannot approve or request changes on their own PR (422), so the control should offer only Comment there. A later verdict marks the same reviewer's earlier non-comment verdict dismissed.
