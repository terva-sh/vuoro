---
schema: 3
id: TKT-01M46TYPS9WFBT865TP52ZH46X
title: Push the editor's fixes and open a draft MR onto the reviewed MR
type: task
status: done
status_reason: null
priority: high
due_on: null
labels:
  - agent
  - review
  - gitlab
assignees: []
milestone: v1.0.0
parent: null
origin: null
dependencies:
  - TKT-01M46TYPQVEZGMJBTC37KPR3P9
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-05T20:09:18Z
updated_at: 2026-10-05T22:11:29Z
created_by:
  id: agent:claude-code/editor-20261005
  name: ""
updated_by:
  id: agent:claude-code/editor-20261005
  name: ""
extensions: {}
---

## Description

D39 and docs/plan.md section 6.3 cover this. Once the person has reviewed the editor's commits, one confirmed action pushes the fix branch to the change's project with the person's credential (never in argv) and opens a Draft merge request onto the original merge request's source branch. The title and description can be edited first. The description lists each finding addressed, with its commit, and links the original.

## Acceptance criteria

- [x] Push and open asks first, with the branch, the target, and an editable title and description
- [x] It pushes with the person's credential kept out of argv, logs, and errors, and opens a Draft MR onto the original source branch
- [x] A change whose source branch is in a fork is refused with the reason
- [x] No code path pushes or opens a merge request without the person's confirmation, and a test holds it

## Implementation plan

- forge: optional `Stacker` interface. `StackTarget(ref)` reads where a stacked change goes (the source branch and head) and refuses a fork; `OpenStacked(ref, NewStacked)` opens a Draft change onto it. GitLab implements both over REST; Forgejo returns ErrUnsupported for now.
- review: `Repos.PushEditor` pushes `refs/vuoro/editor/<run>` from vuoro's clone to `refs/heads/<branch>` over the person's git transport (the remote vuoro fetches from), without force, with hooks off. `Manager.PushEditor` fetches the editor's branch, checks that it descends from the reviewed head and has commits, pushes, and records `EditorState.Pushed`. `StackDescription` lists each commit with the finding it answers, the findings left alone, and links the original.
- app: `GET /api/runs/{id}/editor/stack` previews the title, description, branch, target and files. `POST /api/runs/{id}/editor/push` requires `confirm: "push"`, checks the change is at the reviewed head and not a fork, pushes, opens the Draft MR once, and records it. A second push after more editor commits pushes again and reuses the MR.
- web: a confirm dialog with the branch, target, files (CI configuration flagged), and editable title and description; after pushing, a link to the Draft MR.
- tests: the route refuses without the confirmation, a fork is refused, the fake forge sees exactly one push and one MR, and no other route pushes.

## Notes

**agent:claude-code/editor-20261005** at 2026-10-05T22:11:29Z

Built: forge.Stacker (GitLab REST; Forgejo unsupported), Repos.PushEditor / Manager.PushEditor (push the reviewed sha only, no force, over the person's transport, must descend from the reviewed head), StackTitle/StackDescription, the preview and push routes (confirm "push" required), and the push dialog. Tests: GitLab stacker incl. fork, review push to a local remote (wrong sha refused, MR opened once, second push reuses it), app refusals (no confirmation, fork), an AST test that only pushEditor calls PushEditor or OpenStacked, and the dialog. Not yet run against gitlab.com; the first real push is the owner's, on !2487.

## Summary

Push and open a draft merge request asks first with the branch, target, files, and editable title and description, then pushes the commit the person saw over their own git access and opens a GitLab Draft MR onto the reviewed MR's source branch. Forks are refused. Only the confirmed route pushes, which a test holds.
