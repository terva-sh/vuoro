---
schema: 3
id: TKT-01M5ZDQ3ZB0K6KMW2CB8NM183S
title: "Project triage: every open change in a project, and what holds each"
type: epic
status: ready
status_reason: null
priority: normal
due_on: null
labels:
  - ui
  - forge
assignees: []
milestone: v1.0.0
parent: TKT-01M3YMSTYMFRDFXD4QSVWHMM8P
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-27T19:34:40Z
updated_at: 2026-10-27T19:57:04Z
created_by:
  id: agent:claude-code/project-triage
  name: ""
updated_by:
  id: agent:claude-code/project-triage
  name: ""
extensions: {}
---

## Description

For a person who handles a project's incoming changes: point vuoro at one project and see every open merge or pull request in it, where each sits and for how long, then unblock them by setting reviewers, assignees, labels and draft state, with a read-only agent proposing what the person applies. Merging links out.

Design: docs/plan.md section 6.9. Decisions: D64 (the view, GitLab first, no Gitea) and D65 (the administrative writes, merging links out, agent proposes and the person applies).

## Acceptance criteria

- [ ] A GitLab project's open merge requests show where each sits, for how long, and why
- [ ] Reviewers, assignees, labels and draft state are set from the view on GitLab
- [ ] Agent proposals reach the forge only when the person applies them, and a test holds it
- [ ] GitHub and Forgejo reach the same view
