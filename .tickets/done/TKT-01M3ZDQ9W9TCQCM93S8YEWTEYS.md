---
schema: 3
id: TKT-01M3ZDQ9W9TCQCM93S8YEWTEYS
title: Adopt terva-review and the two-forge process on Forgejo
type: chore
status: done
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
claim: null
archive: null
created_at: 2026-10-02T23:03:23Z
updated_at: 2026-10-08T14:10:31Z
created_by:
  id: agent:claude-code/9c7c9cd7
  name: ""
updated_by:
  id: agent:claude/forgejo-parity-20261006
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

- [x] .forgejo/workflows/terva-review.yml is installed at the v0.5.0 image digest
- [x] just sync-github fast-forwards the stale main and refuses when the two have diverged
- [x] docs/pr-reviews.md describes both review processes and the main sync
- [x] Forgejo CI is green on the adopting pull request
- [x] terva-review publishes a review on the adopting pull request

## Implementation plan

Copy the v0.5.1 single-review example and set warricksothr as the maintainer. Exclude web/dist/** and package-lock.json, and carry .tickets/**. Adapt tuohi's docs/pr-reviews.md and its fixed sync-github recipe. Test the recipe against scratch bare remotes in both directions and diverged. Open the Forgejo PR, then dispatch terva-review on it from this branch and read the result.

## Notes

**agent:claude-code/9c7c9cd7** at 2026-10-02T23:17:19Z

The first terva-review (run 7553f8e1, Actions run 3, gpt-6.1-sol, head `d635e6e`) reviewed the adopting PR #1 and failed it with two findings. Both are accepted and fixed in the next commit.

- **medium, accepted:** `REVIEW_CARRY_PATHS` was left commented out. The v0.5.1 example ships it commented, while lampi's copy enables it, and the adaptation missed that. Carry would have refused although this ticket and docs/pr-reviews.md promise it. Fixed by uncommenting it.
- **low, accepted:** `sync-github` discarded the GitHub fetch's stderr and reported every failure as "github/main does not exist yet". It now probes with `git ls-remote --exit-code --heads github main`. Exit 2 means the remote answered without `main`, and any other failure keeps git's message and says the remote could not be read. Re-tested against scratch bare remotes: no main, unreachable remote, in sync, dry run, both fast-forward directions, divergence, and an unknown flag. tuohi's and lampi's copies of the recipe have the same defect. That is not fixed here.

The review showed the workflow end to end on a private repository: the bot published a review and the `terva-review/code` status. PR CI (Actions run 19656) passed.

**agent:claude-code/9c7c9cd7** at 2026-10-02T23:33:40Z

The second terva-review (run d91ab8c6, request after-fixes-1, Actions run 19686) passed head d2f8b8a with no findings at the failure threshold. CI run 19687 is green on the same head. The commit that records this touches only .tickets/, so it is carried with --input carry=true rather than re-reviewed. Merging PR #1 and the first just sync-github --yes are left to the owner.

## Summary

terva-review runs on every Forgejo pull request from .forgejo/workflows/terva-review.yml, and just sync-github keeps GitHub's main equal to Forgejo's without force-pushing. The adopting PR #1 merged after a clean second review (run d91ab8c6). Since then the process has reviewed every PR, most recently #22 to #26 on 2026-10-08, and docs/pr-reviews.md describes it. Closed in the 2026-10-08 ticket audit; the old claim was released with it.
