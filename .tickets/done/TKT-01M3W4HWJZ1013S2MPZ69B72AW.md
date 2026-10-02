---
schema: 3
id: TKT-01M3W4HWJZ1013S2MPZ69B72AW
title: Decide the forge model and the turn rules
type: spike
status: done
status_reason: null
priority: high
due_on: null
labels:
  - forge
  - inbox
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-01T16:25:25Z
updated_at: 2026-10-02T18:57:48Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-mvp-1
  name: ""
extensions: {}
---

## Description

docs/plan.md section 4 is a starting position: change, role, review, thread,
draft, and turn, with turn rules for a reviewer and for an author. Test it
against real GitLab, GitHub, and Forgejo API responses before any adapter is
written. It is cheaper to find a forge that does not fit now than after the
first adapter has shaped the interface.

Questions to answer:

- Does each forge expose what the turn rules need: last review SHA per
  reviewer, note times per thread, re-requested reviews?
- How are a GitLab approval, a "changes requested" verdict, which GitLab does
  not have natively, and a dismissed review represented?
- Is "my last note in the thread" enough to decide an author's turn, or does
  a thread need to know who is expected to answer?
- What does `since` mean when several reasons apply?

`~/Workspace/code-reviews/tools/` holds working GitLab fetch code, in Python.
It is prior art for the API calls, and its responses are real shapes.

## Acceptance criteria

- [x] docs/plan.md section 4 is rewritten as the decided model, with the open-question marker removed
- [x] Each turn rule names the forge fields it reads, for GitLab, GitHub, and Forgejo
- [x] Recorded API fixtures for at least one GitLab change exist for the turn tests to use

## Implementation plan

Fetch real responses from public changes on gitlab.com, GitHub, and Codeberg (Forgejo), anonymously. Map each turn rule to the fields it reads. Rewrite plan section 4 as the decided model, and commit scrubbed GitLab fixtures for the turn tests.

## Notes

**agent:claude/vuoro-mvp-1** at 2026-10-02T18:57:48Z

Findings that shaped plan section 4, on 2026-10-02. A research agent fetched anonymously from public projects only.
- gitlab.com answers 401 to anonymous REST for MR discussions, notes, and versions. Anonymous GraphQL returns all of them, so GitLab reads go through GraphQL (D30).
- GitLab keeps each reviewer's current state only (UNREVIEWED, REVIEWED, REQUESTED_CHANGES, APPROVED), with no SHA and no history. Times come from system notes and mergeRequestDiffs.createdAt.
- GitHub has no push time in REST, so R2 uses commit_id != head.sha. A dismissed review keeps its original verdict only in the timeline. Thread resolution needs GraphQL with auth.
- Forgejo has real pull_push events, plus dismissed and stale flags, and REQUEST_REVIEW placeholder reviews. Replies have no parent ID.
- Bots (GitLabDuo, gitlab-bot, copilot...[bot]) appear as reviewers and as note authors, so they are filtered out.
- Fixtures: forge/gitlab/testdata/mr3958.json (merged) and mr3991.json (open, reviewer's turn after a push). Non-system note bodies are scrubbed. No emails, and no token was used.
- Not verified: whether a GitLab re-request resets the reviewer state to UNREVIEWED. Forgejo #4212 has dismissed:true with no dismiss event.
The raw responses for GitHub and Forgejo are in /tmp/vuoro-forge-research. They are not committed, because the REST files carry commit-author emails.

## Summary

Plan section 4 is the decided model (D30). Review requests and pushes join the model, bots and system notes count for nothing, a thread's turn is its last real note, and since is the earliest unanswered event. Each turn rule names its fields on GitLab, GitHub, and Forgejo. GitLab reads go through GraphQL. Scrubbed fixtures from two public gitlab-org/cli MRs are in forge/gitlab/testdata.
