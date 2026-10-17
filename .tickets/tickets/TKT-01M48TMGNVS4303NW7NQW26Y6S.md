---
schema: 3
id: TKT-01M48TMGNVS4303NW7NQW26Y6S
title: Create and edit reviewer presets on their own page
type: task
status: ready
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
updated_at: 2026-10-17T21:30:54Z
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

- [ ] A preset can be created, edited and removed in the window, and a run can start from it

## Implementation plan

Beyond the one acceptance criterion already on this ticket, the work has to
hold these, which the owner's 2026-10-17 answers imply:

- A built-in preset offers copy and not edit. A copy lands in config.json as
  the person's own, under a name they choose, and from then on it is editable
  like any other.
- Removing a person's preset that shadows a built-in reveals the built-in
  again rather than leaving a gap.
- A preset that no longer resolves, because it names a profile or a tool that
  is gone, says so on the page instead of failing when a run starts.
- The page writes through the same safe save as forges, so a malformed write
  cannot leave config.json unreadable.
- `config.Validate` already refuses a preset with no name, a duplicate name, no
  reviewers, or a reviewer with no profile (`config/config.go:237`). The page
  should refuse those before the save, with the same words, rather than
  surfacing a validation error after the fact.

Profile editing is out of scope here unless the owner moves it. The page is for
presets.
