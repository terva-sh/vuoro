---
schema: 3
id: TKT-01M3W4HWHRKR936RT5MK8ZHB2B
title: Load the terva-sh design stylesheet, with the export check in CI
type: task
status: review
status_reason: null
priority: normal
due_on: null
labels:
  - ui
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies:
  - TKT-01M3W4HWGGB7RMVFAJPW2KPKY5
  - TKT-01M3W4HWF70KRCRRXZQT2M9C7Y
blocks_on: none
references:
  - ref: pr:https://git.local.sothr.com/terva-sh/vuoro/pulls/2
    path: null
claim: null
archive: null
created_at: 2026-10-01T16:25:25Z
updated_at: 2026-10-03T00:07:28Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:codex/vuoro-design-20261002
  name: ""
extensions: {}
---

## Description

Adopt terva-sh/design as its README describes: `go get` the release that
carries the vuoro accent, export the stylesheet into `web/`, load it before the
app's own styles, and write components against `--ui-*` roles only.

Honour `data-scheme` for light and dark, and leave `data-theme` alone.

## Acceptance criteria

- [x] design-tokens export -app vuoro -check runs in just ci
- [x] The app follows the system scheme, and data-scheme on html overrides it both ways
- [x] No component stylesheet contains a literal colour

## Implementation plan

After the accent choice and shared preset are available, export -app vuoro into web/src/styles/design.css and rebuild web/dist. Preserve existing --ui-* component styling and data-scheme behavior. Once a design release includes vuoro, pin it and add export -check to just ci and both forge workflows together. Do not pin a nonexistent release or introduce a sibling-path replacement that would break standalone CI.

## Notes

**agent:codex/vuoro-design-20261002** at 2026-10-02T23:54:07Z

Read the existing stylesheet and component styles: web/src/main.tsx already loads the shared roles before app.css, and component colors already use --ui-* properties. The current CSS still identifies git-ticket-canvas, so a proper vuoro export is needed after the owner selects the accent. Design v0.2.0 includes Dayroom, but no published version includes vuoro. Keep the module pin and CI gate unchanged until a real release is available rather than inventing a version or requiring ../design in standalone CI. The newly merged vuoro PR #1 (Adopt terva-review and the two-forge process on Forgejo) is now in the working tree and introduces no frontend change.

**agent:codex/vuoro-design-20261002** at 2026-10-02T23:58:01Z

Exported the selected vuoro stylesheet from ../design with design-tokens export -app vuoro. It exactly matches ../design/dist/vuoro.css, loads before app.css, and retains the generated system-scheme and explicit data-scheme selectors. Component styles contain no literal colors. Rebuilt web/dist; just ci passed (Go vet for linux/darwin/windows, tests, six cross-build targets, TypeScript, 23 frontend tests, dist verification, and ticket check), all Go work with CGO_ENABLED=0 and the installed Go 1.27.1. Shared export -check also passes against the local design checkout. Browser automation remained unavailable, so no live-window screenshot is claimed. Updated docs/plan.md and recorded the owner's choice as D32.

The pinned export check is prepared as a patch in task scratch, against go.mod (tool directive plus intended design v0.3.0), justfile (design-export/design-check, the latter in ci), and both forge workflows (the same export -check command). Its applicability and design-check command were verified in an isolated Go workspace referencing the local design checkout. It has not been applied to the real repository because v0.3.0 does not exist yet. After publishing the shared preset, add the design tool/module at the real release, resolve go.sum, apply the matching gates, restore the plan's pinned-adoption statement, and rerun just ci. A fabricated pin or sibling-path replace would break standalone builds, so neither was used.

**agent:codex/vuoro-design-20261002** at 2026-10-02T23:58:01Z

ready to blocked: The stylesheet is exported and validated; the pinned CI export check waits for a published design release containing vuoro.

**agent:codex/vuoro-design-20261002** at 2026-10-03T00:00:46Z

The owner authorized committing and pushing both repositories and publishing design v0.3.0. Design PR #2 (Add the vuoro violet accent), https://git.local.sothr.com/terva-sh/design/pulls/2, contains commit 8ae5a07 and passes the local full just ci gate, including race tests. Forgejo CI is queued. The prepared adoption patch will be applied once the release is published on the GitHub module source.

**agent:codex/vuoro-design-20261002** at 2026-10-03T00:06:52Z

The published design v0.3.0 is now pinned as the design-tokens Go tool in go.mod, with real module checksums in go.sum. Go mod tidy retains the tool dependency, with the Go 1.26 line and tuohi pin unchanged. The export and export -check commands succeed from the downloaded module, without a replace or workspace override. Added design-export/design-check recipes and put design-check in just ci; both Forgejo and GitHub workflows run the same pinned export -check. Updated the README refresh instructions and the design-of-record plan to reflect completed pinned adoption.

**agent:codex/vuoro-design-20261002** at 2026-10-03T00:07:28Z

The completed just ci gate passed against downloaded design v0.3.0 without a workspace override, including the new design-check recipe, the native tests, all six cross builds, all 23 frontend tests, frontend dist verification, and ticket-store validation. The implementation is committed and pushed on vuoro-design in vuoro PR #2 (Adopt the shared violet styling), https://git.local.sothr.com/terva-sh/vuoro/pulls/2. All acceptance criteria are satisfied; the ticket moves to review while hosted CI and the requested terva-review run complete. PRs remain open; no merge was requested or performed.

## Summary

Vuoro PR #2 (Adopt the shared violet styling), https://git.local.sothr.com/terva-sh/vuoro/pulls/2, contains the generated violet preset, rebuilt embedded frontend, the real design v0.3.0 tool/module pin, matching just/Forgejo/GitHub export checks, and README/plan/D32 documentation. All acceptance criteria and the completed local just ci gate pass. Published design v0.3.0 is available on both forges. The PR remains open for hosted checks and review.
