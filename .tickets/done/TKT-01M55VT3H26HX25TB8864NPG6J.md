---
schema: 3
id: TKT-01M55VT3H26HX25TB8864NPG6J
title: Ship named built-in presets, not one hardcoded default
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - review
  - ui
assignees: []
milestone: null
parent: TKT-01M3YMSTYMFRDFXD4QSVWHMM8P
origin: null
dependencies: []
blocks_on: none
references:
  - ref: code:presets-default
    path: app/app.go
  - ref: code:builtin-profiles
    path: review/profile.go
  - ref: code:preset-picker
    path: web/src/local.tsx
claim: null
archive: null
created_at: 2026-10-17T21:20:43Z
updated_at: 2026-10-17T22:14:14Z
created_by:
  id: agent:terva/mieli
  name: Mieli
updated_by:
  id: agent:terva/mieli
  name: Mieli
extensions: {}
---

## Description

`presetsOf` in `app/app.go:199` returns one preset when the config names none:

    []config.Preset{{Name: "default", Reviewers: []config.Reviewer{{Profile: "code"}}}}

So a person who has not hand-written `presets` in config.json gets a single
code reviewer, and the preset picker on the Review tab shows one entry with
nothing to pick between. `web/src/local.tsx:62` hides the picker entirely below
two presets, which is the honest thing to do with a list of one.

vuoro already ships three profiles, code, security, and documentation
(`review/profile.go:20`), and the machinery for several reviewers, a
synthesizer, and a verifier is built and tested. The presets that would combine
them are the missing piece. A person has to read docs/plan.md section 8, learn
the config schema, and hand-write JSON before they can run a review that is
anything other than one code reviewer.

### The asymmetry to settle

Profiles and presets compose differently today, and the difference looks
accidental rather than chosen.

`review/run.go:1046` resolves a profile over `slices.Concat(m.Profiles, Builtin)`,
so a person's profiles supplement the built-ins and a same-named profile
shadows one. `presetsOf` takes the config's list whole when it is non-empty, so
a person who writes one preset loses the default. Whichever rule built-in
presets follow should be the one stated, because a person who adds a preset and
silently loses the shipped ones has lost work they did not know they had.

### What it touches

- `review_cmd.go:196` already treats `quick` as a name with meaning: `vuoro
  review` with no `-preset` runs it when the config defines it. Shipping a
  preset named `quick` makes that path fire for everyone, which is probably
  right and is certainly a behaviour change.
- D14 says presets are the person's, with built-in profiles. Shipping built-in
  presets does not contradict it, but it fills in a part D14 left open, so it
  wants its own number in docs/decisions.md.
- TKT-01M48TMGNVS4303NW7NQW26Y6S (Build reviewer presets in the window) is the
  editor for these. It needs an answer on whether a built-in preset can be
  edited in place, copied and then edited, or neither.

## Acceptance criteria

- [x] vuoro ships more than one preset, and a fresh install with no config.json offers all of them in the Review tab picker
- [x] The rule for how a person's presets meet the built-in ones is stated in docs/plan.md and holds in a test
- [x] vuoro review with no -preset runs the same preset the window would, on a config that names none
- [x] An existing config that names its own presets keeps running exactly as it does now

## Definition of done

- [x] A numbered decision in docs/decisions.md records the set and the rule
- [x] just ci passes
- [x] docs/plan.md section 5.1 describes the shipped presets and the composition rule, with the command line's default in 6.7

## Implementation plan

The owner settled the three open questions on 2026-10-17. They are decisions,
not suggestions, and docs/decisions.md gets the next number for them.

### What ships

`config.Builtin`, beside the `config.Preset` type it is made of:

- **quick.** One code reviewer. What `presetsOf` returns today, given a name,
  so the current default keeps working and `vuoro review` finds it.
- **standard.** Code and security, with a synthesizer.
- **thorough.** Code, security, and documentation, with a synthesizer and a
  verifier. The only shipped preset that exercises verification (D31), which
  is built and which nobody currently meets without writing config.

### The merge, and why the person's come first

`presetsOf` stops replacing and starts merging, which makes it agree with the
profile lookup at `review/run.go:1046` instead of contradicting it. A preset
the person names themselves shadows a built-in of the same name rather than
appearing twice.

The person's presets sort first and the built-ins follow. That ordering is not
cosmetic, it is what keeps the fourth criterion. The window's default is
`ps[0]` (`app/app.go` Plan), so the person's own first preset stays the
default exactly as it is today.

### The default, which is the subtle part

`review_cmd.go:196` reads `a.Config.Presets`, the parsed config file, and
prefers a preset named `quick` when it finds one there. That line must keep
reading the raw config and not the merged list. If it read the merged list,
`quick` would always be present and the command line would start preferring it
for someone whose config names other presets, which is a behaviour change the
fourth criterion forbids.

With that held, the four cases are:

| config | window default | `vuoro review` default |
|---|---|---|
| none | quick, first of the built-ins | quick, via the first preset |
| names quick | the person's first | quick, as today |
| names presets, no quick | the person's first | the person's first |

