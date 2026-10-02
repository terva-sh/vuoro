---
schema: 3
id: TKT-01M3YMSTYMFRDFXD4QSVWHMM8P
title: "vuoro v1.0.0: every forge, the editor agent, and a terva-sh release"
type: epic
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - review
  - inbox
assignees: []
milestone: v1.0.0
parent: null
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-02T15:47:51Z
updated_at: 2026-10-02T15:47:51Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
extensions: {}
---

## Description

The aspirational release, scoped with the owner on 2026-10-02 (docs/plan.md
section 10): GitLab, Forgejo, and GitHub; the editor agent, which fixes the
person's own change from its threads and the reviewers' findings inside a
sandbox; tray notifications; local branch review; budget caps; and a public
terva-sh release with setup in the UI and documentation for somebody new.

## Acceptance criteria

- [ ] GitLab, Forgejo, and GitHub changes share one inbox and one orchestrator
- [ ] The editor agent fixes my change in a sandbox, and I review its commits before vuoro pushes them
- [ ] vuoro notifies me from the tray when it becomes my turn
- [ ] Someone who is not the owner installs a release and sets vuoro up without editing a file
