---
schema: 3
id: TKT-01M3YMSVEW8443QTX654QMJRX0
title: "Editor agent: fix my change from its threads and findings"
type: task
status: draft
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
claim: null
archive: null
created_at: 2026-10-02T15:47:52Z
updated_at: 2026-10-02T15:47:52Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-bootstrap
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
