---
schema: 3
id: TKT-01M47EG9E07Y4N1J34S5VDDC5N
title: Show only the write actions a forge supports, from its capabilities
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - forge
  - ui
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
updated_at: 2026-10-06T03:46:45Z
created_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
updated_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
extensions: {}
---

## Description

`forge.Capabilities` is `{Drafts, Resolve, Approve}`. Forgejo will support part of what GitLab does: the Forgejo posting spike (TKT-) decides which part. Possible gaps are replies, editing pending comments, resolving, withdrawing an approval, verdicts, and suggestions. The UI must show only what this forge, with these credentials, can do, and must stop special-casing a forge by name. `web/src/change.tsx` hardcodes "Forgejo posting is not available yet" today.

Extend the capability set to the granularity the spike shows is needed. Drive every write control from it, and make the read-only explanation come from capabilities and configuration rather than from `kind`. A hidden action is also refused server-side, with `forge.ErrUnsupported`.

## Acceptance criteria

- [x] Capabilities cover every write the spike found Forgejo cannot do
- [x] No web/src control or message branches on forge kind to decide what can be written
- [x] A write the forge cannot do is refused server-side with ErrUnsupported, with a test

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

From the spike, Forgejo's capabilities: drafts yes, but local only; replies yes, published separately at Submit; resolve no; approve yes; withdraw approval no; request changes yes; applicable suggestions no, so a suggestion posts as a plain fenced replacement; own-PR verdicts no. GitLab's withdraw-approval and resolve stay yes. The flags need to separate approve from withdraw, and to say whether a suggestion can be applied.

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

## Summary

Capabilities name reply, unapprove, request_changes and suggestions as well as drafts, resolve and approve. Every write control in the UI follows them, the server refuses unset ones with ErrUnsupported, and a suggestion posts as a plain replacement where the forge cannot apply one.
