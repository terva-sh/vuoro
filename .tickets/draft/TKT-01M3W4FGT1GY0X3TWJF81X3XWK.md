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
updated_at: 2026-10-03T02:01:28Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:codex/windows-roadmap-20261002
  name: ""
extensions: {}
---

## Description

The first usable vuoro, for its owner on macOS. It is built orchestrator first:
open a GitLab merge request by URL, run several reviewer sessions in parallel
on Claude Code, let a synthesizer session merge their
drafts, talk to any of them, edit the drafts, and submit the review. The
whose-turn inbox lands second and feeds the orchestrator.

The decisions behind this scope were made with the owner on 2026-10-02 and are
recorded in docs/plan.md section 10. v1.0.0 is a separate epic.

terva is not an MVP reviewer: decision D28, after the driver spike found that
terva 0.139.7 cannot be locked down. It moves to the v1.0.0 epic.

## Acceptance criteria

- [ ] A GitLab review I am requested on appears in the inbox with the reason it is my turn
- [ ] A review I wrote shows as my turn when a reviewer replies, with the age
- [ ] I can comment inline, reply, resolve, approve, and submit a batch review without opening GitLab
- [ ] just ci passes with CGO_ENABLED=0 and cross-builds linux, darwin, and windows
- [ ] A synthesizer session merges the reviewers' drafts and keeps each finding's sources
- [ ] I can talk to any reviewer while it works, and nothing posts without my action
- [ ] Every session shows its tool, model, tokens, and cost
- [ ] I can open a GitLab merge request by URL and run several Claude Code reviewers on it in parallel
- [ ] GitLab and Forgejo authored/requested changes appear in one inbox with correct turn, reason and earliest-unanswered age.

## Notes

**agent:codex/windows-roadmap-20261002** at 2026-10-03T02:01:28Z

Owner-selected priority: one GitLab-and-Forgejo inbox ordered by whose turn it is. D34 records that outcome and moves Forgejo discovery/inbox into the MVP while writes remain later. The executable backlog is shared discovery/refresh (TKT-01M3ZQXC5CPRDM3CD937C6PC3Q), the existing pure turn ticket, both adapters (new Forgejo slice TKT-01M3ZQXC6SWB7M9D46AN0QJAP8), then the mixed inbox. Structured dependencies connect the slices. The new read-only Forgejo task does not inherit blocked GitLab live-write validation. The broader Forgejo ticket now tracks only writes.

All implementation tickets remain draft. docs/roadmap.md maps the existing implementation, completion gates and subsequent MVP/v1.0 work; docs/plan.md remains authoritative. Considered alternatives were Forgejo posting first and more agent workflows first; the owner chose the inbox because it supplies the missing whose-turn workflow across both existing readers. Older relocation/publication/usage tickets are flagged for evidence-based reconciliation, not silently closed, and the earlier adoption claim is left untouched.
