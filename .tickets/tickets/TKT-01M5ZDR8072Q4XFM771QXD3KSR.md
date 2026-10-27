---
schema: 3
id: TKT-01M5ZDR8072Q4XFM771QXD3KSR
title: Agent triage drafts reviewer and label proposals; the person accepts
type: task
status: ready
status_reason: null
priority: normal
due_on: null
labels:
  - agent
  - review
  - ui
assignees: []
milestone: v1.0.0
parent: TKT-01M5ZDQ3ZB0K6KMW2CB8NM183S
origin: null
dependencies:
  - TKT-01M5ZDR7S4AHCKVQBD2ZEEHGJ2
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-27T19:35:17Z
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

Reviewer-class triage over a project (plan sections 6.4 and 6.9, D65). One read-only session gets, as data from vuoro, each open change's touched paths and size, CODEOWNERS at the target branch, recent committers to those paths, and each candidate's open review load in the project. It drafts proposals for reviewers, labels, and a line saying what would unblock each change, through the drafts MCP server. Each proposal is bound to a change and a field in the run; one naming a change or a person outside the run is refused.

Proposals are drafts in the view. The person edits, accepts or discards each, or accepts several after one confirmation. Accepting turns a proposal into a fixed list of writes that vuoro's code performs through `forge.Administrator`. No agent session runs during the apply. A proposal whose change has moved since the agent read it, by a push or by someone setting the same field, is marked stale and waits for the person. The lockdown is the reviewer's: no shell, no web, none of the person's MCP servers. Change descriptions and repository text are untrusted data.

## Acceptance criteria

- [ ] A triage session runs with the reviewer lockdown, and its proposals land as drafts only
- [ ] A proposal naming a change or person outside the run is refused
- [ ] No proposal reaches the forge without the person accepting it, and a test holds it
- [ ] An accepted proposal is carried out by vuoro's code through forge.Administrator, with no agent session running
- [ ] Accepting several proposals takes one confirmation, and a proposal whose change moved is marked stale, not applied
