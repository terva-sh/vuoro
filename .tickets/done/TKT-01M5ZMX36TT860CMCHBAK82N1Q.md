---
schema: 3
id: TKT-01M5ZMX36TT860CMCHBAK82N1Q
title: Update a clean branch through the forge's own branch update
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - forge
  - gitlab
  - github
  - forgejo
assignees: []
milestone: v1.0.0
parent: TKT-01M5ZMW481E943MKBJ7FSJWNCJ
origin: null
dependencies:
  - TKT-01M5ZMX2P51GCH3KM5VFP755CX
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-27T21:40:16Z
updated_at: 2026-10-27T22:06:54Z
created_by:
  id: agent:claude-code/branch-update
  name: ""
updated_by:
  id: agent:claude-code/branch-update
  name: ""
extensions: {}
---

## Description

A clean update on the person's own change goes through the forge's own branch update when it has one for the strategy, against the expected head (plan section 6.10). `forge.BranchUpdater` sits behind the capability flag `update_branch`. GitLab rebases with `PUT /merge_requests/:iid/rebase` and has no server-side merge of the target. GitHub uses `PUT /pulls/N/update-branch` with `expected_head_sha`. Forgejo uses `POST /pulls/N/update?style=merge|rebase`. A strategy the forge lacks falls back to vuoro's push.

## Acceptance criteria

- [x] GitLab rebases, and GitHub and Forgejo merge or rebase, through the forge for a clean update
- [x] The update is sent against the head the person saw, and a moved head is refused

## Implementation plan

forge.BranchUpdater.UpdateBranch(ctx, ref, strategy, head) per forge. GitHub merges through REST with expected_head_sha and rebases through the GraphQL updatePullRequestBranch with expectedHeadOid. GitLab and Forgejo read the head first, then GitLab runs PUT /rebase and polls until it finishes, and Forgejo runs POST /update?style=. Capabilities.UpdateBranch lists the strategies. Manager.PushUpdate takes viaForge for a clean update on the person's own change, and falls back to the push on ErrUnsupported.

## Summary

forge/{gitlab,github,forgejo}/update.go, with fixture tests. The push route passes the forge update when the change's capabilities list the strategy, and the push dialog says the forge writes its own commits. The AST test holds that only pushUpdate calls UpdateBranch. Not exercised against a live forge, because a live test would write a branch.
