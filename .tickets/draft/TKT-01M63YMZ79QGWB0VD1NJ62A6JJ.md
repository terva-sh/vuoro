---
schema: 3
id: TKT-01M63YMZ79QGWB0VD1NJ62A6JJ
title: Summarise a change whose patch is mostly a lockfile, not refuse it
type: bug
status: draft
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
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-29T13:47:33Z
updated_at: 2026-10-29T13:53:08Z
created_by:
  id: agent:claude-code/ask-plan
  name: ""
updated_by:
  id: agent:claude-code/ask-plan
  name: ""
extensions: {}
---

## Description

Dogfooding on 2026-10-29: the Summary tab refused a GitLab MR that bumps Python dependencies with "review: this summary range is too large; no partial range was sent" (run 20261029-134612-abfb14). The net patch is 1.5 MB across 3 files, and 5,177 of its lines are uv.lock; requirements.txt and requirements-dev.txt are 245 lines. The 256 KiB cap (review/summary_git.go maxSummaryPatch) is meant to stop vuoro silently truncating a patch, but here it blocks the commonest dependency-bump change, and the part a person needs explained is small.

Proposed: the patch sent leaves out lockfiles and generated files (known lockfile names, and paths .gitattributes marks linguist-generated or -diff), and the first message names each one left out with its --stat line, so nothing is dropped without saying so. The cap then applies to what is left, and still refuses past it. Once the Ask context server lands (TKT-01M61JQ2FJ4QGFZWPJE2456FQB), the agent can read a left-out file's diff itself with git_diff, and past the cap the first message can carry the stat and let the agent read per file instead of refusing.

Fold into TKT-01M61JQ22E7049NSPR545323DW if Ask replaces Summary first.

## Acceptance criteria

- [ ] A change whose patch is over the cap only because of lockfiles or generated files is summarised
- [ ] Each file left out of the patch is named in the first message with its stat, and the summary says it was not read
- [ ] A change still over the cap after that is refused with the reason, never truncated
