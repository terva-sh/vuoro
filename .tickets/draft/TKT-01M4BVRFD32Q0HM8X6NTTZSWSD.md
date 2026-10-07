---
schema: 3
id: TKT-01M4BVRFD32Q0HM8X6NTTZSWSD
title: "Link each fix to its finding: trailer, then ask the editor, then by hand"
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - review
  - agent
assignees: []
milestone: null
parent: TKT-01M4BVRFAY1DN7317TACJXXXRA
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-07T18:59:34Z
updated_at: 2026-10-07T18:59:34Z
created_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
updated_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
extensions: {}
---

## Description

D50, first bullet. A fix is linked to the finding by the commit's trailer, then by asking the editor, then by hand.

- Before a push, compare the picked findings with the commits' Vuoro-Finding / Vuoro-Thread trailers. For each picked finding no commit names, send the editor one turn in its session asking it to amend with the trailer or say why the finding is left alone, and show the person the answer before the push.
- A route lets the person mark a finding fixed by one of the review's pushed commits (a commit vuoro pushed for this run, or one in the change since the reviewed head), recording FixedIn. No MCP tool reaches it; the drafts store's CiteFix stays vuoro's only writer.
- The run view carries each finding's FixedIn with the commit's subject.

## Acceptance criteria

- [ ] A push first asks the editor about each picked finding no commit's trailer names
- [ ] The person can mark a finding fixed by a commit of the review, and no MCP tool can
- [ ] A finding's fixed commit is recorded and shown with its subject
