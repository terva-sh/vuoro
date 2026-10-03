---
schema: 3
id: TKT-01M3W4HWHRKR936RT5MK8ZHB2B
title: Load the terva-sh design stylesheet, with the export check in CI
type: task
status: done
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
updated_at: 2026-10-03T00:17:33Z
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

**agent:codex/vuoro-design-20261002** at 2026-10-03T00:08:38Z

Requested terva-review for vuoro PR #2 (Adopt the shared violet styling) using request-id vuoro-design-v030-ready. Tea reported its documented unexpected end of JSON input error; the dispatch was nevertheless accepted, as verified by Forgejo run 19762 at https://git.local.sothr.com/terva-sh/vuoro/actions/runs/10, workflow terva-review.yml, status waiting. No duplicate request was submitted. Hosted CI for the PR is also waiting for a runner. Pushed vuoro-design to the existing GitHub remote as well so the mirror workflow can exercise Go 1.26, macOS/Windows, and the native-window test. Both PRs remain open and are registered with the thread. All intended code, generated assets, and ticket records are committed and pushed; the release/module adoption is complete locally. Hosted findings, if any, still need dispositions before merging.

**agent:codex/vuoro-design-20261002** at 2026-10-03T00:17:32Z

The owner merged vuoro PR #2 (Adopt the shared violet styling) at c874f63aae096a97f091785033f9a615046fcf93. Fast-forwarded the local main and synchronized GitHub main with just sync-github --yes. The requested model review completed cleanly on the final PR head 68ff96c709bfbd9d96709b4b976bf4c77f131148, with no findings: https://git.local.sothr.com/terva-sh/vuoro/pulls/2#issuecomment-24654, Actions run https://git.local.sothr.com/terva-sh/vuoro/actions/runs/10. Generated frontend assets and the external design implementation were outside that model review; local build/export tests and design CI covered those artifacts.

GitHub run https://github.com/terva-sh/vuoro/actions/runs/37080789715 passed Linux and macOS, including the native-window check, but failed four existing Windows test assertions (Unix mode 0600, LF-versus-CRLF in the documented example and Git-apply fixture, and a forward-slash path suffix). The four test files are unchanged between base 6cd5940 and merge c874f63. Filed TKT-01M3ZHY78HXTVPBC8J1QY77BER (Make the Windows CI tests portable across modes and newlines) as a labeled draft with failure evidence and acceptance criteria; it has not been promoted. The styling adoption itself is merged and all of its acceptance criteria are satisfied.

## Summary

Merged through vuoro PR #2 (Adopt the shared violet styling) at c874f63aae096a97f091785033f9a615046fcf93; both forge main branches are synchronized. Vuoro uses the generated violet preset from design v0.3.0, pins its Go exporter, checks stylesheet drift in just/Forgejo/GitHub CI, and carries rebuilt frontend assets plus README/plan/D32 documentation. Local just ci passed; the model review was clean; GitHub Linux and macOS (including GUI) passed. Existing Windows test-portability failures are recorded separately in draft TKT-01M3ZHY78HXTVPBC8J1QY77BER (Make the Windows CI tests portable across modes and newlines).
