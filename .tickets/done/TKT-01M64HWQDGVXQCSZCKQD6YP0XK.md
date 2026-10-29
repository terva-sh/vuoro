---
schema: 3
id: TKT-01M64HWQDGVXQCSZCKQD6YP0XK
title: Tell a following review what was decided on the earlier findings
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - review
assignees: []
milestone: v1.0.0
parent: TKT-01M3YMSTYMFRDFXD4QSVWHMM8P
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-29T19:23:51Z
updated_at: 2026-10-29T21:36:18Z
created_by:
  id: agent:claude-code/release-0.1.0
  name: ""
updated_by:
  id: agent:claude-code/release-tooling
  name: ""
extensions: {}
---

## Description

A run that follows an earlier one (`-follows`, or Review again in the
window) gives its reviewers each earlier finding with where it stands. A
headless self-review has no Keep or Drop step, so every earlier finding
arrives as "open: nobody had decided", even one the person fixed in a
commit or rejected with a reason. The reviewers then check each again, and
raise a rejected one again.

Let the command line record what was decided on the earlier run before the
next round, for example
`vuoro review -follows <run> -decided d4=fixed,d5=dismissed:"reason"`, or a
`vuoro decide <run> ...` that writes to the run's record. A dismissed
finding then reaches the reviewers as dismissed, with the reason, and a
fixed one as fixed, so neither is raised again without new evidence.

Seen in the 2026-10-29 self-reviews: rounds re-checked findings that had
been fixed, and nothing could tell them a finding was set aside on purpose.

## Acceptance criteria

- [x] The command line can record a finding of an earlier run as fixed, with its commit, or dismissed, with a reason
- [x] A following run's reviewers receive that standing and reason
- [x] The vuoro-review skill's round script passes it

## Implementation plan

A repeatable -decided flag on vuoro review, id=fixed:commit or id=dismissed:reason, valid only with -follows. Manager.RecordDecided checks every decision first, then writes them into the earlier run's record before Follow reads it: a dismissal through Store.Dismiss with the reason on the draft (Draft.Dismissed), a fix through MarkFixed after resolving a commit prefix among FixCommits. The decisions persist, so a later round following the same run needs no -decided again. EarlierOf carries the reason for a dismissed finding only, the reviewers' first message gives it as reason=, a restated dismissed finding inherits it, and the Markdown standing quotes it. The vuoro-round script lives in the dotfiles repo, outside this store; its -decided pass-through is dotfiles commit d1386d2.

## Notes

**agent:claude-code/release-tooling** at 2026-10-29T21:30:06Z

Review round 1 (standard, $0.52): d8 medium, Undecide kept a stale reason, fixed with a store test; d5 low, a late refusal left earlier decisions recorded, fixed by checking all first; d6 and d7 low, plan wording, fixed. No second round, per the short-rounds instruction.

## Summary

vuoro review -decided id=fixed:commit | id=dismissed:reason, with -follows, records decisions in the earlier run's record; the following run's reviewers get each standing and a dismissal's reason, and the Markdown quotes it. vuoro-round passes it through (dotfiles d1386d2).
