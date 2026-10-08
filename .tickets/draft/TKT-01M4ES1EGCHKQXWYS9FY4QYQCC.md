---
schema: 3
id: TKT-01M4ES1EGCHKQXWYS9FY4QYQCC
title: Deliver an editor wave as suggestions on the pending comments
type: task
status: draft
status_reason: null
priority: high
due_on: null
labels:
  - review
  - agent
  - gitlab
  - ui
assignees: []
milestone: null
parent: TKT-01M4ES1E4BJAZ78GB96H20W1RD
origin: null
dependencies:
  - TKT-01M4ES1EATMXPJR56FDF54630C
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-08T22:09:46Z
updated_at: 2026-10-08T22:09:46Z
created_by:
  id: agent:claude/editor-suggestions-2026-10-08
  name: ""
updated_by:
  id: agent:claude/editor-suggestions-2026-10-08
  name: ""
extensions: {}
---

## Description

D59, and docs/plan.md section 6.3, "Fixes as suggestions".

- EditorState records how the wave goes back: a stacked change, or suggestions. "Fix these in a branch" offers both on somebody else's change, on a forge with `Suggestions`. The choice is remembered for each change author, by host and username, as the default next time. For suggestions, the editor's prompt adds the constraints: keep to existing lines of the files the change touches, near the finding, and add, delete, or rename no file.
- review/suggest.go turns a wave's commits into suggestions against the reviewed head. Each commit's no-context hunks are mapped back through the earlier commits, and overlaps are misfits. Nearby hunks join, and pure insertions take the line before. Each suggestion either rides on the finding's own pending comment or becomes a new pending comment, under the reach rules in the plan. A suggestion replaces at most 100 lines and writes at most 100.
- Misfits go into the finding's comment as a plain diff, with the reason, capped at 100 lines and 16 KiB per comment. Over the cap, on GitLab, the commit's saved patch is uploaded and linked. Over the patch cap, or on GitHub, the comment gives only the counts. GitLab gains an uploader.
- A preview route shows every comment's new body and every new comment, with the misfits. The confirm route, the person's action, updates and creates the pending comments and records the wave as suggested. It pushes nothing and submits nothing. It saves the wave's patches (D51) and marks each finding suggested, with its hunks and comment ids.
- A wave delivers once, one way. After a suggestion wave, the next wave starts from the reviewed head, given only findings not fixed or suggested.
- The window offers both deliveries in the push bar, with the chosen one first. It has the suggestions dialog, and shows "suggested" on a finding.

## Acceptance criteria

- [ ] The editor can be started for suggestions, the choice is remembered per change author, and the prompt carries the constraints
- [ ] A wave's commits become suggestions at the reviewed head, riding on the finding's comment or in a new one, and overlaps and out-of-diff lines are misfits, which tests prove
- [ ] A misfit is a plain diff within the cap, an uploaded patch on GitLab over it, and only counts beyond that, so no comment carries an unbounded diff
- [ ] Delivery previews first, writes only pending comments on the person's confirmation, saves the wave's patches, and marks the findings suggested
- [ ] The window offers both deliveries and shows a suggested finding
