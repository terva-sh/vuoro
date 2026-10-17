---
schema: 3
id: TKT-01M561J4A1AB61CS7FP6J6TJDD
title: Warn on the presets page when a profile or tool no longer resolves
type: bug
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - ui
  - review
assignees: []
milestone: null
parent: TKT-01M3YMSTYMFRDFXD4QSVWHMM8P
origin: null
dependencies:
  - TKT-01M48TMGNVS4303NW7NQW26Y6S
blocks_on: none
references:
  - ref: plan:design-of-record
    path: docs/plan.md
claim: null
archive: null
created_at: 2026-10-17T23:01:13Z
updated_at: 2026-10-17T23:01:13Z
created_by:
  id: agent:terva/mieli
  name: Mieli
updated_by:
  id: agent:terva/mieli
  name: Mieli
extensions: {}
---

## Description

A preset names a profile and may name a tool. Neither name is checked until
the person starts a review, where `Manager.Start` fails with
`review: no profile named %q` (`review/run.go:221`) or the error from
`m.driver(rv.Tool)` just below it. The person learns on the way into a review
that the preset they chose cannot run.

Nothing malformed has to happen for this. A preset goes stale when they remove
a profile from `config.profiles` while a preset still names it, when they edit
config.json by hand, or when a tool that a preset pins is uninstalled or stops
probing. `config.Validate` does not catch it, and should not: a preset naming
a tool that is absent today is still correct configuration on the machine
where that tool is installed.

There is a sharper half, found while writing this up. The editor's profile
input is `<select value={r.profile}>` (`web/src/presets.tsx:72`), and a select
whose value matches no option renders as the first option. So a preset naming
a profile that is gone opens showing "Code", and saving the form writes `code`
into the preset. The page does not report the dangling name, it quietly
replaces it. The tool select has the same shape (`presets.tsx:114`), where a
missing match falls to the "Any tool available" empty option, which at least
degrades to the behaviour the person would want.

The data to check against is already on the page. `setupView` serves
`Profiles []review.Profile` and `Tools []toolView`, each tool carrying its
`State` (`app/setup.go:322-331`). So this needs no new route and no Go change
beyond tests, unless the warning wants to distinguish a tool that is installed
but signed out, which `toolView.State` already names (D60).

Scope note. This is about reporting, not about refusing. A preset that names a
tool absent on this machine must stay saveable and stay in the file, because
the person may run vuoro on more than one machine. The page says it cannot run
here. It does not repair it for them.

## Acceptance criteria

- [ ] A preset naming a profile or tool that does not resolve on this machine is marked as such in the presets list, naming what is missing
- [ ] Opening such a preset in the editor shows the name the preset actually holds, rather than silently selecting the first option
- [ ] Saving a preset with a dangling name round-trips it unchanged, so a preset for another machine survives an edit
- [ ] A tool that is installed but signed out reads differently from one that is absent

## Implementation plan

### What resolves

A reviewer's profile resolves if it names one of `review.Builtin` or one of
`cfg.Profiles`, which is what `Manager.profile` checks (`review/run.go:1045`).
A reviewer's tool resolves if it is empty, meaning any available tool, or if
it names a probed tool. Mirror those two, and do not invent a third rule.

### The page

Mark the preset in the list when any of its reviewers, its synthesizer, or its
verifier names something that does not resolve, and say which name and which
kind. A shipped preset cannot have a dangling profile, which
`TestBuiltinPresetsNameProfilesThatExist` holds, but it can pin a tool that is
not installed, so do not skip the builtins.

In the editor, fix the silent coercion first. When `r.profile` matches no
option, render an option for the dangling name itself, selected and marked as
unavailable, so the form shows what the preset says and a save round-trips.
The same for the tool select. Then the warning text sits beside it.

Decide with the owner whether `canSave` should stay true with a dangling name
in the form. I lean yes, for the multi-machine reason in the description, with
the warning carrying the weight.

### Tests

`web/src/presets.test.tsx` for the page: a preset naming a gone profile is
marked in the list, the editor shows the dangling name rather than the first
option, and a save round-trips it unchanged. Note that jsdom only submits a
form attached to the document, which `web/src/local.test.tsx:12` records and
which cost real time on the page ticket.

A Go test only if the view gains a field. Prefer computing this in the page
from what `setupView` already serves.
