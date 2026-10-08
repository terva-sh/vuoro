---
schema: 3
id: TKT-01M4CPAZX0WNTCCV7R3HJ5MAJP
title: Wait for a review's goroutines in Shutdown; a local-review test flaked
type: bug
status: done
status_reason: null
priority: low
due_on: null
labels:
  - review
  - ci
assignees: []
milestone: null
parent: null
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-08T02:44:04Z
updated_at: 2026-10-08T03:20:53Z
created_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
updated_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
extensions: {}
---

## Description

`TestABranchIsReviewedFromTheWindowAndItsCheckoutIsLeftAlone` (app/local_test.go)
failed once on Forgejo CI, run #211 on 7c3bc7a. A re-run of the same commit
passed, so it is pinned as intermittent and most likely caused by load on the CI
workers. Pick it up if it happens again.

### The failure

```
--- FAIL: TestABranchIsReviewedFromTheWindowAndItsCheckoutIsLeftAlone (0.28s)
    testing.go:1617: TempDir RemoveAll cleanup: unlinkat /tmp/TestABranchIsReviewedFromTheWindowAndItsCheckoutIsLeftAlone.../002: directory not empty
```

`002` is the test's second `t.TempDir()`, the review state dir. Something wrote
into it while the TempDir cleanup was deleting it.

### What did not reproduce it

- 300 runs of the test alone, locally.
- 200 runs on GOMAXPROCS=1 with six busy loops competing for the CPU.
- `go test -count=4 ./...` in CI's image (golang:1.27-alpine) limited to one CPU.
- Delaying pump's final save by 200ms. This proves little: a write that lands
  after RemoveAll finishes recreates the directory and nobody checks again.
  Only a write that lands during RemoveAll fails the test.

### What was ruled out

The orphaned child of a cancelled `git worktree add`. That command does run a
child `git reset --hard` that writes the checkout. But `Repos.git` captures the
output in buffers, so the child inherits the pipe, and `cmd.Run` does not return
until the child exits. A cancelled checkout therefore still holds up `Shutdown`.

### The likely cause

The test's last step, added in d018a33, starts a second review and returns
straight away. The deferred `Manager.Shutdown` (review/run.go) does three things:
it waits for each run's prepare, closes the sessions that are already live, and
saves. It does not wait for the run's goroutines (`begin`, `pump`, a verifier's
nudge). `pump` still saves the run after its session closes, and a `begin` that
is inside `driver.Start` when `Shutdown` looks at the sessions starts a session
that `Shutdown` never sees.

### Fix when it is picked up

- Have `Manager` track its goroutines and wait for them in `Shutdown`, after
  closing the sessions.
- Have `started` close a session that starts once `m.down` is set. Check and
  set it under `r.mu`, the same lock `Shutdown` reads `s.live` under.

In the app the second gap matters beyond the test: a session that starts while
vuoro is quitting leaves its agent process running.

## Acceptance criteria

- [x] Shutdown returns only after every goroutine the Manager started has exited
- [x] A session that starts after Shutdown has begun is closed and leaves no process
- [x] The local-review test passes under -count with a forced late begin

## Implementation plan

- `Manager.work`, a WaitGroup, counts the session goroutines: begin, pump, and the verifier's nudge. `Manager.spawn` starts each one under `m.mu`, and starts nothing once `down` is set. Shutdown sets `down` under `m.mu` and waits on `work` last, so nothing is counted after the wait has begun.
- `started` closes a session that starts once `down` is set and answers ErrShuttingDown. Shutdown sets `down` before it reads `s.live` under `r.mu`, so every session is either one Shutdown closes or one `started` closes.
- Summaries keep their own `summaryDone`, and prepare keeps `prepared`. Neither changes.
- `TestShutdownClosesASessionThatStartsUnderItAndWaitsForTheRun` holds `driver.Start` until Shutdown has begun. Without the fix it fails, because Shutdown returns before the session has started.

## Notes

**agent:claude/forgejo-parity-20261006** at 2026-10-08T03:20:53Z

Reproduced, and the cause found. This supersedes "The likely cause" in the description, which named the right gap with the wrong evidence.

- **Reproduction.** Under `-race`, in CI's golang:1.27-alpine image with gcc and musl-dev, the test failed 5 times out of 5. The race detector's slowdown lands a late write inside `RemoveAll`. Running it on its own without `-race` never failed.
- **Not git.** A probe in `Repos.git` showed that the only command the shutdown kills is the second run's `git remote set-url`, which starts no child process.
- **The writer.** A listing taken just before the cleanup put the last writes at the two `runs/<id>/run.json` files. Shutdown closes the first run's live session, and its `pump` then saves the record asynchronously, after Shutdown has returned.

## Summary

Fixed on PR #22 with D53. Manager.Shutdown now waits for every session goroutine it started, through a WaitGroup that spawn adds to under m.mu. A session that starts during Shutdown is closed in started. The local-review test passes 20 out of 20 under -race in CI's Alpine image, where it had failed 5 out of 5. TestShutdownClosesASessionThatStartsUnderItAndWaitsForTheRun holds the new behaviour, and the race step in just ci and Forgejo CI keeps it covered.
