---
schema: 3
id: TKT-01M561J4A1AB61CS7FP6J6TJDD
title: Warn on the presets page when a profile or tool no longer resolves
type: bug
status: done
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
updated_at: 2026-10-17T23:21:20Z
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

- [x] A preset naming a profile or tool that does not resolve on this machine is marked as such in the presets list, naming what is missing
- [x] Opening such a preset in the editor shows the name the preset actually holds, rather than silently selecting the first option
- [x] Saving a preset with a dangling name round-trips it unchanged, so a preset for another machine survives an edit
- [x] A tool that is installed but signed out reads differently from one that is absent

## Implementation plan

Decided with the owner on 2026-10-18, after reading the resolution path. The
four answers are in the note below.

### What resolves, exactly

A profile resolves if `setup.profiles` holds it. That array is
`review.Builtin` plus `cfg.Profiles` (`app/setup.go:331,349`), which is the
same set `Manager.profile` concatenates (`review/run.go:1046`). So the page
can decide this without a new field.

A tool is the part I had wrong when filing. `probeTools` puts a driver in
`offered` only when its probe returned no error (`app/app.go:179-184`), and
`toolState` returns `signed_out` with a nil error. So:

- `missing`, `too_old`, `broken`: not in `Manager.Drivers`, and `Start`
  refuses before the run begins. Hard.
- `signed_out`: offered, so the run starts and the session fails at
  authentication. Soft, and it reads differently.
- a name no probe knows at all: hard.
- empty: any tool available, never the preset's fault.

### The page

`troubles(p, setup)` returns what a preset names that this machine cannot
resolve, deduped by name, each marked hard or soft. Render it under
`describe(p)` on the card, hard as `warn` and soft as `muted`. The built-ins
are included, because a shipped preset cannot have a dangling profile, which
`TestBuiltinPresetsNameProfilesThatExist` holds, but it can pin a tool.

In the editor, fix the coercion. When `r.profile` matches no option, render an
option for the dangling name itself so the select shows what the preset
holds. The same in `ToolField`. Mark every tool option that cannot run, so
pinning one is a choice rather than a surprise later.

`canSave` does not change. A preset for another machine stays saveable, which
is the owner's ruling and goes in docs/decisions.md as D62, because it is
exactly the kind of rule a later reader would "fix" by adding validation.

### Out of scope, filed separately

`Manager.driver` falls back to the only registered driver when the tool is
empty, and only when there is exactly one (`review/run.go:1062`). vuoro
registers claude alone today (`main.go:194`), so it works. A second driver
breaks every preset that leaves the tool empty.

### Tests

`web/src/presets.test.tsx`: a card names a gone profile and a tool that
cannot run, a signed-out tool reads differently from an absent one, the
editor shows the dangling name rather than the first option, and a save
round-trips it unchanged. jsdom only submits a form attached to the document,
which `web/src/local.test.tsx:12` records.

No Go change, so no Go test beyond what exists.

## Notes

**agent:terva/mieli** at 2026-10-17T23:16:52Z

Four decisions from the owner, 2026-10-18.

1. A preset naming a profile or tool absent on this machine stays saveable,
   with the warning showing. Refusing it would make the page unable to
   express a configuration they wrote by hand for another machine.
2. A signed-out tool and an absent tool both get said, in different words.
   Absent reads "cannot run here". Signed out reads as needing a sign-in, and
   is the softer mark, because the tool does resolve and the run fails later.
3. The tool select keeps listing every probed tool, and marks the ones that
   cannot run in the option text.
4. The `Manager.driver` empty-name fallback is filed as its own bug and left
   alone here.

The thing I had wrong when filing this ticket, corrected above: I assumed a
signed-out tool fails the same way an absent one does. It does not.
`probeTools` offers a driver whenever the probe returned no error, and
`toolState` calls a probe with no account `signed_out` with a nil error. So a
signed-out tool is in `Manager.Drivers`, `Start` accepts it, and the failure
lands on the session rather than on the run.

