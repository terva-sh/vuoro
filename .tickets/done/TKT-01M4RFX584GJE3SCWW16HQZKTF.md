---
schema: 3
id: TKT-01M4RFX584GJE3SCWW16HQZKTF
title: Capture the README screenshots with a Playwright script
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - docs
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
created_at: 2026-10-12T16:42:33Z
updated_at: 2026-10-12T16:57:42Z
created_by:
  id: agent:claude-code/screenshots-20261013
  name: ""
updated_by:
  id: agent:claude-code/screenshots-20261013
  name: ""
extensions: {}
---

## Description

The README needs pictures of vuoro before the repository goes public, and they must come from a script, so they can be taken again whenever the UI changes rather than by hand.

### Approach (owner's choice, 2026-10-13)

Playwright drives Chromium against the committed web/dist. Every request is answered inside Playwright: static files from web/dist, and each /api/ route from canned fixtures written in TypeScript and typed by web/src/types.ts, so `just web-typecheck` in CI fails when the API's shape moves away from them. No vuoro binary, forge, token or agent runs. The owner preferred this to running `vuoro -browser` against seeded state, which would need https fake forges with a trusted certificate, a fake claude on PATH, and masking of absolute paths shown in About.

Screens: the inbox, a review run's findings, and the editor's fixes. Each in light and in dark.

Time, timezone, locale, viewport and scheme are fixed. Fonts come from the machine's system stacks, so as with git-ticket-canvas's README images the capture is not a CI gate: two machines render text differently.

## Acceptance criteria

- [x] One command regenerates every README image from committed fixtures, with no forge, token, or agent
- [x] The fixtures are typed by web/src/types.ts and checked by just web-typecheck
- [x] The capture fails on an /api/ request it has no fixture for
- [x] The inbox, a review run, and the editor are each captured in light and dark
- [x] Two runs on one machine write identical images
- [x] The README shows the images

## Implementation plan

Add @playwright/test 1.61.1, pinned as git-ticket-canvas pins it. scripts/screenshots/scene.ts holds the fixtures, typed by web/src/types.ts and the inbox's own InboxView, and is in tsconfig's include so CI typechecks it. scripts/screenshots/capture.ts serves web/dist and the fixtures from page.route on a reserved .invalid origin, so nothing reaches the network, fails on any unanswered /api/ path, freezes the clock with page.clock.setFixedTime, pins UTC, en-US, a 1440x900 viewport and the scheme, then clicks to each screen and writes docs/images/{inbox,review,editor}-{light,dark}.png. just screenshots runs it. The README shows each pair through <picture> with prefers-color-scheme. Determinism is checked by running it twice and comparing hashes.

## Notes

**agent:claude-code/screenshots-20261013** at 2026-10-12T16:48:48Z

Built as planned. just screenshots runs scripts/screenshots/capture.ts with Node 24's type stripping, so there is no build step. It writes docs/images/{inbox,review,editor}-{light,dark}.png at 1440x900, 888 KiB in all.

Determinism: without extra Chromium flags, inbox images differed between runs by about 22 pixels. That was the antialiasing of the rounded borders in the sticky header, which is its own compositor layer. With --disable-gpu, --disable-partial-raster, --disable-skia-runtime-opts, --run-all-compositor-stages-before-draw, --force-color-profile=srgb, --font-render-hinting=none and --disable-lcd-text, six runs wrote one hash per image.

The guard was tested by deleting the GET /api/local fixture. The capture exits 1 and names the route, and it writes no image, because shoot() checks for unanswered requests before every write.

Rejected: a vite preview server (a port to manage, while routing in Playwright needs none), and the Playwright Test runner (a test framework for what is a generator).

Not checked: GitHub rendering the <picture> elements, and how Forgejo shows them. The README's light image is the fallback inside each one.

## Summary

just screenshots runs scripts/screenshots/capture.ts. It shoots the committed web/dist in Playwright's Chromium, with every request answered in Playwright: files from web/dist, and /api/ from scripts/screenshots/scene.ts, which web/src/types.ts types. It writes the inbox, a review run, and the editor, in light and dark, to docs/images, and the README shows each pair through <picture>. Two runs on one machine write identical bytes. Another machine's fonts draw differently, so the capture is not in just ci.
