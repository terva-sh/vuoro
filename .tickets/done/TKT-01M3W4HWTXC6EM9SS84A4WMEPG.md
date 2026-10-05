---
schema: 3
id: TKT-01M3W4HWTXC6EM9SS84A4WMEPG
title: Comment, reply, resolve, approve, and submit on GitLab
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - review
  - gitlab
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies:
  - TKT-01M3W4HWSGA04D4VQ8WHBEJC2D
  - TKT-01M3W4HWDY8XV92MXXQWA0AES7
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-01T16:25:25Z
updated_at: 2026-10-05T15:03:56Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude-code/gitlab-live-20261005
  name: ""
extensions: {}
---

## Description

The write half of the GitLab adapter, and the controls in the change view.
Inline and general comments go out as GitLab draft notes, and "submit" publishes
them together with `draft_notes/bulk_publish`, so that a review lands as one
event as reviewers expect. Replies, resolve and unresolve, approve, and revoke
approval.

Every write is the person's explicit action. A draft drafted by an agent is
visibly marked until the person edits or accepts it.

## Acceptance criteria

- [x] An inline comment, a reply, and a resolve each round-trip against a real GitLab change
- [x] Submit publishes every pending draft note as one batch
- [x] Approve and revoke work, and the inbox turn updates after each
- [x] No write is reachable without the loopback protection from the loopback-auth ticket

## Implementation plan

forge.Writer, with GitLab's implementation in forge/gitlab/write.go: draft notes (create, update, delete), bulk_publish, resolve, approve, and unapprove. app/write.go serves them under /api/changes/*, each answering with the change re-read, plus keep and drop for an agent's draft. Inline positions come from patch.Line, which names an unchanged line on both sides as GitLab requires.

## Notes

**agent:claude/vuoro-mvp-1** at 2026-10-02T20:04:06Z

Not yet run against a real GitLab change. It needs the owner's token, and a
merge request the owner can write to. The recorder test checks every request's
method, path, and position body. The end-to-end app test runs an agent,
then keeps, drops, and submits against a fake GitLab.

Criterion 3's inbox half waits for the inbox. Approve and revoke are wired,
but nothing derives the turn yet (TKT-01M3W4HWP).

Writes need a configured forge: an anonymous client refuses every write
before any request leaves.

**agent:claude/vuoro-mvp-1** at 2026-10-02T20:04:15Z

in-progress to blocked: The live round-trip needs the owner's GitLab token in the age store, the config file, and a merge request the owner can write to

**agent:claude-code/gitlab-live-20261005** at 2026-10-05T15:03:26Z

Live validation passed on 2026-10-05 against https://gitlab.com/drewshort/vuoro-sandbox/-/merge_requests/1, a private sandbox project the owner chose for this. The token came from the configured `token_command` (`ritual-secret get gitlab-pat`, `api` scope); its value was never read.

`forge/gitlab/write_live_test.go` adds `TestLiveWrite`, skipped unless `VUORO_LIVE_GITLAB_WRITE_MR` names a merge request. It resolves the token from vuoro's own config file, the way the app does. In one run of 36s:

- An inline draft on an added line (`notes.txt` new 5) and a general draft stayed invisible until Submit. One draft was edited, and another was created and deleted. Then `bulk_publish` published both as one batch, at the right position, and the deleted one was not published.
- A reply draft landed in the inline thread after Submit.
- Resolve and unresolve each round-tripped.
- With the owner added as a reviewer, the inbox's own derivation (fresh `Discover` roles plus `ReadFacts`, then `turn.Derive`) moved from `mine [R1 Your review is requested.]` to `theirs [waiting]` after approve, and back to `mine [R1]` after revoke.

gitlab.com accepted the author's self-approval on this free-tier project, although the project reports `merge_requests_author_approval: false`, and that setting cannot be changed on free tier. The earlier 401 on !144 was another project's rule.

The test drives `forge.Writer` and not the window's HTTP routes. Those routes keep their offline coverage in `app/write_test.go`. Rerun the test on the same MR at any time: each run uses a fresh nonce, and it refuses to start while drafts are pending.

## Summary

GitLab writes are validated live. The opt-in TestLiveWrite (forge/gitlab/write_live_test.go) round-tripped inline and general drafts, batch Submit, a reply, resolve and unresolve, and approve and revoke against the private sandbox drewshort/vuoro-sandbox!1. The inbox's discovery and turn derivation moved R1 to waiting and back after each verdict change. The window's routes over the writer keep their offline coverage. docs/roadmap.md is updated.
