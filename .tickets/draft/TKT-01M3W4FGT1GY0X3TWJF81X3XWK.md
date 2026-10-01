---
schema: 3
id: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
title: "vuoro v0.1.0: one inbox for the reviews I give and get"
type: epic
status: draft
status_reason: null
priority: high
due_on: null
labels:
  - inbox
  - review
assignees: []
milestone: v0.1.0
parent: null
origin: null
dependencies: []
blocks_on: none
references:
  - ref: handoff:bootstrap
    path: handoffs/2026-10-01-vuoro-bootstrap.md
  - ref: plan:design-of-record
    path: docs/plan.md
claim: null
archive: null
created_at: 2026-10-01T16:24:07Z
updated_at: 2026-10-01T16:26:36Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
extensions: {}
---

## Description

The first release that does the job: one person, one window, every GitLab merge
request they review or wrote. Each one is sorted by whose turn it is, and the
review gets done in place. One agent action drafts and never posts.

GitHub and Forgejo are not in this epic. They are filed separately and build on
the `forge` interface this epic proves against GitLab.

docs/plan.md is the design of record. The handoff at
handoffs/2026-10-01-vuoro-bootstrap.md has the orientation.

## Acceptance criteria

- [ ] A GitLab review I am requested on appears in the inbox with the reason it is my turn
- [ ] A review I wrote shows as my turn when a reviewer replies, with the age
- [ ] I can comment inline, reply, resolve, approve, and submit a batch review without opening GitLab
- [ ] An agent can draft a review as pending comments, and nothing posts without my action
- [ ] just ci passes with CGO_ENABLED=0 and cross-builds linux, darwin, and windows
