---
schema: 3
id: TKT-01M3YK0CNGAX0M1Y755995W7QN
title: Talk to the reviewing agent from the change view
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - agent
  - ui
  - review
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies:
  - TKT-01M3YK0CM0SWMYXMM7A7M7J2QX
  - TKT-01M3W4HWSGA04D4VQ8WHBEJC2D
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-02T15:16:29Z
updated_at: 2026-10-02T19:47:38Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-mvp-1
  name: ""
extensions: {}
---

## Description

A review run is a conversation, not a report. The change view keeps the
session open in a panel. The person can ask why, ask the agent to look at
something else, or ask it to drop, merge, or reword a finding. The agent's
draft edits arrive through the channel the spike chose, and they show as
edits to the pending drafts. Tool calls are visible as they happen.

Closing the change view keeps the session, and reopening resumes it.

## Acceptance criteria

- [x] A follow-up question about a finding streams an answer in the panel
- [x] Asking the agent to reword or withdraw a finding changes the pending draft, marked as drafted
- [x] Reopening a change resumes its review conversation

## Implementation plan

Each session pane gets a talk box (Enter sends, Stop while it works). The person's message goes to the same session, and one that ended resumes from its handle. Run records now carry what a reload needs (resume handles, repository key, anchor index), and vuoro loads them at start, so reopening a change after a restart finds its runs and resumes their conversations. Draft cards show revisions.

## Notes

**agent:claude/vuoro-mvp-1** at 2026-10-02T19:47:38Z

Live, 2026-10-02, Haiku on gitlab-org/cli!3991. A message typed in the
talk box asked for a finding, and the reviewer drafted d1. Asked to reword it,
it revised d1 to revision 2, "listTitle needs a doc comment". vuoro was then
killed with SIGTERM, which left no claude process behind, and restarted. The
run came back with its 30 transcript entries and d1. The next message
resumed the same Claude Code session, which knew d1, and withdrew it.

The answer streams through polling: a working run is read once a second, and
text deltas are appended to the open assistant entry.

## Summary

The person talks to each reviewer from its pane, stops it, and resumes it, including after vuoro restarts. Drafts change in place and show their revisions.
