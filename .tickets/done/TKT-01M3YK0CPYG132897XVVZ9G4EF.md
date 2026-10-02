---
schema: 3
id: TKT-01M3YK0CPYG132897XVVZ9G4EF
title: Run reviewers in parallel, and synthesize their drafts
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - agent
  - review
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies:
  - TKT-01M3YK0CM0SWMYXMM7A7M7J2QX
  - TKT-01M3YK0CNGAX0M1Y755995W7QN
  - TKT-01M3YMSSRC81M0BYHK15TW9VDY
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-02T15:16:29Z
updated_at: 2026-10-02T19:56:49Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-mvp-1
  name: ""
extensions: {}
---

## Description

Moved into the MVP on 2026-10-02: vuoro is an orchestrator, and this is the
orchestration. A preset names several reviewers, each a tool, a model, and a
profile, and they run as parallel sessions on one change, each in a pane the
person can talk to. When they finish, a synthesizer session lists every
reviewer's drafts through the drafts MCP server. It proposes merged drafts that
name their sources, and it says what it set aside and why.

Prior art: the review action's panels, and terva's raati. A finding that only
one reviewer raised is kept and labelled. It is not dropped by a vote.

## Acceptance criteria

- [ ] A preset of three reviewers on two drivers runs in parallel on one change
- [x] The synthesizer's merged drafts name their sources, and set-aside findings are listed with a reason
- [ ] A finding only one reviewer raised is kept and labelled

## Implementation plan

A run with more than one reviewer gets a synthesizer session, waiting until every reviewer's first turn ends. It then reads the change and every draft through list_drafts, proposes merged drafts that name their sources, and ends its reply with a Set aside section. The run view shows merged drafts first, each saying where it came from, and marks each reviewer draft as merged or not carried forward.

## Notes

**agent:claude/vuoro-mvp-1** at 2026-10-02T19:56:49Z

Live, 2026-10-02, on gitlab-org/cli!3991: three Haiku reviewers (code,
security, documentation) ran in parallel in about four minutes, and the
synthesizer waited for all three. None drafted anything on this small MR. Each
of two reviewers was then asked to draft an overlapping finding, and one of
them also drafted a CHANGELOG finding. Asked to merge again, the synthesizer
merged d1 and d2 into d4 with both as sources. It set d3 aside with a reason
that it checked in CHANGELOG.md: the changelog is generated. Total cost was
$0.77.

Criterion 1 says "on two drivers". Only Claude Code can be offered (D28), so
three reviewers on one driver is what was run. The box stays empty.

Criterion 3 is half proven, so the box stays empty. The label is held by a
unit test (sourceLabel), and the prompt tells the synthesizer to keep a
single-reviewer finding. But no live run has yet shown it keeping one: the
only singleton it saw was wrong, and it was set aside with a reason.

Set-aside reasons live in the synthesizer's closing reply, not in a
structured field, because the drafts server's four tools are fixed and
tested. Listing them as data needs a fifth tool, which is a decision for the
owner.

## Summary

Reviewers run in parallel, and a synthesizer merges their drafts with sources and lists what it set aside. Proven with a fake-driver test and live on Claude Code. Criteria 1 and 3 are left open, per the note.
