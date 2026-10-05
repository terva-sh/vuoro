---
schema: 3
id: TKT-01M46QZPBQCN6T9QJBBZ8A894P
title: Keep pressed twice posts the finding twice
type: bug
status: done
status_reason: null
priority: high
due_on: null
labels:
  - review
  - ui
  - gitlab
assignees: []
milestone: null
parent: null
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-05T19:17:24Z
updated_at: 2026-10-05T19:24:37Z
created_by:
  id: agent:claude-code/keep-collapse-20261005
  name: ""
updated_by:
  id: agent:claude-code/keep-collapse-20261005
  name: ""
extensions: {}
---

## Description

On terraform!2487 the owner pressed Keep on a finding more than once while the first request was still running, and got duplicate pending comments on GitLab. The button stays enabled during the request. On the server, `keep` checks that the draft is undecided, writes to the forge, and only then records the decision, so two requests that arrive together both pass the check and both write.

## Acceptance criteria

- [x] A second Keep on a draft whose first Keep is still running is refused, and only one pending comment is written
- [x] The Keep button shows that it is working and cannot be pressed again until the first Keep finishes

## Summary

Store.Keep holds a draft while its forge write runs. A second Keep, a drop, or an agent's revision or suggestion is refused until it settles, and a failed write releases it. The Keep and Drop buttons show progress and stay off until the run shows the decision.
