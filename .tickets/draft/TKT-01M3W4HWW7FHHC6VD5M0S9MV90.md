---
schema: 3
id: TKT-01M3W4HWW7FHHC6VD5M0S9MV90
title: Decide how vuoro embeds terva for agent actions
type: spike
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - agent
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies: []
blocks_on: none
references:
  - ref: repo:terva-sh/terva-action-code-review
    path: null
claim: null
archive: null
created_at: 2026-10-01T16:25:25Z
updated_at: 2026-10-02T14:39:44Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
extensions: {}
---

## Description

docs/plan.md section 6 sets the starting position. Reviews come from
terva-sh/terva-action-code-review's local mode, and the other agent actions
call terva directly. This spike confirms that or replaces it.

### Review drafting through the action

Run the action's local mode against a real GitLab change, fetched into a local
checkout, and decide:

- **How to run it.** The container image
  (`container.local.sothr.com/terva-sh/terva-action-code-review`, pinned by
  digest, through podman or docker), or `node src/local-main.mjs` with terva on
  `PATH`. A container keeps the reviewer reproducible. Node avoids a container
  runtime. The image registry is internal, which matters if vuoro is
  published.
- **How to map findings.** Turn the `local-1.x` result's anchored findings
  into pending drafts. Say what happens to an unanchored finding, a `base`-side
  anchor, and a run whose completion is not `completed`.
- **How to validate.** The JSON schema alone is not enough, because
  `src/review-result.mjs` adds cross-field rules. Port them to Go, or take only
  what vuoro reads.

### The other agent actions

Summary and reply drafts: `terva rpc` as a subprocess (the action's choice,
which keeps vuoro off Go 1.27), or the in-process SDK or engine. Decide where
the provider key comes from, how streamed output reaches the window, and how
cancel reaches the provider call.

Read: the action's `docs/local-reviews.md`, `docs/review-result-contract.md`,
`docs/architecture.md`, and `src/terva.mjs`, plus terva's `docs/rpc.md` and
`docs/embedding.md`.

## Acceptance criteria

- [ ] docs/plan.md section 6 records the chosen entry point and why, with the open-question marker removed
- [ ] A throwaway program streams one terva completion over a recorded diff on this Mac
- [ ] The tool list for agents is written down, and it contains no forge write
