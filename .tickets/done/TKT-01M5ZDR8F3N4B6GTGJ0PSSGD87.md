---
schema: 3
id: TKT-01M5ZDR8F3N4B6GTGJ0PSSGD87
title: "Project triage on Forgejo: read a repository's PRs and set them up"
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - forgejo
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
updated_at: 2026-10-27T21:02:37Z
created_by:
  id: agent:claude-code/project-triage
  name: ""
updated_by:
  id: agent:claude-code/project-triage
  name: ""
extensions: {}
---

## Description

Bring project triage to Forgejo through the contracts the GitLab slice adds (plan section 6.9, D64): search the repositories a configured account can see or take a pasted URL, list every open pull request with its status, conflicts, labels and a stack read from base branches, and implement `forge.Administrator` for requested reviewers, assignees, labels, and draft state, which Forgejo carries in a `WIP:` title prefix (section 5.3). Pagination follows the adapter's existing rules. Gitea is not a target (D64).

## Acceptance criteria

- [x] A Forgejo repository is added by search or URL and all its open pull requests are read
- [x] Reviewers, assignees, labels and draft state are set through forge.Administrator
- [ ] An opt-in live test round-trips each write on a sandbox pull request

## Implementation plan

Read path: labels, head repository, files list (now for facts too) and the combined commit status. ProjectLister through repos/search, repos/O/R and the paged pulls?state=open. Administrator: reviewers through requested_reviewers; labels by ID; assignees and the WIP title prefix read just before a whole-field PATCH. Candidates are assignees, filtered by vuoro. TriageReader for raw CODEOWNERS and commit authors. Unit tests on a fake Forgejo, and an opt-in live test through tea.

## Notes

**agent:claude-code/project-triage** at 2026-10-27T21:02:37Z

Two things for the next person. (1) The opt-in live test TestLiveAdminister (forge/forgejo/admin_live_test.go: VUORO_LIVE_FORGEJO_ADMIN_PR, VUORO_LIVE_FORGEJO_AUTHOR, VUORO_LIVE_FORGEJO_LABEL, and optionally VUORO_LIVE_FORGEJO_REVIEWER) is written but has NOT been run: the machine has no tea and no Forgejo account in vuoro's config. Criterion 3 stays unticked until it runs on a sandbox. (2) Forgejo has no additive endpoint for assignees, and the draft state is the title, so both are read just before a whole-field PATCH. A colleague's edit made between that read and the write can still be lost. docs/plan.md section 6.9 records this; the owner may want to confirm it is acceptable under D65.

## Summary

Forgejo repositories are added by search or URL, and every open pull request is listed with labels, touched files, the fork source and the combined commit status. Conflicts stay unknown, because mergeable is also false for a draft and while Forgejo checks. forge.Administrator: reviewers through requested_reviewers (add and remove); labels by ID; assignees and draft state (the WIP title prefix) read just before a whole-field write, the closest Forgejo's API allows to D65. Members are the repository's assignees. CODEOWNERS and commit authors feed agent triage. The live test is written but not run (see the note).
