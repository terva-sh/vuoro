---
schema: 3
id: TKT-01M3W4HWXJYKCNXM4G7E86YX1M
title: "Agent: summarise a change, and what changed since my review"
type: task
status: in-progress
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
claim:
  actor: agent:codex/head-summary-20261003
  branch: tickets/change-summaries
  worktree: /home/sothr/workspace/git.local.sothr.com/terva-sh/vuoro
  commit: e641406982a82e6ca061df1c77379c278c5bce4a
  session: null
  claimed_at: 2026-10-03T07:00:00Z
  expires_at: null
archive: null
created_at: 2026-10-01T16:25:25Z
updated_at: 2026-10-03T07:00:01Z
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

- [ ] A summary streams into the change view for a real GitLab change
- [ ] The since-my-review summary covers only commits after my last review
- [ ] Cancelling stops the stream and the provider call

## Implementation plan

Add a separate summary run mode with one locked driver session, no drafts MCP capability and no forge writes. From a Summary panel choose the whole change or an explicit recorded completed Vuoro review head; fresh-read identity/head and verify the selected baseline server-side. GitLab's normalized published reviews lack reliable commit SHAs, so never infer a baseline from review timestamps: label the action since a Vuoro review, expose the exact range and explain when no verified completed baseline exists. Persist successful first-turn completion for new review sessions; legacy records lacking proof remain unqualified. Build a bounded complete git patch/commit list over immutable SHAs, verify ancestry, refuse missing/rewritten/shallow/oversized ranges without fallback or truncation. Stream through existing driver events/transcripts, persist range/source/partial output, keep summaries out of ordinary review/triage selectors, and offer history and explicit cancellation. Cancellation covers repository locking/checkout, late driver startup and provider generation; no process may launch/send after cancellation or outlive shutdown. Tests prove read-only capabilities, exact incremental ranges, persistence, cancellation and route protection/UI interactions. Update plan/roadmap and embedded dist. Validate a bounded read-only Haiku summary of a real public GitLab MR, full CGO_ENABLED=0 CI and native gates, then commit/push/PR/review/fix/merge/sync. Live GitLab posting remains deferred.
