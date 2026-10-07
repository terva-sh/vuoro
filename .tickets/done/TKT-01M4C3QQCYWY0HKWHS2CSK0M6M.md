---
schema: 3
id: TKT-01M4C3QQCYWY0HKWHS2CSK0M6M
title: Save a patch of each pushed or hand-linked fix, and show it again
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - review
  - state
  - ui
assignees: []
milestone: null
parent: null
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-07T21:18:58Z
updated_at: 2026-10-07T23:46:06Z
created_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
updated_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
extensions: {}
---

## Description

D51, and docs/plan.md section 6.3, "A fix keeps its patch". A fix is recorded as a SHA and subject. Its diff lives only in the editor's repository, which closing the run drops, and on a branch that goes when it is merged or deleted.

- At each push, PushEditor saves every pushed commit with `git format-patch` from the cache into `runs/<id>/patches/<sha>.patch`, beside run.json. It writes through `state`, so the write is atomic.
- MarkFixed saves the linked commit's patch when the cache has it.
- Each patch has a size cap. Over the cap, no patch is saved; the commit's file list is kept and the patch is marked too large.
- RunView lists the saved patches, by SHA, with the subject and whether the patch was too large.
- A guarded GET route answers one patch, parsed into FileDiffs for the diff viewer, with the raw patch available too.
- Close leaves the patches in place. Discard saves none.
- The window shows a saved patch in the diff viewer from three places: a commit of an earlier wave, "fixed in" on a fixed finding, and a fix branch on the Local tab.

## Acceptance criteria

- [x] Each commit vuoro pushes for the editor is saved as a patch under the run's record
- [x] A commit linked with Mark fixed is saved when vuoro's cache has it
- [x] Patches survive the run's close and the branch's deletion, and discarded work saves none
- [x] A patch over the size cap is not saved, and the run says so with the commit's files
- [x] The window shows a saved patch from an earlier wave's commit, a fixed finding, and a Local fix branch

## Summary

D51 is built. `review/patches.go` holds `savePatches`, `Manager.Patch`, and `parsePatch`.

**Saving.** PushEditor saves every pushed commit, and MarkFixed saves the linked commit. Each is saved with `git format-patch --stdout -1 --no-signature -M` from the repository cache into `runs/<id>/patches/<sha>.patch`, written atomically with mode 0600. A commit the cache does not have is skipped, and saving never fails the push or the link.

**Size cap.** The cap is 1 MiB. A patch over it is not written; its file list (`git show --name-only`) is kept with `too_large`. The run records each saved patch in RunView.patches, which run.json persists.

**Reading.** `GET /api/runs/{id}/patches/{sha}` reads only a SHA the run lists, and goes through the window guard like every route. It answers the raw patch and the diff split into files: renamed, new, deleted, and binary files are handled, and the message is skipped up to the first `---` line. Close leaves the patches in place. Discard saves none, since nothing was pushed.

**In the window.** A shared SavedPatch component offers "Show the diff" in the diff viewer, with "Copy the patch", from three places: a fixed finding's "Fixed in", each commit of an earlier wave, and "Show the fix" on a Local fix branch, which opens a modal. A patch over the cap says it was too large and names its files.

**Tests.**
- Go: TestAFixKeepsItsPatchPastTheRunsClose (push, a large hand-linked commit, then close, restart, and read again), TestAPatchIsSplitIntoItsFiles, TestASavedPatchIsReadAgainFromTheRunRecord (route, unknown and malformed SHAs answer 404), and fix-branch patches in the Local tab test.
- Frontend: web/src/patch.test.tsx.
- A headless screenshot in Light looked right.

Commits pushed before this change have no saved patch, and none is made for them after the fact.
