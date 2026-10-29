---
schema: 3
id: TKT-01M647F9EEBGPXHE5RMF64WHVD
title: Name every place a finding's pattern occurs, not just the first
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - review
  - agent
assignees: []
milestone: v1.0.0
parent: TKT-01M3YMSTYMFRDFXD4QSVWHMM8P
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-29T16:21:45Z
updated_at: 2026-10-29T16:28:28Z
created_by:
  id: agent:claude-code/review-rounds
  name: ""
updated_by:
  id: agent:claude-code/review-rounds
  name: ""
extensions: {}
---

## Description

A reviewer that finds one case of a pattern reports that case and stops. The
person fixes it, the next round finds the next case, and the review takes a
round per case.

The getting-started self-review on 2026-10-29 showed this twice:

- Round 1 found that Enter on the tour's Next button also opened the inbox's
  selected change. Round 2 found the same key leak on the tour offer and on
  every hint's "Got it" button.
- Round 2 found that replaying the tour with a change open skipped the home
  part. Round 3 found that replaying it with no forge went back to setup.

Each pair is one cause in two places. A reviewer that had looked for the
other places when it found the first would have saved a round each time.

The reviewer prompt asks: when a finding is one case of a pattern (a handler,
a state, a check, or a claim repeated elsewhere in the change), look for the
other cases in the change before reporting, and name every place in the one
finding. The anchor stays on the first place, and the body lists the others
with path and line. The synthesizer keeps that list when it merges findings,
and joins the places when two reviewers each found a different case.

## Acceptance criteria

- [x] The reviewer prompt asks for the other cases of a finding's pattern in the change, listed in the same finding
- [x] The synthesizer prompt keeps the list, and joins the places when reviewers found different cases
- [x] A prompt test holds both instructions
- [x] Plan section 6.2 says so

## Implementation plan

Add a paragraph to the reviewer system prompt asking for every case of a finding's pattern in one finding, anchored on the first, the rest listed by path and line. Add to the synthesizer's grouping step that drafts finding different cases of one pattern are one group and the merged draft keeps every place. A test in review/run_test.go holds both. Plan section 6.2.

## Summary

The reviewer prompt now asks for every case of a finding's pattern in the one finding, and the synthesizer groups drafts that found different cases and keeps every place. TestEveryCaseOfAPatternIsNamedInOneFinding holds both; it fails with the reviewer paragraph removed. Plan 6.2 says so, with the getting-started rounds as the reason. Whether reviewers follow it is only seen in live runs.
