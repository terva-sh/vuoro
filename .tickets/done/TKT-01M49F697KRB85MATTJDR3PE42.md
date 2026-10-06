---
schema: 3
id: TKT-01M49F697KRB85MATTJDR3PE42
title: Keep local projects and scan roots, and list each one's branches
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - state
  - review
assignees: []
milestone: null
parent: TKT-01M49F68YV694W81S3ESJKVH02
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-06T20:41:27Z
updated_at: 2026-10-06T21:01:11Z
created_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
updated_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
extensions: {}
---

## Description

Section 6.8: local.json in the state directory holds added projects, scan roots, hidden projects, and each project's base. A bounded scan finds checkouts under roots. GET /api/local lists projects keyed by git common directory, each branch with commits beyond the base, its worktrees and their dirty mark, its last run, and whether it needs the person. git runs read-only with hooks, fsmonitor, external diff, and textconv off.

## Acceptance criteria

- [x] Projects can be added, removed, and hidden, and roots added and removed, through routes behind the guard; paths must be absolute
- [x] The scan honours depth 4, no symlinks, skipped directories, 500 checkouts, and ten seconds, and reports when it stopped
- [x] A main checkout and its worktrees are one project
- [x] Each branch row carries ahead count, newest commit age, worktree tags with a dirty mark, its last run, and why it needs the person
- [x] Tests run against temporary repositories, with no network

## Summary

A new package, `local`, reads checkouts and nothing else: the store (local.json), a bounded scan, and Inspect, which reads a project's common directory, worktrees with a dirty mark, base, branches beyond the base, and vuoro's fix branches. The boundary test now holds that `local` reaches no forge, agent, review runner, API, or net/http, and the plan's package table lists it.

`app/local.go` serves GET /api/local, POST /api/local/refresh, and edits for projects, roots, hide, and base. Hide and base accept only a project the built view lists, so no route runs git in a folder named by key alone. A scan stands for an hour unless the roots change or Refresh asks; a built view is served for a minute. A run is matched to a project by its recorded checkout (Run.Origin, a new accessor), and to a branch when the branch contains the reviewed head and the base does not. A branch's needs: never reviewed, moved since its review, undecided findings, a local fix branch not yet merged into it.

On the owner's ~/Workspace: 60 checkouts scanned in about 20 ms, and 49 projects with 118 branch rows built in 1.9 s, after counting ahead with for-each-ref's ahead-behind (git 2.41+). An older git counts one branch at a time; that fallback is not exercised by the tests, which run on 2.50.

Open question for the owner: every branch that was never reviewed counts as needing the person, so the owner's tab would read 118. The Local view slice should decide whether old, never-reviewed branches count.
