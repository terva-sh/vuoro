---
schema: 3
id: TKT-01M55VT3H26HX25TB8864NPG6J
title: Ship named built-in presets, not one hardcoded default
type: task
status: ready
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
updated_at: 2026-10-17T21:30:42Z
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

- [ ] vuoro ships more than one preset, and a fresh install with no config.json offers all of them in the Review tab picker
- [ ] The rule for how a person's presets meet the built-in ones is stated in docs/plan.md and holds in a test
- [ ] vuoro review with no -preset runs the same preset the window would, on a config that names none
- [ ] An existing config that names its own presets keeps running exactly as it does now

## Definition of done

- [ ] docs/plan.md section 8 describes the shipped presets and the composition rule
- [ ] A numbered decision in docs/decisions.md records the set and the rule
- [ ] just ci passes

## Implementation plan

The owner settled the three open questions on 2026-10-17. They are decisions,
not suggestions, and docs/decisions.md gets the next number for them.

### What ships

- **quick.** One code reviewer. This is what `presetsOf` returns today, given a
  name, so the current default keeps working and `vuoro review` finds it.
- **standard.** Code and security, with a synthesizer.
- **thorough.** Code, security, and documentation, with a synthesizer and a
  verifier. This is the only shipped preset that exercises verification (D31),
  which is built and which nobody currently meets without writing config.

### How a person's presets meet them

They supplement, the way profiles already do. A preset the person names
themselves is added to the shipped set, and a preset with the same name as a
shipped one shadows it rather than appearing twice. So `presetsOf` stops
replacing and starts merging, which makes it agree with the profile lookup in
`review/run.go:1046` instead of contradicting it.

### Editing

A shipped preset is copied before it is edited. The editor offers copy and
never edit-in-place, so the shipped set keeps its meaning across an upgrade and
a person who improves `thorough` next release still gets it.

### A wrinkle this creates

Today the window's default is `presets[0]` and `vuoro review`'s default is
`quick` when the config defines it, otherwise the first preset
(`review_cmd.go:196`). Once `quick` always exists, those two rules stop
agreeing: the CLI would always run `quick` while the window runs whatever sorts
first, which for a person with their own presets may not be `quick`. Decide one
rule and state it, rather than leaving the window and the command line to
default differently.

Ordering also decides what the picker preselects, so settle whether the
person's own presets sort before or after the shipped ones.

### Where the code goes

`config.Preset` is the type, so a `Builtin` list in `config` is the closer
parallel to `review.Builtin`, and `app.presetsOf` becomes the merge. Check
that `config` taking on shipped content does not cross the import boundaries
AGENTS.md names.
