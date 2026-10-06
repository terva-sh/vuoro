---
schema: 3
id: TKT-01M47EG9CSRDZ2HKANYN30XBAY
title: "Carry the person's verdict on submit: comment, approve, request changes"
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - forge
  - review
  - ui
  - gitlab
assignees: []
milestone: v1.0.0
parent: TKT-01M47EFQ400T20P79H9X152NAZ
origin: null
dependencies:
  - TKT-01M47EG9BHW0HBVFBTSBB1915X
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-06T01:50:57Z
updated_at: 2026-10-06T04:02:24Z
created_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
updated_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
extensions: {}
---

## Description

`forge.Writer` knows `Submit` and `Approve(bool)`, but has no "request changes". Forgejo submits a review with a verdict of `COMMENT`, `APPROVED` or `REQUEST_CHANGES`. The Forgejo posting ticket requires request changes, and GitLab has a requested-changes reviewer state as well.

Change the shared boundary so that a submit carries the person's verdict. Implement it on GitLab, and show it in the submit control. Recent GitLab has a GraphQL mutation for this; whether the configured instance supports it has not been checked, and a GitLab without it reports the capability false. The change moves the design: update docs/plan.md (the forge boundary) in the same pull request, and record the choice as the next decision in docs/decisions.md.

Keep the existing guarantee: the verdict is the person's explicit choice in the UI, and agent output never selects or sends it.

## Acceptance criteria

- [x] Submit takes a verdict through the shared forge boundary, and GitLab implements request changes or reports it unsupported
- [x] The submit control offers only the verdicts the forge supports
- [x] docs/plan.md and a new docs/decisions.md entry record the boundary change

## Implementation plan

One PR for this ticket and TKT-01M47EG9E07Y4N1J34S5VDDC5N (Show only the write actions a forge supports, from its capabilities), because both change the shared boundary and the same submit controls.

### The boundary

- **Submit.** `forge.Writer.Submit(ctx, ref, verdict forge.Verdict)` publishes the person's pending comments with a verdict. It reuses the existing `forge.Verdict` values: commented, approved, changes_requested. `Approve(ctx, ref, bool)` stays for a standalone approve or a withdrawal.
- **GitLab.**
  - Commented is `POST draft_notes/bulk_publish`, as today.
  - Changes requested is the same call with `reviewer_state=requested_changes`. This is documented on the draft notes API, and the GraphQL schema also has `mergeRequestRequestChanges`.
  - Approved is bulk_publish, then `/approve`. This moves "Submit and approve" out of app/write.go and into the adapter, where Forgejo's one-call review also belongs.

### Capabilities

`forge.Capabilities` gains four flags:

- `Reply`: a pending comment can answer a thread.
- `Unapprove`: withdrawing one's own approval.
- `RequestChanges`.
- `Suggestions`: the forge applies a suggestion block.

GitLab with a token sets every flag. Forgejo stays all false until its writer lands.

### The server refuses what the forge cannot do

app/write.go refuses with `forge.ErrUnsupported` any write the change's capabilities do not allow: a reply, resolving, approving, withdrawing an approval, or a verdict. `keep` formats a suggestion as a plain fenced replacement when `Suggestions` is false (D43).

### The UI

The web `Writer` carries the capabilities. `SubmitBar` offers Submit, "Submit and approve" and "Request changes" from the flags, and shows "Revoke approval" only with `unapprove`. Reply and Resolve buttons follow `reply` and `resolve`. The read-only line comes from capabilities and configuration rather than from `kind`, and the wording uses `changeNumber` rather than GitLab's "merge request" and `!`. "request_changes" becomes a confirmable action.

### Docs

docs/plan.md section 5 records the boundary, and D44 records the decision.

### Tests

- GitLab: write_test checks the request for each verdict.
- app: refusal for each capability, and the submit verdict.
- Web: tests cover the gating.
- The GitLab live test gains a request-changes step, if a token is available.

## Notes

**agent:claude/forgejo-parity-20261006** at 2026-10-06T02:00:31Z

From the spike: Forgejo submit takes COMMENT, APPROVED or REQUEST_CHANGES. REQUEST_CHANGES requires a body, and COMMENT requires a body or an inline comment. The author cannot approve or request changes on their own PR (422), so the control should offer only Comment there. A later verdict marks the same reviewer's earlier non-comment verdict dismissed.

**agent:claude/forgejo-parity-20261006** at 2026-10-06T03:46:45Z

