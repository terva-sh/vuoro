---
schema: 3
id: TKT-01M562ESB0WGCRWJJW0QYTBZE6
title: A preset with no tool pinned breaks when a second agent tool exists
type: bug
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - agent
  - review
assignees: []
milestone: null
parent: TKT-01M3YMSTYMFRDFXD4QSVWHMM8P
origin: null
dependencies: []
blocks_on: none
references:
  - ref: plan:design-of-record
    path: docs/plan.md
claim: null
archive: null
created_at: 2026-10-17T23:16:52Z
updated_at: 2026-10-18T00:40:48Z
created_by:
  id: agent:terva/mieli
  name: Mieli
updated_by:
  id: agent:terva/mieli
  name: Mieli
extensions: {}
---

## Description

`Manager.driver` resolves an empty tool name to "the only driver", but only
when exactly one driver is registered (`review/run.go:1062`):

```go
if name == "" && len(m.Drivers) == 1 {
	for n, d := range m.Drivers { return n, d, nil }
}
d, ok := m.Drivers[name]
if !ok {
	return "", nil, fmt.Errorf("review: no agent tool named %q is available", name)
}
```

With two drivers registered and an empty name, control falls through to
`m.Drivers[""]`, which is never a key, and the run fails with
`review: no agent tool named "" is available`.

An empty tool is the common case, not an edge. It is what every shipped
preset uses, what the presets page calls "Any tool available", and what
`config.Preset` means by leaving `tool` out. `Plan` passes it through
untouched (`app/app.go:392`), and `StartSummary`, `StartTriage`, and
`StartEditor` all reach `driver` the same way.

Nothing is broken today. `main.go:194` registers claude alone, so
`len(m.Drivers)` is 1 whenever claude probes clean, and the fallback fires.
The bug arrives with the second driver, which docs/plan.md section 6 says is
terva. Whoever adds it will see every default preset stop working at once,
and the message will point at the preset rather than at this function.

Found while reading the resolution path for TKT-01M561J4A1AB61CS7FP6J6TJDD
(Warn on the presets page when a profile or tool no longer resolves). The
owner chose to file it rather than fix it there, because it is a different
fix in a different package.

Worth deciding rather than assuming: with two tools installed and no tool
pinned, which one should run? Picking the map's first key is not an answer,
since Go randomizes that order. A stable rule might be the person's
configured preference, or the first that probes clean in a fixed order.

## Acceptance criteria

- [x] With two agent tools registered, a preset that pins no tool starts a run on a defined tool rather than failing
- [x] The rule that picks the tool is written down, and does not depend on Go's map iteration order
- [x] A test registers two drivers and covers the empty tool name, including the path that reloads a run after a restart

## Implementation plan

### The rule, decided 2026-10-18

The owner delegated this one. It goes in docs/decisions.md as D63.

An empty tool means the person does not care which tool runs, so vuoro picks,
and the pick has to be stable. vuoro picks a tool that can run, preferring
one that is signed in over one that is not, and breaks a tie by name.

No hand-ranked list of tools. vuoro has no basis for an opinion about whether
claude beats terva, and writing one into the code would be a preference
nobody chose, hidden where nobody looks. It has a basis for two opinions: the
choice must be stable, and a signed-out tool loses, because a run on it
reaches authentication and dies there.

Rejected: a `default_tool` key in config.json. A person who cares which tool
runs already has a place to say so, the preset's own `tool` field, which the
presets page now makes a select (D62). A second place to express the same
preference is one too many, and it would need validation, a view field, and
a settings control to be worth having.

### Who decides, and who obeys

`review` owns `Drivers` and knows nothing about probes. `app` owns the probes
and so is the only one that knows which tools are signed in. So `app` decides
and `review` obeys, which also matches `review` staying mechanical.

`SetDrivers` becomes `SetDrivers(d map[string]agent.Driver, pick string)`,
one call so the two cannot drift. `Manager.pick` is read under the same
`driversMu` as `Drivers`.

`driver("")` then resolves in this order:

1. `m.pick`, when it is set and is a key of `Drivers`.
2. The only driver, when exactly one is registered. This keeps every test
   that builds a `Manager` literal working, and it is the honest answer when
   there is no choice to make.
3. An error. Not the present `no agent tool named "" is available`, which
   reads as though the preset named the empty string.

`app.pickTool` sorts the offered names by signed-in first, then by name, and
returns the first. `offered` already holds exactly the drivers whose probe
returned no error, which is `ok` and `signed_out` (`app/app.go:179-184`).

