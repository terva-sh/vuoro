---
schema: 3
id: TKT-01M46B3VTQXGGYRG88BX49PJXZ
title: Run a review from the command line on a range or a change
type: task
status: in-progress
status_reason: null
priority: high
due_on: null
labels:
  - review
  - agent
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies: []
blocks_on: none
references: []
claim:
  actor: agent:claude-code/cli-review-20261005
  branch: tickets/cli-review
  worktree: /Users/drewshort/Workspace/github.com/terva-sh/vuoro
  commit: 6890c242c0798a645b476f273a995e52db0d4eb6
  session: null
  claimed_at: 2026-10-05T15:32:30Z
  expires_at: null
archive: null
created_at: 2026-10-05T15:32:30Z
updated_at: 2026-10-05T15:34:02Z
created_by:
  id: agent:claude-code/cli-review-20261005
  name: ""
updated_by:
  id: agent:claude-code/cli-review-20261005
  name: ""
extensions: {}
---

## Description

The owner wants vuoro to review its own development from a terminal (D35). `vuoro review` runs a preset, or one profile, against a git range in a local checkout or a change URL. It uses the same runner, drivers, lockdown, synthesis and verification as the window, and prints the findings. docs/plan.md section 6.7 is the design.

This is the headless first slice of TKT-01M3YMSV6BABG7EW1NP8J6W5SR (Review a local branch or range with no forge). That ticket keeps the window side and the carry-over of findings into a merge request, at v1.0.

The command has no write path to a forge.

## Acceptance criteria

- [ ] vuoro review on a local range runs the chosen preset and prints its findings as Markdown
- [ ] vuoro review on a GitLab or Forgejo change URL reviews that change the same way
- [ ] -json prints one envelope with schemaVersion and kind, and a test holds its shape
- [ ] Exit status is 0 with no findings, 1 with findings, 2 for usage, and 3 for a failed run
- [ ] A test runs the command against a recording forge and proves it sends no write
- [ ] The drafts MCP server it serves goes through the loopback guard with per-session credentials
- [ ] vuoro reviews one of its own changes with the command, and the run is recorded

## Implementation plan

docs/plan.md section 6.7 is the design. The slices:

1. **review/local.go**: `LocalChange(ctx, git, dir, target)` resolves the range in the person's checkout. It runs git with hooks, fsmonitor, external diff and textconv disabled. It returns a forge-less `forge.Change` (Kind `local`, Ref `local/<name>-<hash>`, title from the newest commit, commit messages as the description, per-file patches from `--raw -z`/`--numstat -z` and a per-file `git diff`, and binary or oversized files Omitted) and a `review.Source` whose Remote is the checkout and whose refspec fetches the head SHA. A bare clone fetching a SHA from a local checkout is verified to work.
2. **review/findings.go**: `Findings(RunView)` renders every finding the person would work from, as Markdown. "Shown" becomes a pure function, shared with `Run.shown`, so the terminal and the window pick the same drafts.
3. **app**: export `Open`, `Source`, `ProbeTools` and a `Reviewers(preset, profile, model)` resolver, so the command and `startRun` share preset, synthesis and verifier defaults. There is no new route and no writer.
4. **main, `vuoro review`**: flags as in section 6.7. Its own loopback server serves only `/mcp/` behind `server.New`'s guard, with a fresh launch token. It starts the run, polls the view until every session has reported and none is working, prints progress to stderr, prints Markdown or the `review-result` envelope, closes the run (the worktree goes and the record stays), and exits 0/1/2/3, or 130 on interrupt.
5. **web**: the home run list shows a local run unlinked and marked local, since it has no forge page.
6. **Tests**: local ranges (`..`, `...`, a single rev, the default, renames, binary, dirty tree, a bad range); `Findings`; the command end to end with the fake driver, on a local range and on a change URL against a recording fake GitLab that refuses any write; the JSON shape; exit statuses.
7. **Dogfood**: run `vuoro review -preset quick` on this branch's own commits and record the result.
