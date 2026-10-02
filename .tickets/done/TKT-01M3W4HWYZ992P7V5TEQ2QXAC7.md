---
schema: 3
id: TKT-01M3W4HWYZ992P7V5TEQ2QXAC7
title: "Pending drafts: keep, edit, or drop what agents drafted"
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - agent
  - review
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies:
  - TKT-01M3W4HWTXC6EM9SS84A4WMEPG
  - TKT-01M3YK0CM0SWMYXMM7A7M7J2QX
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-01T16:25:25Z
updated_at: 2026-10-02T20:04:07Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-mvp-1
  name: ""
extensions: {}
---

## Description

The pending-draft surface in the change view. Drafts from reviewers and from the
synthesizer appear at their lines, or in the summary when they have no anchor.
Each is marked with the tool and model that drafted it, and the synthesizer's
drafts show their sources. The person keeps, edits, or drops each one, and
submits through the same control as their own comments. Nothing is posted by an
agent.

## Acceptance criteria

- [x] Agent findings appear as pending drafts anchored at file and line
- [x] A drafted comment is marked as drafted until the person edits or accepts it
- [x] No code path lets an agent run publish drafts, and a test proves it

## Implementation plan

The run's drafts show at their lines and in the run view, each marked with the tool and model that drafted it, and with Keep, Edit and keep, and Drop. Keeping one makes it the person's pending comment on the forge. The person's own pending comments show at their lines and in the overview, with edit and delete, and a submit bar sits in the change's header. Writing is offered only for a forge with a token in the config.

## Notes

**agent:claude/vuoro-mvp-1** at 2026-10-02T20:04:07Z

Checked in the browser read-only, with no forge configured: the run's drafts and the read-only notice render, and no write controls show. The write controls have not been clicked against a real forge, which waits on the owner's token. The rule that only the person posts is held by app TestAnAgentDraftsAndOnlyThePersonPosts and by the boundary test in imports_test.go.

## Summary

Agent drafts are kept, edited, or dropped by the person, and become their pending comments on GitLab only by that click. Tests prove that no agent path writes to the forge.