Landed together with TKT-01M47EG9E07Y4N1J34S5VDDC5N (Show only the write actions a forge supports, from its capabilities). The plan section on each ticket is the design. D44 records the decision.

### What changed

- **The boundary:**
  - `forge.Writer.Submit(ctx, ref, forge.Verdict)` publishes with the verdict.
  - GitLab: commented is bulk_publish, changes requested is bulk_publish with `reviewer_state=requested_changes`, and approved is bulk_publish then `/approve`.
  - `forge.Capabilities` gains `Reply`, `Unapprove`, `RequestChanges` and `Suggestions`. GitLab with a token sets all of them, and Forgejo stays all false until its writer lands.
- **The server** (`allowed` in app/write.go) refuses each write whose flag is unset with `forge.ErrUnsupported` (422) before the adapter runs: drafts, replies including triage replies, resolve, approve, withdraw, and each verdict. An unknown verdict is a 400. `keepBody` posts a plain "Suggested replacement for lines N–M" block when `Suggestions` is false (D43). "request_changes" is a confirmable action.
- **The UI:**
  - The web `Writer` carries the capabilities. `SubmitBar` offers Submit, "Submit and approve" with approve, and "Request changes" with request_changes. It shows Revoke only with unapprove and Approve only with approve.
  - Reply and Resolve follow their flags, triage's "Add to pending replies" follows reply, and `keepText` mirrors the server's suggestion choice.
  - The read-only line comes from configuration and capabilities, not kind.
  - The bar's wording uses `changeNumber` and no longer says "merge request" or names GitLab.
  - The `kind` branches left in web/src are display only: the number prefix, link paths, forge names, and local runs.

### Tests

- forge/gitlab/write_test checks each verdict's request.
- app/caps_test covers every refusal, and that a read-only forge takes no write.
- app/keep_test checks both suggestion forms.
- Web tests cover the gating, the verdict sent, and the replacement text.
- `just ci` passes on Linux.

### Not verified live

GitLab's `reviewer_state=requested_changes` has offline coverage only. `TestLiveWrite` gained a request-changes step, but this machine has no vuoro config, so it was not run here. Run `VUORO_LIVE_GITLAB_WRITE_MR=<sandbox MR> go test ./forge/gitlab -run TestLiveWrite` where the GitLab token is configured. An instance too old for `reviewer_state` would ignore it and leave the reviewer state unchanged, and that step would catch it.

### Seen in passing

`web/node_modules/.vite/vitest/.../results.json` is tracked, added in 86395f0, and every test run rewrites it. It was restored, not committed. It wants a .gitignore entry and removal from the index, in its own change.

**agent:claude/forgejo-parity-20261006** at 2026-10-06T03:53:28Z

This supersedes the 'Not verified live' paragraph of the previous note. terva-review on PR #16 (run 4dd6edbf, high) found that GitLab answers bulk_publish with success whether or not it recorded reviewer_state, so an older GitLab would drop the person's request for changes without anyone knowing. Fix: after a changes-requested publish, Submit reads /user and the merge request back, and confirms the person's reviewer state is requested_changes. If it is not, Submit returns errChangesNotRecorded, which says the comments were published but the request for changes was not recorded. GitLab records the state only for the change's reviewers, and an older GitLab ignores it. TestRequestingChangesThatGitLabDidNotRecordIsAnError holds this. Alternatives rejected: probing the version in Capabilities, which has no context or network, and the version the feature arrived in is not documented; and the GraphQL mergeRequestRequestChanges mutation after publishing, which fails loudly where it is missing but lands the verdict as a second event. TestLiveWrite's request-changes step still needs a machine with the token.

**agent:claude/forgejo-parity-20261006** at 2026-10-06T04:02:24Z

Follow-up from terva-review run f172ee88 (medium) on PR #16. If reading /user or the merge request failed after the publish succeeded, requestedChanges returned the bare read error, which reads as a failed submit even though the comments were public. Every failure after the publish now says the comments were published and the request for changes could not be confirmed, and it wraps the read error. That error is distinct from errChangesNotRecorded, which means GitLab was read and the state is missing. TestAnUnconfirmedRequestForChangesSaysTheCommentsWerePublished holds this. Left as it is: the window shows the error without refreshing the change, as for every failed write. The message tells the person to check the merge request.

## Summary

Submit takes the person's verdict through forge.Writer. GitLab maps changes requested to bulk_publish's reviewer_state and approved to publish-then-approve, and the submit control offers only supported verdicts. D44 records the decision. GitLab's request-changes step in TestLiveWrite is still to run live.
