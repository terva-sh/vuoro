---
schema: 3
id: TKT-01M3W4HWXJYKCNXM4G7E86YX1M
title: "Agent: summarise a change, and what changed since my review"
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - agent
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies:
  - TKT-01M3W4HWSGA04D4VQ8WHBEJC2D
  - TKT-01M3YK0CH5RHPJJS7E0DX9SH8X
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-01T16:25:25Z
updated_at: 2026-10-03T07:21:09Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:codex/head-summary-20261003
  name: ""
extensions: {}
---

## Description

The first agent action, chosen because it is read-only and useful on every
change. From the change view, ask for a summary of the change, or of the
commits since my last review, through the driver the person chose. Output
streams into a panel and is never posted anywhere.

## Acceptance criteria

- [x] A summary streams into the change view for a real GitLab change
- [x] The since-my-review summary covers only commits after my last review
- [x] Cancelling stops the stream and the provider call

## Implementation plan

Add a separate summary run mode with one locked driver session, no drafts MCP capability and no forge writes. From a Summary panel choose the whole change or an explicit recorded completed Vuoro review head; fresh-read identity/head and verify the selected baseline server-side. GitLab's normalized published reviews lack reliable commit SHAs, so never infer a baseline from review timestamps: label the action since a Vuoro review, expose the exact range and explain when no verified completed baseline exists. Persist successful first-turn completion for new review sessions; legacy records lacking proof remain unqualified. Build a bounded complete git patch/commit list over immutable SHAs, verify ancestry, refuse missing/rewritten/shallow/oversized ranges without fallback or truncation. Stream through existing driver events/transcripts, persist range/source/partial output, keep summaries out of ordinary review/triage selectors, and offer history and explicit cancellation. Cancellation covers repository locking/checkout, late driver startup and provider generation; no process may launch/send after cancellation or outlive shutdown. Tests prove read-only capabilities, exact incremental ranges, persistence, cancellation and route protection/UI interactions. Update plan/roadmap and embedded dist. Validate a bounded read-only Haiku summary of a real public GitLab MR, full CGO_ENABLED=0 CI and native gates, then commit/push/PR/review/fix/merge/sync. Live GitLab posting remains deferred.

## Notes

**agent:codex/head-summary-20261003** at 2026-10-03T07:11:08Z

Implemented separate one-turn summary runs with no MCP/draft access, bounded full exact git ranges, explicit completed-local-review baselines, protected fresh-read routes, streaming/history UI and cancellation across repository waiting/startup/generation/shutdown. Successful initial review completion is persisted; legacy/cancelled/failed records cannot be inferred as completed. Rationale: normalized GitLab published review facts lack commit SHAs, so timestamps cannot safely define the since-review range; labels identify Vuoro review runs instead. Rewrites/missing/shallow/oversized ranges refuse before provider work, and no partial patch is sent. A regression exposed bytes.Buffer's promoted ReadFrom bypassing a bounded Write; replaced it with a named buffer and confirmed oversized input refusal. Offline tests prove incremental input excludes earlier changes, partial-stream cancellation, late-start suppression, reload state, shutdown/close, no draft access, route protection and frontend response races. Full CI passed before the final small history-label/UI copy adjustment; latest gate is running.

Live read-only browser validation opened public gitlab-org/cli!3991 anonymously and prepared exact range 59ae78ca..8526e3d7. Claude's session offered only Glob/Grep/Read but returned a login failure before text. Normal auth status reports loggedIn=true (oauth_token); the actual locked invocation, --restricted --setting-sources '', reports loggedIn=false. No auth files or tokens were read, and no GitLab writes were attempted. Asked the owner to expose the tool login independently of skipped settings while PR work continues. Added a typed login failure with a controlled actionable summary error. The real-change streaming criterion remains unchecked pending successful live text.

**agent:codex/head-summary-20261003** at 2026-10-03T07:16:33Z

PR #14 review findings addressed: cancellation and timeout now Cancel and Close the one-turn summary session; fixture cancellation leaves Events open until Close to prove lifecycle completion. Initial turn reported state is persisted separately from success, with reload plus successful-follow-up regressions for failed/cancelled first turns and conservative legacy handling. Additional blocked-pipe regression proves Claude Send does not hold the cancellation mutex, cancellation releases the write and the process is reaped. stopSummary cancels context before acquiring the send-held run mutex and records stop intent atomically so preparation failure races remain Cancelled. Full local just ci passed on these fixes: cgo-free Go tests/vet/six cross-builds, 51 frontend tests, embedded dist/design and strict ticket checks. Dispositions posted while PR open at https://git.local.sothr.com/terva-sh/vuoro/pulls/14#issuecomment-25291. A fresh full model review is required for the fix commit.

**agent:codex/head-summary-20261003** at 2026-10-03T07:16:54Z

Live real-change streaming criterion passed on 2026-10-03 in Vuoro's protected Summary UI: anonymously opened https://gitlab.com/gitlab-org/cli/-/merge_requests/3991, prepared exact range 59ae78ca..8526e3d7, and observed working text followed by a complete ended summary describing the label-count fix, changed files, tests and risks. Locked Claude session offered only Glob/Grep/Read, with no MCP server or forge credential. Authentication was supplied privately to the validation launch environment; values were not printed or saved. This supersedes the earlier live-login blocker. No GitLab posting was attempted. Since-review isolation remains proven by exact real local git histories rather than inferred published-review timestamps. README documents authentication through the launch environment without weakening the reviewer settings lockdown.

**agent:codex/head-summary-20261003** at 2026-10-03T07:21:08Z

Fresh full model review on ecc2258c57a8eabcec63a41ea61d220da41e0d24 is clean: Actions run #133 explicitly resolves both prior findings. Forgejo CI passes; GitHub run 37105855914 passes Linux, macOS (including the native window check) and Windows. Two bounded read-only public GitLab summaries completed in the UI. The optional Stop click occurred after the second summary had already ended; it is not claimed as live cancellation evidence. Cancellation/provider-stop evidence remains the targeted lifecycle and blocked-subprocess regression tests. All acceptance criteria are earned. Final ticket-only closure will carry the clean review and receive fresh exact-head CI before merge.

## Summary

Implemented in Forgejo PR #14: protected whole-change and explicit completed-Vuoro-review summaries with immutable exact ranges, read-only one-turn driver sessions, no drafts MCP or forge writes, streaming and saved history, partial text and cancellation through startup/send/close/shutdown. Failed/cancelled/legacy-unverified initial reviews stay ineligible after reload and successful follow-ups. Missing/rewritten/shallow/oversized ranges refuse before provider work. A public GitLab MR streamed to completion using locked Claude; exact incremental histories and cancellation are regression-tested. Both model findings fixed and dispositioned; fresh full review clean. Full cgo-free local CI and Linux/macOS/Windows gates pass. Live GitLab posting remains deferred, and published review timestamps are not used as commit baselines.
