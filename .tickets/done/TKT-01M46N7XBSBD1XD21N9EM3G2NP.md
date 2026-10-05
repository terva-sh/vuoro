---
schema: 3
id: TKT-01M46N7XBSBD1XD21N9EM3G2NP
title: GitLab's ghost user hands the person the turn
type: bug
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - gitlab
  - inbox
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-05T18:29:28Z
updated_at: 2026-10-05T18:33:59Z
created_by:
  id: agent:claude-code/inbox-tuning-20261005
  name: ""
updated_by:
  id: agent:claude-code/inbox-tuning-20261005
  name: ""
extensions: {}
---

## Description

On gitlab.com, notes by deleted users are reassigned to the ghost user, ghost1 ("Ghost User"). The inbox reads them as a person, so the owner saw "ghost1 is waiting for your answer in an unresolved thread" on years-old changes. GitLab's REST API reports bot as null for it, but GraphQL's UserCore.type is GHOST. Treat a user whose type is not HUMAN as automated, so it never hands over the turn. Fall back without the field on a GitLab that does not have it. Do not match by username: on gitlab.com, plain 'ghost' is a real person.

## Acceptance criteria

- [x] A note by GitLab's ghost user never makes it the person's turn
- [x] A GitLab without UserCore.type still reads changes

## Notes

**agent:claude-code/inbox-tuning-20261005** at 2026-10-05T18:33:58Z

query.graphql asks for UserCore.type on every user. forge/gitlab user() marks any type other than HUMAN as Bot, which covers GHOST and the bot types. On a GitLab that refuses the field, the client switches to a legacy query once and stays on it (noUserType). Tests: TestGitLabsGhostUserIsNotAPerson and TestAGitLabWithoutUserTypeStillReads. Live: ghost1 no longer appears in the owner's inbox, and their turn went from 21 to 12. flywheel-admin-bot reports type HUMAN, so only the config's bots list can mark it; with it listed, their turn is 10.

## Summary

GitLab users whose type is not HUMAN, including the ghost user, never hand over the turn, with a fallback for GitLabs without the field.
