---
schema: 3
id: TKT-01M63YYXR9C2DKP4619KT9Q6KP
title: Test a change in the editor's sandbox, with nothing to push
type: task
status: ready
status_reason: null
priority: normal
due_on: null
labels:
  - agent
  - sandbox
  - review
assignees: []
milestone: v1.0.0
parent: TKT-01M61JPA9TYM392M8XBAZ6RQV4
origin: null
dependencies:
  - TKT-01M61JQ22E7049NSPR545323DW
  - TKT-01M61JQ2FJ4QGFZWPJE2456FQB
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-29T13:53:00Z
updated_at: 2026-10-29T13:59:18Z
created_by:
  id: agent:claude-code/ask-plan
  name: ""
updated_by:
  id: agent:claude-code/ask-plan
  name: ""
extensions: {}
---

## Description

A run of mode `probe`: a conversation about a change answered by an agent in the editor's sandbox (D40), which may build and run commands and has nothing to push. Its repository is its own, with no forge remote, holding the change's head checked out and the base commit so the agent can compare the two. It may change files and commit inside that repository; vuoro offers no push, stacked change, or suggestion from it, and the AST guard proves no push route accepts a probe. It gets the `context` server of the Ask epic. Follow-ups and resume work as for an ask run; Stop keeps what was written; elapsed time is shown.

The sandbox starts as the editor's is and is adapted in use (section 6.12, D69). Network policy, runtime (Claude Code's sandbox, srt, later a container), and workspace retention stay separate seams, each chosen in one place and recorded on the run, so running probes on real changes can change one without a rewrite. A refusal by the sandbox is reported as such, so a sandbox limit is not mistaken for a test failure.

The case that raised it: a dependency bump whose diff is almost all uv.lock, where the question before merging is what the new versions break.

Design: docs/plan.md section 6.12, D69.

## Acceptance criteria

- [ ] A probe runs in the editor's sandbox with no forge credential, and a test proves it
- [ ] Its repository holds the head checked out and the base commit, with no forge remote
- [ ] No route pushes, stacks, or suggests from a probe, and the AST guard proves it
- [ ] The person follows up and stops it as in an ask run, and it resumes after a restart
- [ ] Network policy, runtime, and workspace retention are each chosen in one place and recorded on the run
- [ ] A probe the sandbox refused says what was refused, apart from a failing test
