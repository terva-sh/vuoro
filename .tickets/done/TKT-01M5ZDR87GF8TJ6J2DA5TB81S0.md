---
schema: 3
id: TKT-01M5ZDR87GF8TJ6J2DA5TB81S0
title: "Project triage on GitHub: read a repository's PRs and set them up"
type: task
status: done
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
updated_at: 2026-10-27T20:57:16Z
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

- [x] A GitHub repository is added by search or URL and all its open pull requests are read
- [x] Reviewers, assignees, labels and draft state are set through forge.Administrator
- [ ] An opt-in live test round-trips each write on a sandbox pull request

## Implementation plan

Read path: add labels/files connections, mergeable, headRepository and check rollup to the read query; team review requests become Change.Teams. ProjectLister through GraphQL (search, viewer repositories, pullRequests(states: OPEN)). Administrator through additive REST endpoints, with GraphQL for draft state; candidates are assignableUsers. TriageReader for CODEOWNERS and commit authors. Teams make a pull request wait on review in the project view. Unit tests on a fake GitHub, and an opt-in live test.

## Notes

**agent:claude-code/project-triage** at 2026-10-27T20:57:16Z

The opt-in live test TestLiveAdminister (forge/github/admin_live_test.go, VUORO_LIVE_GITHUB_ADMIN_PR plus VUORO_LIVE_GITHUB_TOKEN) is written but has NOT been run: the machine this was built on has no GitHub account in vuoro's config and no gh. Criterion 3 stays unticked until someone runs it against a sandbox pull request. To round-trip reviewers it also needs VUORO_LIVE_GITHUB_REVIEWER, because GitHub refuses a review request from an author to themselves.

## Summary

GitHub repositories are added by search (search REPOSITORY, or the viewer's repositories when the query is empty) or by URL. ListOpen reads every open pull request by cursor and refuses a repeat. Facts now carry labels, touched files, conflicts (mergeable), checks (statusCheckRollup), the fork source and requested teams. forge.Administrator uses additive REST endpoints for reviewers, assignees and labels; a dropped assignee is detected; draft state goes through convertPullRequestToDraft and markPullRequestReadyForReview. Members are assignableUsers. CODEOWNERS and recent commit authors feed agent triage. The decision on teams: a pull request whose only request is to a team waits on review with the team as holder, and the inbox is unchanged. The fixtures gained the new fields by hand, as their README says. The live test is written but not yet run (see the note).