The first row is the third criterion, and the window and the command line
agree there. Rows two and three are unchanged from today, which is the fourth.
Row two disagrees with itself, and it already does: that divergence predates
this ticket, the fourth criterion protects it, and fixing it is somebody's
decision rather than a tidy-up to make in passing.

### Deliberately not done

No `builtin` flag on the preset view. TKT-01M48TMGNVS4303NW7NQW26Y6S needs one
to offer copy on a shipped preset and edit on the person's own, but nothing
here uses it, and `config.Preset` is the type that serializes into the
person's config.json, so the flag cannot live on it without writing
`builtin: true` into their file on save. It wants a view type in
`app/setup.go`, which the editor ticket should add where its own tests
exercise it.

## Notes

**agent:terva/mieli** at 2026-10-17T22:14:07Z

Task worklog for this ticket, from the session task board.

### Tasks

- [x] task-11 vuoro ships more than one preset, and a fresh install with no config.json offers all of them in the Review tab picker — config.Builtin ships quick (code), standard (code + security), and thorough (code + security + documentation). presetsOf merges them in, so a config-less install resolves all three: TestPresetsSupplementTheBuiltInOnes case 'a fresh install gets what vuoro ships' asserts exactly [quick standard thorough], and 'no configuration at all is the same' covers a nil config. The pickers need no change to show them: web/src/local.tsx renders its selector when presets.length > 1, which is now true with no…
- [x] task-12 The rule for how a person's presets meet the built-in ones is stated in docs/plan.md and holds in a test — Stated in docs/plan.md section 5.1, on the `presets` bullet, which is where the config fields are actually documented rather than section 8 as the DoD guessed: it names the three shipped presets, says a person's supplement rather than replace, that a shared name shadows, and that theirs sort first so the window's default stays the one they wrote. Section 6.7 carries the command line's half. Held by TestPresetsSupplementTheBuiltInOnes, a five-case table covering fresh install, nil config, theirs-…
- [x] task-13 vuoro review with no -preset runs the same preset the window would, on a config that names none — Both halves are tested against the same place. TestReviewOnAFreshInstall runs the command with a nil config and asserts the output holds 'reviewer code on fake' and neither 'security' nor 'documentation', so quick's single code reviewer ran. app.TestPlanOnAFreshInstall asserts the window's Plan("") on an empty App gives one code reviewer, no synthesizer, and a verifier. The agreement is structural rather than coincidental: with no config file review_cmd.go's quick loop finds nothing, so both fal…
- [x] task-14 An existing config that names its own presets keeps running exactly as it does now — Three paths could have changed for an existing config, and each is pinned. The command line: review_cmd.go still matches 'quick' against a.Config.Presets, the parsed file, never the merged list, so TestReviewKeepsAnExistingConfigurationsDefault (a config naming only 'mine') still runs documentation rather than the shipped quick's code reviewer, and the pre-existing TestReviewQuickPresetIsTheDefault passes unchanged. The window: theirs sort first in presetsOf, so Plan's ps[0] default is still the…

## Summary

Landed as D61. `config.Builtin` ships quick (one code reviewer), standard
(code and security), and thorough (code, security, and documentation).
`presetsOf` stopped replacing and started merging, so a person's presets
supplement the shipped ones as their profiles already do, and one of theirs
shadows a built-in of the same name instead of appearing beside it.

None of the three names a synthesizer or a verifier. `Plan` already adds a
synthesizer above one reviewer (D11) and a verifier always (D31), on the tool
and model that drafted or merged the findings. Naming them in the preset would
have pinned an empty tool where those defaults pick up the reviewer's, which
is worse than saying nothing.

Two ordering facts carry the whole change, and both are easy to get wrong.

The person's presets sort first. The window's default is `ps[0]`, so putting
the built-ins first would have changed which preset an existing config starts
with.

`review_cmd.go` still matches `quick` against `a.Config.Presets`, the parsed
file, and not the merged list. vuoro now ships a preset by that name, so
reading the merged list would have made the command line start preferring it
for anyone whose config names other presets. That line looks like a candidate
for tidying into `presets()`, and tidying it would be a behaviour change. The
comment there says so.

With both held, a config-less install gets quick from the window and quick
from the command line, which is the agreement the third criterion asked for,
and an existing config does exactly what it did before.

Not done, deliberately: no `builtin` flag on the preset view.
TKT-01M48TMGNVS4303NW7NQW26Y6S needs one to offer copy on a shipped preset and
edit on the person's own, but nothing here uses it, and it cannot live on
`config.Preset`, which serializes into the person's config.json and would
write `builtin: true` into their file on save. It wants a view type in
`app/setup.go`, added where the editor's own tests exercise it.

One thing a later reader should not mistake for an oversight. A config that
names its own `quick` still diverges between the two: the window runs their
first preset and the command line runs their `quick`. That predates this
ticket and the fourth criterion protects it, so it is left alone. Settling it
is a decision of its own, and D61 records that.

The documentation landed in docs/plan.md 5.1, where the config fields are
described, not section 8 as this ticket's definition of done guessed. The
command line's default is in 6.7. `just ci` passes.
