---
schema: 3
id: TKT-01M5ZDR87GF8TJ6J2DA5TB81S0
title: "Project triage on GitHub: read a repository's PRs and set them up"
type: task
status: ready
status_reason: null
priority: normal
due_on: null
labels:
  - github
  - forge
assignees: []
milestone: v1.0.0
parent: TKT-01M5ZDQ3ZB0K6KMW2CB8NM183S
origin: null
dependencies:
  - TKT-01M5ZDR7S4AHCKVQBD2ZEEHGJ2
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-27T19:35:18Z
updated_at: 2026-10-27T19:57:04Z
created_by:
  id: agent:claude-code/project-triage
  name: ""
updated_by:
  id: agent:claude-code/project-triage
  name: ""
extensions: {}
---

## Description

Bring project triage to GitHub through the contracts the GitLab slice adds (plan section 6.9, D64): search the repositories a configured account can see or take a pasted URL, list every open pull request with checks, conflicts, labels and a stack read from base branches, and implement `forge.Administrator`: requested reviewers, assignees, labels, and draft or ready (`convertPullRequestToDraft`, `markPullRequestReadyForReview`). Team review requests name nobody today (section 5.4); decide how the view shows them. A search over GitHub's result cap is refused, not read in part.

## Acceptance criteria

- [ ] A GitHub repository is added by search or URL and all its open pull requests are read
- [ ] Reviewers, assignees, labels and draft state are set through forge.Administrator
- [ ] An opt-in live test round-trips each write on a sandbox pull request
