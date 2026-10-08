---
schema: 3
id: TKT-01M4ES1EGCHKQXWYS9FY4QYQCC
title: Deliver an editor wave as suggestions on the pending comments
type: task
status: done
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
updated_at: 2026-10-08T23:07:12Z
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

- [x] The editor can be started for suggestions, the choice is remembered per change author, and the prompt carries the constraints
- [x] A wave's commits become suggestions at the reviewed head, riding on the finding's comment or in a new one, and overlaps and out-of-diff lines are misfits, which tests prove
- [x] A misfit is a plain diff within the cap, an uploaded patch on GitLab over it, and only counts beyond that, so no comment carries an unbounded diff
- [x] Delivery previews first, writes only pending comments on the person's confirmation, saves the wave's patches, and marks the findings suggested
- [x] The window offers both deliveries and shows a suggested finding

## Summary

An editor wave on somebody else's change can now go back as suggestions on the person's pending comments, instead of as a stacked change (D59).

- **Choosing.** "Fix these in a branch" offers "suggestions on your comments" or "a draft MR" where the forge applies suggestions and the person is not the author. The choice is remembered in prefs.json per host/author (`GET /api/runs/{id}/editor/delivery?author=`, which makes no forge call). A suggestion wave adds suggestionRules to the editor's prompt and stands on the reviewed head. The push bar puts the chosen delivery first and offers the other. A wave delivers once: `PushEditor` refuses a suggested wave (ErrDelivered). A suggestion wave cannot be pushed onto a stack an earlier wave holds.
- **Planning.** review/suggest.go reads each commit's `-U0` diff. It turns insertions and deletions into line replacements and joins changes within 3 lines. It maps lines back through the wave's earlier commits, and any overlap is a misfit. A joined edit that overlaps is retried piece by piece. A suggestion rides on the finding's own comment when the comment reaches it: on GitLab, within 3 lines, extending through head lines; on GitHub, exactly its line or span. Otherwise it becomes a new pending comment. The change's diff is consulted through `SuggestOptions.Shows`, built in app from forge/patch, since review may not import forge/.
- **Caps.** A suggestion is at most 100 lines either way. Misfits (new, deleted, renamed or binary files, untouched files, unshown lines, overlaps) go in the comment as a plain diff with reasons, up to 100 lines and 16 KiB. Past that, on GitLab, the commits' patch (under 1 MiB) is uploaded via the new `forge.Uploader` (GitLab `POST /projects/:id/uploads`) and linked. Otherwise only the counts appear.
- **Delivery.** `POST /editor/suggestions` previews every comment body and every new comment. `POST /editor/suggest` (confirm "suggest", plus the previewed sha) runs `DeliverSuggestions`. That rechecks the sha, asks the editor about unnamed findings as a push does, then writes per finding: new comments first, the upload, then UpdateDraft on its own comment, or CreateDraft if that comment is gone. It then calls MarkSuggested. A partial failure leaves the wave open, and a retry skips findings already sent. On finishing, it sets `EditorState.Suggested` and saves the wave's patches (D51). Nothing is pushed or submitted.
- **Window.** It has the choice at start, both buttons, SuggestDialog, "✎ suggested" on findings, and "Sent back as N suggestions" in FixLink with the saved patch.

Tested: review planner unit tests (edits, joins, mapping, comment text, cap), an end-to-end wave with a real editor repo (riding, misfits, overlap, GitHub span, partial delivery and retry, ErrDelivered, patches), the cap with and without uploads, GitLab Upload, app writeFix order, anchors and missing comment, shows(), route refusals, the AST only-caller test extended to DeliverSuggestions, writeFix and Upload, and vitest for the choice, button order, dialog, delivered state and FixLink. Visual check via a headless screenshot.

Not yet seen live: GitLab's rendering and applying of the editor's blocks, uploads on gitlab.com, and GitHub multi-line comments. The first live test is the terraform MR.
