---
schema: 3
id: TKT-01M63YMZ79QGWB0VD1NJ62A6JJ
title: Summarise a change whose patch is mostly a lockfile, not refuse it
type: bug
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - review
  - agent
assignees: []
milestone: null
parent: null
origin: null
dependencies:
  - TKT-01M61JQ22E7049NSPR545323DW
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-29T13:47:33Z
updated_at: 2026-10-29T18:40:20Z
created_by:
  id: agent:claude-code/ask-plan
  name: ""
updated_by:
  id: agent:claude-code/ask-20261029
  name: ""
extensions: {}
---

## Description

Dogfooding on 2026-10-29: the Summary tab refused a GitLab MR that bumps Python dependencies with "review: this summary range is too large; no partial range was sent" (run 20261029-134612-abfb14). The net patch is 1.5 MB across 3 files, and 5,177 of its lines are uv.lock; requirements.txt and requirements-dev.txt are 245 lines. The 256 KiB cap (review/summary_git.go maxSummaryPatch) is meant to stop vuoro silently truncating a patch, but here it blocks the commonest dependency-bump change, and the part a person needs explained is small.

As built: a patch that fits is sent whole, lockfiles included, since a lockfile edit can be the change that matters. Past the cap, known lockfiles the change's forge base already holds, and files it marks linguist-generated or -diff in .gitattributes, are left out, largest first and only until the rest fits, and the first message names each with its --numstat line counts. What is left still refuses past the cap. A new file with a lockfile's name is sent like any other, so a change that adds a large lockfile the base lacks, such as a repository's first uv.lock, is still refused past the cap; that is the price of not letting a change pick what is hidden. An ask agent can read a left-out file's diff with git_diff (TKT-01M61JQ2FJ4QGFZWPJE2456FQB).

Ask runs (TKT-01M61JQ22E7049NSPR545323DW, done) read their range through the same summaryInput, so they share the cap and the refusal. This ticket fixes both the summary and the ask paths.

## Acceptance criteria

- [x] A change whose patch is over the cap only because of lockfiles or generated files is summarised
- [x] Each file left out of the patch is named in the first message with its stat, and the summary says it was not read
- [x] A change still over the cap after that is refused with the reason, never truncated

## Notes

**agent:claude-code/ask-20261029** at 2026-10-29T15:03:04Z

The work has landed (through b2d38a5). The ticket stays in progress until a vuoro self-review round over 474f7f8..HEAD raises nothing above low and nothing in production code; round 4 (run 20261029-145238-5bc642) did not, and its fixes are b2d38a5.

**agent:claude-code/ask-20261029** at 2026-10-29T16:06:32Z

Criterion 1 is left unticked: a change over the cap only because of a lockfile the forge base already holds, or of generated files, is summarised (TestALockfileIsLeftOutOfTheDiffAndNamed), but one that adds a large lockfile the base lacks is still refused. Counting a new file as a lockfile would let a change choose what is hidden (vuoro review round 5, d7). Closing it needs a way to leave a new lockfile out that the change cannot steer, such as also requiring its manifest to have changed.

**agent:claude-code/ask-20261029** at 2026-10-29T18:40:20Z

Criterion 1 is now met, superseding the note that left it open: D70 (owner, 2026-10-29) stops refusing an ask for its size. Past the cap an ask leaves out lockfiles and generated files first, then the largest other files, so a change that adds a repository's first lockfile is asked, with the lockfile named as left out. Tests: review TestAnOversizedAskLeavesOutTheLargestFilesAndSaysWhich and TestAnAskOverThousandsOfFilesAndHundredsOfCommitsIsAsked. A summary run keeps its refusal, and no new summary can be started. Reviewed over c30f479..2192875; round 6 was clean.

## Summary

Landed in review/summary_git.go (38ed0b8, 8a6b060, b2d38a5, 9bf901c). A patch that fits is sent whole. Past the cap, lockfiles the forge base already holds and files its .gitattributes marks linguist-generated or -diff are left out, largest first and only until the rest fits, and named in the first message with --numstat counts; the summary and ask prompts tell the agent to say it did not read them, and an ask agent can read them with git_diff. What remains past the cap is still refused (summary_test oversize). A new large lockfile is still refused; see the note on criterion 1. Tests: TestALockfileIsLeftOutOfTheDiffAndNamed, TestASmallLockfileEditIsSent, TestAttributesFromTheReviewedCommitHideNothing.
