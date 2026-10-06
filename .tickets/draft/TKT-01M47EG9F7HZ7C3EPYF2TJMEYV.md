---
schema: 3
id: TKT-01M47EG9F7HZ7C3EPYF2TJMEYV
title: "Push the editor's fixes to Forgejo: own branch, or a stacked draft PR"
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - forgejo
  - agent
  - review
assignees: []
milestone: v1.0.0
parent: TKT-01M47EFQ400T20P79H9X152NAZ
origin: null
dependencies:
  - TKT-01M47EG9BHW0HBVFBTSBB1915X
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-06T01:50:57Z
updated_at: 2026-10-06T02:00:31Z
created_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
updated_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
extensions: {}
---

## Description

On GitLab the editor's fixes go back through `forge.Stacker`. D39 puts a stacked draft merge request onto someone else's change. D41 and D42 push to the branch of the person's own change. Both paths call `StackTarget`, and `app/editor.go` refuses a Forgejo change outright ("GitLab only, so far").

Implement `forge.Stacker` for Forgejo:

- **`StackTarget`** reads the PR's head branch and SHA, and refuses with `forge.ErrFork` when the head repository is not the base repository.
- **`OpenStacked`** opens a draft PR from the pushed branch onto the reviewed PR's head branch, and returns its web URL.

Remove the GitLab-only refusal. Make the editor's wording forge-neutral where it says "merge request" (`review.StackTitle`, `StackDescription`, `threadsMessage`); `changeName` already uses `#` on Forgejo.

The push itself stays vuoro's own git push with the person's transport, and happens only at their confirmed action. The editor never holds a Forgejo token.

## Acceptance criteria

- [ ] Forgejo implements forge.Stacker, and StackTarget refuses a fork with ErrFork
- [ ] On my own Forgejo PR the editor's accepted fixes push to its branch
- [ ] On someone else's Forgejo PR the accepted fixes open a draft PR onto its head branch
- [ ] Both routes are validated live against a sandbox PR

## Notes

**agent:claude/forgejo-parity-20261006** at 2026-10-06T02:00:31Z

From the spike: POST /repos/{o}/{r}/pulls with head=<branch> and base=<reviewed PR's head.ref> and a 'WIP: ' title prefix returns draft: true ([WIP] works too). A fork PR has head.repo.full_name different from base.repo.full_name, and head.repo.fork true, so StackTarget refuses it with ErrFork. Sandbox for the live test: warricksothr/vuoro-sandbox, plus reanzu's fork reanzu/vuoro-sandbox for the fork case.
