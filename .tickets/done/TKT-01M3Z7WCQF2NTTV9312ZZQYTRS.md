---
schema: 3
id: TKT-01M3Z7WCQF2NTTV9312ZZQYTRS
title: Ask a reviewer for a suggestion block on a finding
type: task
status: done
status_reason: null
priority: high
due_on: null
labels:
  - gitlab
  - review
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-02T21:21:18Z
updated_at: 2026-10-02T21:45:33Z
created_by:
  id: agent:claude/vuoro-mvp-1
  name: ""
updated_by:
  id: agent:claude/vuoro-mvp-1
  name: ""
extensions: {}
---

## Description

D31. On a pending finding with a head-side anchor, the person asks for a
suggestion. vuoro asks the session that drafted it to revise the draft with
replacement text for exactly the anchored lines. The card shows the
suggestion, and keeping it writes a GitLab suggestion block, sized to the
range, into the comment. The person can keep it without the suggestion.

## Acceptance criteria

- [x] Asking for a suggestion gets replacement lines from the drafting session
- [x] A kept finding with a suggestion posts a suggestion block that covers its range, which a test proves
- [x] A suggestion is only ever on a head-side anchor, and the rule says so

## Implementation plan

Proposal and Draft gain an optional suggestion: the text replacing the anchor's lines at the head. Validation refuses one without a head-side anchor (suggestion.anchor). Manager.AskSuggestion sends the drafting session a request to revise with a suggestion, only for an undecided head-side finding; POST /api/runs/{id}/drafts/{did}/suggest is the person's click. keepBody appends drafts.SuggestionBlock, a suggestion:-(end-start)+0 block, since the comment lands on the anchor's last line; drafts.Fence keeps any fence longer than backtick runs inside, for evidence too. The UI mirrors keepText on a shared test vector, shows the suggestion, and the report carries it.

## Notes

**agent:claude/vuoro-mvp-1** at 2026-10-02T21:45:33Z

Tested with the fake driver (the drafting session receives the request naming the range and revises with a suggestion; a base-side or kept draft is refused), the block's offsets and fence escaping, and keepBody against the same vector as the UI's keepText. Not yet seen live: no real model has written a suggestion, and GitLab's rendering of a multi-line suggestion block from a draft note has not been checked against gitlab.com. Fixed on the way: evidence holding three backticks broke out of its fence in the kept comment.

## Summary

The person can ask the drafting session for a suggestion on an undecided head-side finding; keeping it posts a GitLab suggestion block over the finding's whole range, which the author can apply. The hand-off report carries it too.
