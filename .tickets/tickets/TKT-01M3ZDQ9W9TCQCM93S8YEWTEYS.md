---
schema: 3
id: TKT-01M3ZDQ9W9TCQCM93S8YEWTEYS
title: Adopt terva-review and the two-forge process on Forgejo
type: chore
status: in-progress
status_reason: null
priority: high
due_on: null
labels:
  - ci
  - forgejo
  - github
  - review
assignees: []
milestone: null
parent: null
origin: null
dependencies: []
blocks_on: none
references: []
claim:
  actor: agent:claude-code/9c7c9cd7
  branch: terva-review-adoption
  worktree: /home/sothr/workspace/git.local.sothr.com/terva-sh/vuoro
  commit: 1db8913f0452847f5eb809ec2b2afd06ec6db02e
  session: null
  claimed_at: 2026-10-02T23:03:29Z
  expires_at: null
archive: null
created_at: 2026-10-02T23:03:23Z
updated_at: 2026-10-02T23:03:29Z
created_by:
  id: agent:claude-code/9c7c9cd7
  name: ""
updated_by:
  id: agent:claude-code/9c7c9cd7
  name: ""
extensions: {}
---

## Description

vuoro was developed elsewhere and pushed only to GitHub. On 2026-10-02 the owner had it stood up on the internal Forgejo as `terva-sh/vuoro`, which is private like the GitHub copy. Forgejo `main` was created equal to GitHub `main` at `1db8913`. The workspace side is ledger TKT-01M3ZDN0AAZZ5GQ54KKYBYN33K (Stand up vuoro on the internal forge from its GitHub copy).

vuoro already had `.forgejo/workflows/ci.yml`, a GitHub CI workflow, and a ticket store. It lacked the process the other two-forge terva-sh repositories carry. The owner wants automated terva-review code review to be standard on the internal forge.

### What this adds

- `.forgejo/workflows/terva-review.yml`, from terva-action-code-review's `examples/workflows/single-review.yml` at v0.5.1. That example pins the v0.5.0 image digest, which every terva-sh repository uses, and `warricksothr` is the maintainer. It reads the org `BOT_TOKEN` and `CPA_API_KEY` secrets and the `TERVA_REVIEW_*` org variables. `web/dist/**` and `package-lock.json` are excluded from review because both are generated, and `.tickets/**` is the carry path.
- `docs/pr-reviews.md`, describing both review processes and the `main` sync. It is adapted from tuohi.
- `just sync-github`, which fast-forwards whichever `main` is behind, never force-pushes, and stops on divergence. It carries `[positional-arguments]`, the fix from tuohi TKT-01M3J0VR65WX7HQAEDT5P6Q8P3, without which `--yes` was ignored.
- A pointer in AGENTS.md.

### Why not a push mirror

lampi TKT-01M3D7YXPKK7115YHXQYZS0AKR decided this for the org. A push mirror force-pushes, so a pull request merged on GitHub would be overwritten at the next sync.

## Acceptance criteria

- [ ] .forgejo/workflows/terva-review.yml is installed at the v0.5.0 image digest
- [ ] just sync-github fast-forwards the stale main and refuses when the two have diverged
- [ ] docs/pr-reviews.md describes both review processes and the main sync
- [ ] Forgejo CI is green on the adopting pull request
- [ ] terva-review publishes a review on the adopting pull request

## Implementation plan

Copy the v0.5.1 single-review example and set warricksothr as the maintainer. Exclude web/dist/** and package-lock.json, and carry .tickets/**. Adapt tuohi's docs/pr-reviews.md and its fixed sync-github recipe. Test the recipe against scratch bare remotes in both directions and diverged. Open the Forgejo PR, then dispatch terva-review on it from this branch and read the result.
