---
schema: 3
id: TKT-01M562ESB0WGCRWJJW0QYTBZE6
title: A preset with no tool pinned breaks when a second agent tool exists
type: bug
status: draft
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
updated_at: 2026-10-17T23:16:52Z
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

- [ ] With two agent tools registered, a preset that pins no tool starts a run on a defined tool rather than failing
- [ ] The rule that picks the tool is written down, and does not depend on Go's map iteration order
- [ ] A test registers two drivers and covers the empty tool name, including the path that reloads a run after a restart

## Implementation plan

Decide the rule first, because the code is small and the choice is not.

The options I can see. One is a configured default tool, which gives the
person control and adds a config field. One is a fixed preference order in
code, which needs no configuration and is opaque when it surprises someone.
One is to refuse an empty tool once more than one is registered, which is
honest and breaks every existing preset the moment terva lands, so it is
probably wrong on its own.

Whatever the rule, a test must pin it with two drivers registered. The
present tests all run with one (`review/run_test.go:159`), which is why this
has stayed invisible. `TestDriversCanBeReplacedWhileRunsStart` is the only
test that touches the map directly and it does not exercise the empty name.

Check the other callers while there: `StartSummary` (`review/summary.go:41`),
`StartTriage` (`review/triage.go:76`), `StartEditor` (`review/editor.go:281`),
and the reload path in `launch` (`review/run.go:487`). The reload path is the
nastiest, because a run recorded with an empty tool would fail to come back
after a restart rather than failing at the start.
