---
schema: 3
id: TKT-01M3YMSVEW8443QTX654QMJRX0
title: "Editor agent: fix my change from its threads and findings"
type: task
status: in-progress
status_reason: null
priority: normal
due_on: null
labels:
  - agent
  - sandbox
assignees: []
milestone: v1.0.0
parent: TKT-01M3YMSTYMFRDFXD4QSVWHMM8P
origin: null
dependencies:
  - TKT-01M3YMSVD4Y1ME5QST9BY818XD
  - TKT-01M3YMSSYSBT0X4SMB712EA0PQ
  - TKT-01M3YK0CPYG132897XVVZ9G4EF
blocks_on: none
references: []
claim:
  actor: agent:claude-code/editor-20261005
  branch: tickets/author-editor
  worktree: /Users/drewshort/Workspace/github.com/terva-sh/vuoro
  commit: aa32c5d64060af38dea1e7361bb764b4ee030637
  session: null
  claimed_at: 2026-10-05T23:48:36Z
  expires_at: null
archive: null
created_at: 2026-10-02T15:47:52Z
updated_at: 2026-10-05T23:53:36Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude-code/editor-20261005
  name: ""
extensions: {}
---

## Description

A separate class of agent, and the only one that writes code. Each review gets
at most one editor, so parallelism is never a question for writes. On the
person's own change, the editor takes the triaged threads and the reviewers'
findings as its priors, and it works in the sandbox.

It commits on a local branch, one commit per finding or thread it addresses,
and each message names what it answers. vuoro shows its commits for review like
any change. The person accepts, and vuoro pushes to the merge request branch on
that action, or the person discards them. The editor never holds forge
credentials, and it never pushes.

Reviewer-side use, which turns a finding into a suggested patch on somebody
else's change, comes after v1.0.0.

## Acceptance criteria

- [ ] The editor addresses selected threads with one commit each, on a local branch
- [ ] I can review the editor's commits as a diff before anything is pushed
- [ ] Accepting pushes to the merge request branch, and discarding leaves the forge untouched
- [ ] The editor's environment has no forge credential, and a test proves it

## Implementation plan

D41. The editor generalizes from kept findings to triaged threads.

- review: `StartEditor` takes the items to fix. A review run's items are its kept findings; a triage run's are the pending thread drafts the person picked, checked against a fresh forge read like CheckReply. The prompt and first message name each item's id; commits carry `Vuoro-Finding: dN` or `Vuoro-Thread: dN`. `PushEditor` gains a mode that pushes to the change's own source branch, refuses when that branch's head is not the editor's base, and opens nothing. After the push, each thread draft named by a pushed commit gets `FixedIn` and, unless the person edited it and unless already accepted, the reply "Fixed in `<sha8>`: <subject>".
- app: the editor routes accept a triage run when the person is the change's author, with `items`; the preview and push say "your branch" and skip the merge request.
- web: the Threads tab gets a checkbox per triaged thread (must fix ticked), "Fix the picked threads in a branch", and the same Editor panel below; a reply shows "Fixed in" once pushed.
- tests: review (triage editor start refusals, own-branch push, refused when moved, reply citation and its respect for edits), app (author-only), web.

## Notes

**agent:claude-code/editor-20261005** at 2026-10-05T23:53:36Z

Built and tested (review, app, web): triaged-thread picks, the own-change editor, fast-forward push to the source branch refused when moved, replies citing pushed commits (person-edited replies stand; the agent can no longer revise a fixed reply). Not yet run on a real change; next-gen-compute!144 is the candidate. The owner raised the three flows (reviewing others, answering your own threads, reviewing your own work to dogfood) mid-build; the third is not covered yet.
