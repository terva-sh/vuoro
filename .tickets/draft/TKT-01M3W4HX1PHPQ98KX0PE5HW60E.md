---
schema: 3
id: TKT-01M3W4HX1PHPQ98KX0PE5HW60E
title: Show a change's ticket, and give it to reviewers as context
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - git-ticket
  - review
assignees: []
milestone: v1.0.0
parent: TKT-01M3YMSTYMFRDFXD4QSVWHMM8P
origin: null
dependencies:
  - TKT-01M3W4HX0BNXTDC2DXBH5HQ0FM
  - TKT-01M3W4HWSGA04D4VQ8WHBEJC2D
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-01T16:25:25Z
updated_at: 2026-10-29T18:49:01Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude-code/release-0.1.0
  name: ""
extensions: {}
---

## Description

The MVP half of the git-ticket integration. When a change links to a ticket in
a known local store, show its description, acceptance criteria, plan, and
newest note beside the change, and give them to every reviewer session as
context, so that the review checks the change against what was asked. Writing
dispositions back is v1.0.0.

Without a store, vuoro behaves exactly as it does without this ticket.

## Acceptance criteria

- [ ] A linked change shows its ticket's acceptance criteria in the change view
- [ ] With no store configured, nothing in the UI mentions tickets
- [ ] A reviewer's prompt includes the linked ticket's acceptance criteria

## Notes

**agent:claude-code/release-0.1.0** at 2026-10-29T18:49:01Z

Moved from the v0.1.0 epic to v1.0.0 by the owner on 2026-10-29 (D73).
