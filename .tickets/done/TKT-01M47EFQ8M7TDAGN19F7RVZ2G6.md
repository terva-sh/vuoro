---
schema: 3
id: TKT-01M47EFQ8M7TDAGN19F7RVZ2G6
title: "TestReviewMCPServerIsGuarded fails on Windows: temp dir still in use"
type: bug
status: done
status_reason: null
priority: high
due_on: null
labels:
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
created_at: 2026-10-06T01:50:38Z
updated_at: 2026-10-06T03:58:01Z
created_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
updated_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
extensions: {}
---

## Description

GitHub CI on `main` (run 37398773918, head 9c2234b) fails on windows-latest only:

```
--- FAIL: TestReviewMCPServerIsGuarded (1.88s)
    testing.go:1464: TempDir RemoveAll cleanup: unlinkat C:\Users\RUNNER~1\AppData\Local\Temp\TestReviewMCPServerIsGuarded.../002/repos/local/001-16277a27.git: The directory is not empty.
```

The test's checks pass; its `t.TempDir` cleanup fails. A git child process or a background goroutine still has files open in the local repository cache when the test returns, and Windows refuses to unlink them. Linux and macOS pass. The previous run on main (eac889e) was cancelled, so this may have been red since the editor and command-line review landed.

Find what still has files open at cleanup: an unwaited `exec.Cmd`, a background refresh from `app.background`, or a git maintenance child. Make the test, or the code under it, wait for it. Do not paper over the failure by ignoring cleanup errors.

## Acceptance criteria

- [x] TestReviewMCPServerIsGuarded passes on windows-latest in GitHub CI
- [x] The cause is named in a note, and the fix waits for the process rather than ignoring cleanup errors

## Implementation plan

Run every Repos git command with maintenance.autoDetach=false and gc.autoDetach=false so no detached maintenance outlives it. Add a trace-based regression test. Confirm on windows-latest in GitHub CI.

## Notes

**agent:claude/forgejo-parity-20261006** at 2026-10-06T01:54:50Z

Cause: `git fetch` (git 2.47 here, and the Windows runner's Git for Windows is the same line) ends by spawning `git maintenance run --auto --quiet --detach`. The detached child outlives the fetch that `Repos.git` waited for, inside the cached bare clone `repos/local/<n>.git`. On Linux and macOS a delete under a running process succeeds. On Windows the open handles make `RemoveAll` fail with "The directory is not empty", which is the error `t.TempDir`'s cleanup reported. I confirmed with `GIT_TRACE2_EVENT`: plain fetch shows `child_start [git maintenance run --auto --quiet --detach]`. With `-c maintenance.autoDetach=false -c gc.autoDetach=false` the same child runs `--no-detach` and finishes before the fetch returns.

The same race reaches production as well as the test. A detached maintenance process could still be writing to a clone when `Repos.Remove` takes the lock and prunes, or when the person deletes vuoro's state on Windows.

Fix: `Repos.git` passes both settings on every command, so maintenance still runs, attached. `TestCheckoutLeavesNoDetachedGit` (review/run_test.go) traces a `Checkout` and fails if any maintenance child has `--detach`. I confirmed it fails with the settings removed. It skips on a git that starts no maintenance on fetch.

Alternatives rejected:
- Ignoring the cleanup error in the test: it hides the race, and the ticket refuses it.
- Turning auto maintenance off (`maintenance.auto=false`, `gc.auto=0`): a long-lived cache of large projects would then never be repacked.
- Waiting or retrying `RemoveAll`: that is a timing guess, and it does nothing about the lock race.

Not covered: `summaryGit` only reads, so it never starts maintenance. `review/local.go` runs in the person's own checkout, where their own git config applies. The editor's agent runs git itself in its sandbox; any detached maintenance there belongs to that session.

`just ci` passes on Linux. Acceptance criterion 1 waits for a windows-latest run on GitHub CI.

**agent:claude/forgejo-parity-20261006** at 2026-10-06T02:44:01Z

The fix is in Forgejo PR #15 (https://git.local.sothr.com/terva-sh/vuoro/pulls/15), commit 358c9be. The branch tickets/forgejo-write-spike is also pushed to GitHub, for the windows-latest leg that acceptance criterion 1 needs.

**agent:claude/forgejo-parity-20261006** at 2026-10-06T02:50:36Z

GitHub CI run 37405570542 on head cbb5e38 passed on windows-latest, ubuntu-latest and macos-latest, including TestReviewMCPServerIsGuarded. Forgejo CI and terva-review (run 832d2085, no findings) passed on PR #15. One green run does not prove a race is gone, but the trace test pins the cause.

**agent:claude/forgejo-parity-20261006** at 2026-10-06T03:53:28Z

A second cause of the same Windows symptom. GitHub CI on PR #16 failed TestReviewInterrupt on windows-latest, with the temp dir not empty in repos/local/<n>.git/hooks. Manager.start ran prepare (git init and fetch, then the checkout) in a goroutine that neither Close nor Shutdown waited for. An interrupt that arrived before the checkout finished let git keep writing the clone after the test, or vuoro, had moved on. It could also start reviewer sessions on a run that was already closed, and flip it back to Working. Fixed in PR #16: each run gets a cancellable prepare context and a prepared channel. Close marks the run closed, cancels the checkout and waits for it. Shutdown cancels and waits, and sets Manager.down so a checkout that finishes starts no session. TestClosingARunDuringItsCheckoutStartsNothing holds the clone's lock so that Close lands mid-checkout.

**agent:claude/forgejo-parity-20261006** at 2026-10-06T03:58:01Z

Follow-up from terva-review run 177d0c6d (high) on PR #16. start set stopPrepare and prepared after publishing the run in m.runs, so a concurrent Close or Shutdown could see them nil, skip the wait, and race the assignment. They are now set in the Run literal, before the run is visible. start also refuses a run with ErrShuttingDown once Shutdown has set Manager.down, which it does before listing the runs, so every run is either one Shutdown waits for or one that never starts. TestShutdownWaitsForACheckoutAndRefusesNewRuns holds this.

## Summary

Repos.git runs git's automatic maintenance attached (maintenance.autoDetach=false, gc.autoDetach=false), so no detached maintenance holds the cached clone open after a command. TestCheckoutLeavesNoDetachedGit holds it. Windows CI passed on PR #15.
