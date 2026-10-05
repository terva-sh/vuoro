---
schema: 3
id: TKT-01M46RXTQ98KJNR86RAZ7KB59D
title: "Redesign the inbox: summary, lanes, detail pane, dormant, snooze"
type: task
status: done
status_reason: null
priority: high
due_on: null
labels:
  - inbox
  - ui
  - desktop
assignees: []
milestone: null
parent: null
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-05T19:33:52Z
updated_at: 2026-10-05T19:41:31Z
created_by:
  id: agent:claude-code/inbox-20261005
  name: ""
updated_by:
  id: agent:claude-code/inbox-20261005
  name: ""
extensions: {}
---

## Description

The owner found the inbox overwhelming: one long column of tall cards, with a reason line for every thread and abandoned changes at the top. D38 records the chosen design, which builds on the proposals in docs/inbox-design.md.

## Acceptance criteria

- [x] A summary strip counts changes requested, ready to merge, replies owed, and reviews requested, and each count filters the list
- [x] Your turn splits into lanes (your changes, replies owed, reviews), oldest first within each
- [x] Rows are two lines with an age badge, and repeated reasons collapse into one line with counts
- [x] Selecting a row shows a detail pane with the waiting threads and the change's actions
- [x] Changes quiet for inbox.dormant_days (default 90) of human activity fold into Dormant
- [x] Snooze offers until tomorrow, a week, and until it changes
- [x] Their turn and Dormant fold by default, and j/k, Enter, o, s, r and / work from the keyboard
- [x] D38 and docs/plan.md describe the inbox as built

## Implementation plan

Backend:
- turn: Reason gains Who (the person the reason is about: the thread's last author for A1, the author for R3, the reviewer for A2 and waiting). Add LastHumanActivity(ch), which ignores bots, system notes, and ch.Updated.
- inbox Row gains Lane (yours | replies | reviews, empty for theirs), Summary (one line per reason kind, with counts), Waiting (the threads behind the reasons: author, path:line, first line of the last real note, since), Quiet (the last human activity), and the project's short name.
- Display gains Dormant. A row quiet past the threshold goes there instead of Mine or Theirs, and snoozed rows stay snoozed. The threshold comes from config `inbox.dormant_days` (default 90), which the Service takes as an option.
- Snooze: a sentinel `until` (9999-12-31) means "until it changes". The event key already ends it.

Frontend (web/src/inbox.tsx, rewritten):
- A header line with each account's refresh state and the Refresh button. A summary strip of four counts that filter, with numbers on each.
- The left list: Your turn in three lanes, then Their turn, Dormant, Snoozed and Could not read, each folded. Rows are two lines with a colourblind-safe age badge (blue < 2d, amber < 14d, magenta older, number always shown).
- The right pane: title, project, roles, age, actions (Open in vuoro, Open on forge, Snooze ▾ with three choices), the summary lines, and the waiting threads.
- Keys: j/k move, Enter opens in vuoro, o opens on the forge, s snoozes until tomorrow, r refreshes, / focuses the URL field.
Tests in turn, inbox, and web. Update docs/plan.md and mark docs/inbox-design.md as decided.

## Notes

**agent:claude-code/inbox-20261005** at 2026-10-05T19:41:30Z

Checked headless against the owner's real gitlab.com inbox (scratch home, read-only). 51 changes: 8 on your turn in two lanes (2 replies owed, 6 reviews), 24 on their turn, and 19 dormant, including the 1,974-day and 1,473-day review requests that used to top the list. The !144 row reads '8 threads await your answer: ehlertjd (5), naterichman (3)', and its pane lists all eight threads with place and first line. The triage action and its t key are left out of the pane; docs/inbox-design.md says why.

## Summary

The inbox now has a summary strip with four filtering counts, lanes on Your turn, two-line rows with colourblind-safe age badges, and a detail pane with the waiting threads. Their turn, Dormant (inbox.dormant_days, default 90, counting human activity only) and Snoozed fold away. Snooze offers tomorrow, a week, or until it changes, and there are keyboard keys. D38 records it; the plan and inbox-design.md are updated.
