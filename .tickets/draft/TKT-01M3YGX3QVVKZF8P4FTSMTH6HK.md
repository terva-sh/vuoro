---
schema: 3
id: TKT-01M3YGX3QVVKZF8P4FTSMTH6HK
title: Show the review action's published findings on Forgejo pull requests
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - forgejo
  - review
  - agent
assignees: []
milestone: null
parent: null
origin: null
dependencies:
  - TKT-01M3W4HX4CJE62G9R124EV5PT4
blocks_on: none
references:
  - ref: repo:terva-sh/terva-action-code-review
    path: null
claim: null
archive: null
created_at: 2026-10-02T14:39:44Z
updated_at: 2026-10-02T14:39:44Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
extensions: {}
---

## Description

On a Forgejo pull request where terva-sh/terva-action-code-review has run, its
review, its inline findings, and its `terva-review/*` statuses are already on
the forge. Show them in the change view as review state, read from the forge,
and never post them again (docs/plan.md section 6.3).

Offer the action's disposition command, `/terva disposition REVIEW_ID
FINDING_ID accepted|declined|deferred reason`, as a control on each finding.
The control drafts the comment and the person posts it. When the git-ticket
integration is on, the same disposition can also be written as a ticket note.

## Acceptance criteria

- [ ] A pull request reviewed by the action shows its findings and status in the change view
- [ ] Nothing the action published is posted again by vuoro, and a test proves it
- [ ] The disposition control drafts the /terva disposition comment for the person to post
