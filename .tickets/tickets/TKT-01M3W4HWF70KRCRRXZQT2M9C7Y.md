---
schema: 3
id: TKT-01M3W4HWF70KRCRRXZQT2M9C7Y
title: Add a vuoro accent to terva-sh/design and release it
type: task
status: in-progress
status_reason: null
priority: normal
due_on: null
labels:
  - ui
  - cross-repo
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies: []
blocks_on: none
references: []
claim:
  actor: agent:codex/vuoro-design-20261002
  branch: main
  worktree: /home/sothr/workspace/git.local.sothr.com/terva-sh/vuoro
  commit: 6cd594094a45010bc33180d7e389d24907cdcfe2
  session: null
  claimed_at: 2026-10-02T23:59:40Z
  expires_at: null
archive: null
created_at: 2026-10-01T16:25:25Z
updated_at: 2026-10-02T23:59:41Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:codex/vuoro-design-20261002
  name: ""
extensions: {}
---

## Description

terva-sh/design v0.1.0 has accents for terva, lampi, ketju, and
git-ticket-canvas, and none for vuoro. `design-tokens export -app vuoro` cannot
work until one exists.

This is a change in the design repository, not here: add `vuoro` under
`accents` in `tokens.json`, run `just generate` and `just ci` there (the
contrast test checks both schemes), and tag a release. Pick a hue that is
distinct from the existing four. lampi and ketju are both blue.

Record the design release here when it lands.

## Acceptance criteria

- [x] terva-sh/design has a vuoro accent that passes its contrast test in both schemes
- [ ] A design release containing it is tagged, and the version is recorded in a note here

## Implementation plan

The owner selected violet and authorized committing/pushing both repositories and publishing design v0.3.0. Land the validated violet preset and its docs in design, tag and publish the release on Forgejo and the GitHub module source, then record the version and complete the accent ticket. Finish vuoro adoption by pinning the real release as a Go tool/module, exporting its stylesheet, and adding matching export -check gates to just and both CI workflows. Use the internal Forgejo pull-request process for vuoro.

## Notes

**agent:codex/vuoro-design-20261002** at 2026-10-02T23:54:07Z

The owner requested an accent review before deciding whether to retain blue. Added ../design/docs/vuoro-accent.md, .svg, and .html, linked from its README. The review compares inherited git-ticket-canvas blue, violet (#7546A4 on Birch / #B89AE3 on Tar), and teal (#176B74 / #79C5CE). Violet is recommended because lampi, ketju, and git-ticket-canvas already use blue, Dayroom uses pine/mint, and violet is separate from review status green/amber/red. Blue has no accessibility defect; teal is closer to Dayroom and success colors. Every candidate passes the unchanged shared renderer, contrast, and hover suites when added to a scratch copy of tokens.json. Minimum accent-or-hover contrast on all three grounds: blue 4.94/6.57, violet 5.68/7.10, teal 5.30/8.66 (light/dark). Shared go vet and tests pass with CGO_ENABLED=0. HTML syntax and all six preview variants were checked; the collaborative browser repeatedly failed navigation and snapshot, so no interactive browser verification is claimed. The comparison image was rendered and inspected. No production token, stylesheet, pin, or release has changed while the choice is pending. No commit or push was requested.

**agent:codex/vuoro-design-20261002** at 2026-10-02T23:58:00Z

The owner selected violet, superseding the pending-choice state in the preceding note. Registered accents.vuoro in ../design/tokens.json, generated dist/vuoro.css, updated the README and comparison documents to record the selection, and recorded D32 in vuoro/docs/decisions.md. Light roles: accent #7546A4, hover #603588, on-accent #FFFFFF. Dark: accent #B89AE3, hover #D0B5F2, on-accent #20122F. Shared go vet, renderer, contrast, and hover tests pass with CGO_ENABLED=0. Other apps' generated stylesheets are byte-for-byte unchanged. No design release has been tagged or published; the release acceptance criterion remains open. Both repositories prohibit commits/pushes unless the person asks. Sources and generated assets are ready for review, but remain uncommitted pending that authorization.

**agent:codex/vuoro-design-20261002** at 2026-10-02T23:58:01Z

in-progress to blocked: The violet preset is validated locally; publishing its design release requires authorization to commit and push.

**agent:codex/vuoro-design-20261002** at 2026-10-02T23:59:41Z

The owner explicitly authorized commits/pushes for both repositories and publication of design v0.3.0. The earlier authorization blocker is resolved. The GitHub design main and Forgejo main were verified equal at a49e3ba before publication; neither has v0.3.0 yet.

## Summary

The selected violet vuoro preset and comparison documentation are ready in the sibling design working tree; validation passes. Publishing a version containing the preset remains pending authorization to commit and push.