**agent:terva/mieli** at 2026-10-17T23:21:16Z

Task worklog for this ticket, from the session task board.

### Tasks

- [x] task-22 A preset naming a profile or tool that does not resolve on this machine is marked as such in the presets list, naming what is missing — troubles() in web/src/presets.tsx returns what a preset names that does not resolve, rendered under describe(p) on each card, hard as warn and soft as muted. Covered by "names the profile that is gone", "names a tool this machine never probed", "marks a shipped preset too", "says each missing name once", and "says nothing about a preset that resolves" in web/src/presets.test.tsx.
- [x] task-23 Opening such a preset in the editor shows the name the preset actually holds, rather than silently selecting the first option — The profile select and ToolField each render an option for a value no option matches, so the dangling name shows instead of another. Verified by sabotage: with the option removed, "shows the name the preset holds rather than the first option" fails with "expected '' to be 'vibes'". Restored, 18 of 18 pass.
- [x] task-24 Saving a preset with a dangling name round-trips it unchanged, so a preset for another machine survives an edit — canSave is unchanged, so a dangling name does not block a save. "round-trips the dangling name through an edit of another field" renames "old" to "older" and asserts post carries profile: "vibes" untouched. Recorded as D62, since a later reader would otherwise add validation here.
- [x] task-25 A tool that is installed but signed out reads differently from one that is absent — toolTrouble marks signed_out as the only soft state, worded "is not signed in, so a run would start and then fail" against "cannot run here" for missing, too_old, and broken. "reads differently for a tool that is signed out and one that is absent" asserts both, including that the signed-out card does not say "cannot run here".

## Summary

Landed as D62, in `web/src/presets.tsx` alone. No Go change, no new route,
no new field on the view: `setupView` already serves `profiles` and `tools`
with each tool's state, so the page can decide this with what it has.

`troubles(p, setup)` returns what a preset names that this machine cannot
resolve, deduped by name. The card shows it under `describe(p)`, hard as
`warn` and soft as `muted`. The selects each render an option for a value no
option matches, and every tool option that cannot run says why.

Two things in the description above are wrong, and both were corrected by
measuring rather than reading.

**A signed-out tool is not the same failure as an absent one.** I assumed it
was when I filed this. `probeTools` offers a driver to the run manager
whenever its probe returned no error (`app/app.go:179-184`), and `toolState`
calls a probe with no account `signed_out` with a nil error. So missing, too
old, and broken never reach `Manager.Drivers` and `Start` refuses before the
run begins, while a signed-out tool is offered, the run starts, and the
session fails at authentication. The page words them differently because of
this, which is what the fourth criterion was reaching for.

**The save did not write `code` into the preset.** I claimed it did. It does
not: Preact state holds the reviewer the preset came with, the select's
`onChange` never fires, and the save round-trips the real name. I found this
by removing the fix and watching which tests failed. Only the one asserting
the displayed value failed, and the round-trip test passed without the fix.

So the defect is narrower than filed and, I think, nastier. The form showed a
profile the preset does not name. The person confirms what the form shows
them and saves something else. A silent rewrite would at least leave the file
agreeing with the screen.

The sabotage run is worth repeating on the next change here, because it is
what separated the two claims: remove the dangling option, and
`shows the name the preset holds rather than the first option` fails with
`expected '' to be 'vibes'`. Note the empty string. jsdom returns no value
rather than the first option's value, so a test asserting the coercion picks
`code` would itself be wrong.

Rejected, and recorded in D62 because it is the obvious wrong turn: putting
this check in `config.Validate`. That runs on load on every machine, so a
config naming a tool installed on one of them would stop loading on the
others. Resolution is a fact about this machine, so it belongs where the
machine is known.

`just ci` passes: 29 test files, 174 frontend tests, up 9, and `web/dist`
matches a fresh build.

Filed and not fixed: TKT-01M562ESB0WGCRWJJW0QYTBZE6, the `Manager.driver`
empty-name fallback that works only while exactly one driver is registered.
