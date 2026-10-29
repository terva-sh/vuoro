---
schema: 3
id: TKT-01M63Z7G86D39B7PXG7XDJVRHD
title: "Spike: a practice review on a generated local repository"
type: spike
status: draft
status_reason: null
priority: low
due_on: null
labels:
  - sandbox
  - agent
  - review
assignees: []
milestone: v1.0.0
parent: TKT-01M63Z778B87DTP765W8JWSMPN
origin: null
dependencies:
  - TKT-01M63Z7G66X679ZJEHFAGK7FD1
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-29T13:57:41Z
updated_at: 2026-10-29T13:57:41Z
created_by:
  id: agent:claude-code/getting-started-tutorial
  name: ""
updated_by:
  id: agent:claude-code/getting-started-tutorial
  name: ""
extensions: {}
---

## Description

The deepest way to learn the window is to run a real review. A forge change is the wrong place to practise, so this spike decides whether vuoro can generate a small git repository with a planted bug in its state directory and offer it on the Local tab as a practice review, exercising the reviewers, the synthesizer, and the drafts with no forge involved.

Questions to answer:

- Cost: what the cheapest preset costs on the practice repository, and how the offer shows that before anything starts. It must be opt-in.
- Absence: what the offer says when no agent tool is installed or signed in.
- The end: the practice stops at the drafts. Submit has nothing to post to and must not pretend to. Decide what the last step shows instead.
- Lifetime: where the repository lives, how it is removed, and that it never appears as one of the person's own projects after the practice ends.
- Whether levels 1 and 2 already make this unnecessary.

## Acceptance criteria

- [ ] A written answer to each question in the description, as a note on this ticket
- [ ] A recommendation to build, defer, or drop it, for the owner to decide