### The reload path, which the filing worried about

It is already safe, for a reason worth writing down rather than rediscovering.
`Start` records `Tool: name`, the name `driver` resolved, not the name the
preset held (`review/run.go:228`). So a session on disk always names a
concrete tool and the reload in `launch` never passes an empty name. A test
pins this, because the day somebody records `rv.Tool` instead, every run with
no tool pinned stops coming back after a restart.

### Tests

In `review`: two drivers with a pick set resolves to the pick; two with no
pick is an error whose message does not say `named ""`; one driver with no
pick still resolves, which is today's behaviour; a pick naming a driver that
is gone falls through rather than being trusted.

In `app`: `pickTool` prefers `ok` over `signed_out`, breaks a tie by name,
and returns empty when nothing is offered.

In `review`: a run started with no tool records the resolved name, and comes
back after a reload.

## Notes

**agent:terva/mieli** at 2026-10-18T00:40:43Z

Task worklog for this ticket, from the session task board.

### Tasks

- [x] task-26 With two agent tools registered, a preset that pins no tool starts a run on a defined tool rather than failing — Manager.driver resolves an empty name to m.pick when it is a key of Drivers, then to the only driver, then errors. SetDrivers takes drivers and pick in one call so the two cannot drift, and app/app.go:187 passes pickTool(offered, probes). TestAReviewerThatNamesNoToolResolvesWithSeveralRegistered covers two drivers with the pick on each of them, and TestARunWithNoToolPinnedRecordsTheToolItResolved starts a real run with two registered.
- [x] task-27 The rule that picks the tool is written down, and does not depend on Go's map iteration order — D63 in docs/decisions.md and docs/plan.md section 5.1. pickTool in app/app.go sorts the offered names by signed-in first, then strings.Compare, so no map range remains. TestPickToolPrefersASignedInToolAndBreaksTiesByName runs each case 20 times to catch a reintroduced map range.
- [x] task-28 A test registers two drivers and covers the empty tool name, including the path that reloads a run after a restart — review/run_test.go gains three tests. TestAReviewerThatNamesNoToolResolvesWithSeveralRegistered registers claude and terva and checks the pick wins in both directions, 20 times each. TestAReviewerThatNamesNoToolFailsClearlyWithNothingToPick covers no drivers, a stale pick, two with no pick, and a pick with no drivers, and asserts no message says named "". TestARunWithNoToolPinnedRecordsTheToolItResolved starts a real run with two registered, checks the session records the resolved name, shuts do…

## Summary

Landed as D63, in `review/run.go` and `app/app.go`.

`SetDrivers` now takes the drivers and the pick in one call, so a pick naming
a driver that is no longer registered cannot outlive it. `driver("")`
resolves to the pick when it is a key of `Drivers`, then to the only driver,
then errors. `app.pickTool` chooses it: the offered names sorted by
signed-in first, then by name.

The rule and its reasoning are in D63. The short form: an empty tool means
the person did not care, so vuoro picks, and the pick being stable matters
more than which tool wins it. No hand-ranked list of tools, because vuoro has
no basis for an opinion about whether claude reviews better than terva.
Rejected a `default_tool` config key, since the preset's own `tool` field is
already that place (D62).

Three things worth knowing that the plan did not predict.

**The reload path was already safe, and now a test says so.** The filing
worried that a run recorded with an empty tool would fail to come back after
a restart. It cannot: `Start` records the name `driver` resolved, not the
name the preset held (`review/run.go:228`). The test reloads into a `Manager`
with no pick at all, which only passes while that stays true.

**The single-driver fallback stays, deliberately.** It looks redundant now
that `app` always sets a pick, and it is not. Every `Manager` built as a
literal in a test has no pick, and dropping it would have broken a dozen
tests for no gain. It is also the honest answer when there is no choice.

**The error message changed, and that was half the bug.**
`no agent tool named "" is available` blamed the preset for naming the empty
string, which is exactly the wrong place to send whoever hits this. It now
reads `no agent tool is available to run a reviewer that names none`, and a
test asserts no failure on this path says `named ""`.

One stumble worth recording. `TestARunWithNoToolPinnedRecordsTheToolItResolved`
first used `setup(t, nil)` and panicked with a nil dereference inside
`fakeSession.Send`. `setup`'s second argument is the function the fake agent
answers with, and nil is fine only for a test that never runs a session.
Most tests here do not, so `setup(t, nil)` reads as the normal call and is a
trap for anything that actually starts a run.

`just ci` passes, exit 0, every package green.
