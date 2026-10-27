---
schema: 3
id: TKT-01M5ZDR8072Q4XFM771QXD3KSR
title: Agent triage drafts reviewer and label proposals; the person accepts
type: task
status: done
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
updated_at: 2026-10-27T20:49:04Z
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

- [x] A triage session runs with the reviewer lockdown, and its proposals land as drafts only
- [x] A proposal naming a change or person outside the run is refused
- [x] No proposal reaches the forge without the person accepting it, and a test holds it
- [x] An accepted proposal is carried out by vuoro's code through forge.Administrator, with no agent session running
- [x] Accepting several proposals takes one confirmation, and a proposal whose change moved is marked stale, not applied

## Implementation plan

- drafts: a ProjectTriage role whose proposals carry change, reviewers, labels, unblock and reason. Validate them by role, allow one pending proposal per change, and add Store.AcceptProposal, which marks a proposal kept only once the apply succeeds.
- review: ProjectMode runs. One reviewer-class session runs in an empty directory with no checkout. CheckProposal binds each proposal to the run's changes, candidates and labels. A stamp is kept per change, and the binding is restored when a run reloads.
- forge: Change.Touched, read from GitLab's diffStats, and a TriageReader that reads CODEOWNERS and recent authors.
- app:
  - POST /api/projects/{key}/triage builds the facts: places, threads, stacks, paths, code owners, committers by directory, and each candidate's review load. It then starts the run.
  - GET returns the newest run.
  - accept re-reads each change and compares its stamp; a stale proposal waits. It computes additions only and checks the capability flags before calling Administer.
  - discard drops a proposal.
  - Project runs are left out of /api/runs, and the generic run routes refuse them except to read, close and talk to the session.
- web: a triage panel in the project view. Each proposal has a checkbox and editable chips, and one confirmation accepts the selected ones. Stale proposals offer "accept anyway".
- tests: drafts validation, the binding, the accept, stale and refusal paths in the app, the route guard, and the UI.

## Summary

Done. One ProjectTriage session runs per run. It is read-only, has no checkout and starts in an empty directory, and it writes only through the drafts MCP server. It reads facts that vuoro built: places, threads, stacks, diffStats paths, CODEOWNERS, recent committers by directory, and each candidate's review load.

- Each proposal is bound to the run's changes, candidates and labels. One that names anything else is refused, and the binding is restored when a run reloads.
- The person edits proposals, discards them, or accepts several after one confirmation. Accepting reads the change again and compares a stamp. A proposal whose change moved is reported stale and is applied only when the person accepts it again.
- An accepted proposal is applied by vuoro through forge.Administrator, with additions only. The author and existing values are skipped, and the capability flags are checked first.
- Project runs are left out of /api/runs. The generic run routes refuse them, except to read, close and talk to the session.

Tested in drafts, review, app (accept, stale, refusal, guard) and the UI. Checked live on drewshort/vuoro-sandbox: the agent proposed unblock lines only, because the only candidate was the author.

Also fixed a data race in the projects test fake, which `just race` had been failing on.
