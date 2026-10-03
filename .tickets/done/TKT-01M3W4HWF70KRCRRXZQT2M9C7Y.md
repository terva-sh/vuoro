---
schema: 3
id: TKT-01M3W4HWF70KRCRRXZQT2M9C7Y
title: Add a vuoro accent to terva-sh/design and release it
type: task
status: done
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
claim: null
archive: null
created_at: 2026-10-01T16:25:25Z
updated_at: 2026-10-03T00:17:32Z
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
- [x] A design release containing it is tagged, and the version is recorded in a note here

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

**agent:codex/vuoro-design-20261002** at 2026-10-03T00:06:33Z

Published design v0.3.0 on both forges, with annotated tag 6bf4ad0cac8fbcfb7616f83eba8b8582b9194c6c pointing to the tested preset commit 8ae5a076158fde2f3afc4799b9cdbe3b45b69c3d. Forgejo release: https://git.local.sothr.com/terva-sh/design/releases/tag/v0.3.0. GitHub module-source release: https://github.com/terva-sh/design/releases/tag/v0.3.0. Both releases are published and neither draft nor prerelease, and tag/commit identities match on both remotes. The full local design gate, including race tests, passed before tagging. Design PR #2 (Add the vuoro violet accent), https://git.local.sothr.com/terva-sh/design/pulls/2, remains open because Forgejo CI is waiting for a runner. Publication uses the tested branch commit and does not merge the PR. Vuoro successfully downloaded v0.3.0, resolved its module checksums, and exported/checked -app vuoro from the real module without a replace or workspace override. This resolves the release criterion and supersedes the earlier unpublished-release notes.

**agent:codex/vuoro-design-20261002** at 2026-10-03T00:17:32Z

The owner merged design PR #2 (Add the vuoro violet accent) at 1c24ed707a4aea192ba39bf1474aa726f34d501a. Fast-forwarded the local main and GitHub main to the merged Forgejo main. The v0.3.0 tag remains unchanged on the tested source commit 8ae5a076158fde2f3afc4799b9cdbe3b45b69c3d; its source is now also in both main branches.

## Summary

The owner-selected violet vuoro preset, generated stylesheet, and comparison docs are published in design v0.3.0 on Forgejo and GitHub, pointing to commit 8ae5a076158fde2f3afc4799b9cdbe3b45b69c3d. Both acceptance criteria are satisfied and local just ci passes. The tagged source branch is in open design PR #2 (Add the vuoro violet accent); hosted CI was still waiting for a runner at publication.
