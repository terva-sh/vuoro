---
schema: 3
id: TKT-01M46B3VTQXGGYRG88BX49PJXZ
title: Run a review from the command line on a range or a change
type: task
status: done
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
claim: null
archive: null
created_at: 2026-10-05T15:32:30Z
updated_at: 2026-10-05T15:43:33Z
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

- [x] vuoro review on a local range runs the chosen preset and prints its findings as Markdown
- [x] vuoro review on a GitLab or Forgejo change URL reviews that change the same way
- [x] -json prints one envelope with schemaVersion and kind, and a test holds its shape
- [x] Exit status is 0 with no findings, 1 with findings, 2 for usage, and 3 for a failed run
- [x] A test runs the command against a recording forge and proves it sends no write
- [x] The drafts MCP server it serves goes through the loopback guard with per-session credentials
- [x] vuoro reviews one of its own changes with the command, and the run is recorded

## Implementation plan

docs/plan.md section 6.7 is the design. The slices:

1. **review/local.go**: `LocalChange(ctx, git, dir, target)` resolves the range in the person's checkout. It runs git with hooks, fsmonitor, external diff and textconv disabled. It returns a forge-less `forge.Change` (Kind `local`, Ref `local/<name>-<hash>`, title from the newest commit, commit messages as the description, per-file patches from `--raw -z`/`--numstat -z` and a per-file `git diff`, and binary or oversized files Omitted) and a `review.Source` whose Remote is the checkout and whose refspec fetches the head SHA. A bare clone fetching a SHA from a local checkout is verified to work.
2. **review/findings.go**: `Findings(RunView)` renders every finding the person would work from, as Markdown. "Shown" becomes a pure function, shared with `Run.shown`, so the terminal and the window pick the same drafts.
3. **app**: export `Open`, `Source`, `ProbeTools` and a `Reviewers(preset, profile, model)` resolver, so the command and `startRun` share preset, synthesis and verifier defaults. There is no new route and no writer.
4. **main, `vuoro review`**: flags as in section 6.7. Its own loopback server serves only `/mcp/` behind `server.New`'s guard, with a fresh launch token. It starts the run, polls the view until every session has reported and none is working, prints progress to stderr, prints Markdown or the `review-result` envelope, closes the run (the worktree goes and the record stays), and exits 0/1/2/3, or 130 on interrupt.
5. **web**: the home run list shows a local run unlinked and marked local, since it has no forge page.
6. **Tests**: local ranges (`..`, `...`, a single rev, the default, renames, binary, dirty tree, a bad range); `Findings`; the command end to end with the fake driver, on a local range and on a change URL against a recording fake GitLab that refuses any write; the JSON shape; exit statuses.
7. **Dogfood**: run `vuoro review -preset quick` on this branch's own commits and record the result.

## Notes

**agent:claude-code/cli-review-20261005** at 2026-10-05T15:43:33Z

Built and dogfooded on 2026-10-05.

- **Local range:** `vuoro review -preset quick main..HEAD` on this branch reviewed its own two commits with one Sonnet reviewer plus a verifier, for $1.26 in about four minutes, and exited 1. It found one real problem, confirmed by the verifier: `Settled`'s doc comment had been inserted inside `settle`'s. That is fixed in a6f4c95.
- **Change URL:** `vuoro review -preset quick -json https://gitlab.com/drewshort/vuoro-sandbox/-/merge_requests/1` returned a `review-result` envelope, outcome clean, exit 0, $0.02. Afterwards the MR's newest note was still the 15:02 write-test note, and it had no pending drafts.
- **Offline tests:** review/local_test.go (range forms, renames, binary, deletes, dirty tree, refusals, checkout through Repos); review/findings_test.go; review_cmd_test.go (local range, JSON shape, exit statuses, interrupt, the quick default, a recording GitLab that refuses every write, and MCP guard refusals for a foreign Host, a foreign Origin, no credential, and a made-up credential); web/src/runlist.test.tsx.

One change outside the command: `Run.Settled` needed a `nudging` flag. A verifier sent back for unmarked findings is otherwise idle for a moment before the nudge lands, which would read as settled.

Not done here, and kept in TKT-01M3YMSV6BABG7EW1NP8J6W5SR: carrying a local review's findings into the merge request opened later, and reviewing a local range from the window.

## Summary

vuoro review runs a preset or one profile on a local range or a change URL, through the window's own runner, drivers, lockdown, synthesis and verification. It prints Markdown findings or a review-result JSON envelope, exits 0/1/2/3/130, and has no forge write path, which a recording-forge test proves. Its drafts MCP server sits behind the loopback guard. Dogfooded on its own branch (one real finding, fixed) and on the sandbox MR. docs/plan.md section 6.7 and D35 record the design.
