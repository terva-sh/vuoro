---
schema: 3
id: TKT-01M4D283WTGMG60THDTWD3X59Y
title: Verify GitHub approve and request changes live from a second account
type: task
status: draft
status_reason: null
priority: low
due_on: null
labels:
  - forge
  - github
assignees: []
milestone: null
parent: TKT-01M3YMSTYMFRDFXD4QSVWHMM8P
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-08T06:12:13Z
updated_at: 2026-10-08T06:12:13Z
created_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
updated_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
extensions: {}
---

## Description

TKT-01M3W4HX30MB0D601R3S0ZXSER (GitHub adapter for the forge interface) built approving and requesting changes on GitHub, and they are fixture-tested in `forge/github/write_test.go`. Live, they were checked only as far as GitHub's refusal: on the warricksothr sandbox the person is the author, and GitHub refuses an author's approval or request for changes ("Review Can not approve your own pull request"). `TestLiveWrite` checks that refusal.

What a live run still has to show, from a second GitHub account that is a reviewer on the sandbox pull request:
- `Approve(true)` publishes an approval, and a second call returns nil, because the approval already stands on the head.
- `Submit(VerdictChangesRequested)` with pending comments publishes them under a request for changes.
- `Submit(VerdictApproved)` with no content publishes an approval alone.
- After a push, `ReadFacts` shows the approval as on an older head (turn rule R2).

That needs a second account and its token, which the owner holds (D54).

## Acceptance criteria

- [ ] Approve, request changes, and a verdict-only submit are checked live on a sandbox pull request from a reviewer account
