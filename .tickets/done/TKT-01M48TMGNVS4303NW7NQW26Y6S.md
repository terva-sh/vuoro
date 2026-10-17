---
schema: 3
id: TKT-01M48TMGNVS4303NW7NQW26Y6S
title: Create and edit reviewer presets on their own page
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - ui
assignees: []
milestone: null
parent: TKT-01M3YMSTYMFRDFXD4QSVWHMM8P
origin: null
dependencies:
  - TKT-01M3YMSVJ2BTQN940P1XR3CJD6
  - TKT-01M55VT3H26HX25TB8864NPG6J
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-06T14:42:13Z
updated_at: 2026-10-17T22:33:57Z
created_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
updated_by:
  id: agent:terva/mieli
  name: Mieli
extensions: {}
---

## Description

Split from TKT-01M3YMSVJ2BTQN940P1XR3CJD6 (Set up forges and check agent tools
in the window), whose first slice covers forges and the tool check only, as the
owner chose on 2026-10-06.

Create, edit, and remove reviewer presets in the window: reviewers with a
profile, tool, and model each, plus an optional synthesizer and verifier,
written to config.json through the same safe save as forges. Today a person
hand-writes that JSON after reading docs/plan.md section 8, or runs the one
reviewer `presetsOf` falls back to.

The owner settled the shape on 2026-10-17.

**Presets get their own page with its own tab**, not a section beside forges
and background in the setup window. A preset holds a list of reviewers, each
with three fields, and two optional roles besides. That is more structure than
a forge, and it does not sit well in a settings column.

**A shipped preset is copied, never edited in place.** The page offers copy on
a built-in, and edit on a person's own. This keeps the shipped set meaningful
across an upgrade, so someone who improves `thorough` next release still gets
the improvement rather than a pinned fork of it.

TKT-01M55VT3H26HX25TB8864NPG6J (Ship named built-in presets, not one hardcoded
default) is the dependency and carries the rest of that decision: which presets
ship, and that a person's own supplement them rather than replacing them.
Building this editor first would mean building it against a composition rule
that is about to change.

## Acceptance criteria

- [x] A preset can be created, edited and removed in the window, and a run can start from it

## Implementation plan

### Where it lives, settled 2026-10-17

The owner chose "its own page with its own tab" on 2026-10-17, and this
codebase has two things called tabs, so the reading was settled separately:
the Settings side nav gains a Presets entry, and choosing it swaps the
settings body for a dedicated page instead of scrolling to another section.

The rejected readings and why:

- A third home tab beside Reviews and Local. `HomeTab` is a real tablist with
  "needs you" badges and a preference validated server-side against
  `app.Tabs`, with `app/prefs_test.go` asserting an unknown tab is refused.
  Presets are configuration, not work, so they would inherit a badge slot
  with no number and cost a Go change for the privilege.
- A second header button beside the Settings gear. Most literally its own
  page, and it clutters the one piece of chrome every screen shows.

Nothing in the Go navigation changes. No new preference, no change to
`app.Tabs`.

### Backend

`setupView` serves `[]presetView`, a `config.Preset` embedded with a
`builtin` flag, which flattens in JSON the way `toolView` already embeds
`probe`. The flag cannot live on `config.Preset`, which serializes into the
person's config.json and would write `builtin: true` into their file.

Two routes beside the forge ones, both through `editConfig`, which already
reads the file, refuses a base hash that moved, saves, reloads, and answers
with the fresh view:

- `POST /api/setup/presets` with `{base, original, preset}`. An empty
  `original` creates, otherwise it replaces the preset of that name, which is
  also how a rename lands.
- `POST /api/setup/presets/remove` with `{base, name}`.

Removing a preset that shadowed a built-in needs no special case: `presetsOf`
merges on every read, so the shipped one reappears by itself (D61).

Validation mirrors `config.Validate`'s preset rules, in the window's words
rather than the file's, so a refusal arrives before the save instead of as a
parse error after it. Taking a built-in's name is allowed and is how
shadowing is expressed, so only a collision with another of the person's own
presets is refused.

### The page

A list of presets. The person's own carry Edit and Remove. A shipped one
carries Copy alone and no Edit, which is D61: copying keeps the shipped set
meaningful across an upgrade, where editing in place would pin a private fork
of `thorough` at the version they first touched it.

The form is a name, a list of reviewers each with a profile, an optional tool
and model, and an optional synthesizer and verifier. The profile choices come
from `setup.profiles`, which already carries the shipped three plus the
person's own.

### Out of scope

A preset that names a profile or tool that no longer resolves should say so on
the page rather than failing when a run starts. Worth doing, not required by
the one criterion here, and it wants its own ticket if this one grows.

## Notes

**agent:terva/mieli** at 2026-10-17T22:32:50Z

Task worklog for this ticket, from the session task board.

### Tasks

- [x] task-15 A preset can be created, edited and removed in the window, and a run can start from it — Created, edited, and removed on a page of its own, reached from the Settings nav (web/src/presets.tsx, settings.tsx), writing through POST /api/setup/presets and /presets/remove on the same editConfig safe save as forges. Proven end to end by TestThePresetsPageCreatesEditsAndRemoves, which adds a preset, renames it, removes it, and in the middle calls a.Plan("mine") to show the saved preset actually starts a run with the security reviewer on opus. The page side is covered by 9 tests in web/src/p…

## Summary

Landed as a page reached from the Settings nav, which swaps the settings body
for it rather than scrolling to another section. "Its own tab" was ambiguous
in this codebase, because `HomeTab` is a real tablist with badges and a
server-validated preference while the Settings nav is anchor links, so the
reading was put to the owner before any code was written. Nothing in the Go
navigation changed: no new preference, no change to `app.Tabs`.

`setupView` now serves `[]presetView`, a `config.Preset` embedded with a
`builtin` flag, flattening in JSON the way `toolView` already embeds `probe`.
The flag deliberately does not live on `config.Preset`, which serializes into
the person's config.json and would write `builtin: true` into their file.

`POST /api/setup/presets` and `/presets/remove` both go through `editConfig`,
so they inherit the forge path whole: read, refuse a base hash that moved,
save, reload, answer with the fresh view. An empty `original` creates and
otherwise it replaces the preset of that name, which is also how a rename
lands.

Two things fell out of D61 rather than needing code. Removing a preset that
shadowed a shipped one brings the shipped one back by itself, because only the
person's own are in the file and `presetsOf` merges on the read the save
answers with. And taking a shipped name is allowed, since that is how
shadowing is written, so only a collision with another of their own is
refused.

`cleanPreset` mirrors `config.Validate` in the window's words, so a mistake is
a message rather than a file that will not load next start. `cleanRole` drops
a synthesizer or verifier naming neither tool nor model, which matters: the
form always sends both, and an empty `&Reviewer{}` would pin an empty tool
where `Plan` otherwise picks up the reviewer's (D11, D31).

Worth knowing for the next page in this area. jsdom only submits a form that
is attached to the document, so a test helper that renders into a detached div
silently never calls the server and the failure reads as "called 0 times".
`web/src/local.test.tsx:12` already carries the comment; I rediscovered it the
slow way. Two tests caught it.

`just ci` passes: 165 frontend tests across 29 files, `web/dist` rebuilt and
matching.

Left for a ticket of its own, as the plan said: a preset naming a profile or
tool that no longer resolves should say so on the page instead of failing when
a run starts.
