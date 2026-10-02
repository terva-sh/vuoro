---
schema: 3
id: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
title: "vuoro v0.1.0 (MVP): orchestrated GitLab reviews, then the inbox"
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
  - ref: plan:design-of-record
    path: docs/plan.md
  - ref: handoff:first-session
    path: handoffs/2026-10-02-vuoro-handoff.md
  - ref: doc:decisions
    path: docs/decisions.md
  - ref: doc:research
    path: docs/research.md
claim: null
archive: null
created_at: 2026-10-01T16:24:07Z
updated_at: 2026-10-02T15:57:57Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
extensions: {}
---

## Description

The first usable vuoro, for its owner on macOS. It is built orchestrator first:
open a GitLab merge request by URL, run several reviewer sessions in parallel
on any mix of terva and Claude Code, let a synthesizer session merge their
drafts, talk to any of them, edit the drafts, and submit the review. The
whose-turn inbox lands second and feeds the orchestrator.

The decisions behind this scope were made with the owner on 2026-10-02 and are
recorded in docs/plan.md section 10. v1.0.0 is a separate epic.

## Acceptance criteria

- [ ] A GitLab review I am requested on appears in the inbox with the reason it is my turn
- [ ] A review I wrote shows as my turn when a reviewer replies, with the age
- [ ] I can comment inline, reply, resolve, approve, and submit a batch review without opening GitLab
- [ ] just ci passes with CGO_ENABLED=0 and cross-builds linux, darwin, and windows
- [ ] I can open a GitLab merge request by URL and run several reviewers on it in parallel, on terva, Claude Code, or both
- [ ] A synthesizer session merges the reviewers' drafts and keeps each finding's sources
- [ ] I can talk to any reviewer while it works, and nothing posts without my action
- [ ] Every session shows its tool, model, tokens, and cost
