---
schema: 3
id: TKT-01M55VRM16QASJ8CER793CR9NF
title: Find the agent tool login when vuoro starts from the Dock
type: bug
status: ready
status_reason: null
priority: normal
due_on: null
labels:
  - agent
  - credentials
  - desktop
assignees: []
milestone: null
parent: null
origin: null
dependencies: []
blocks_on: none
references:
  - ref: code:claude-probe
    path: agent/claude/claude.go
  - ref: code:tool-state
    path: app/setup.go
  - ref: code:signed-in-wording
    path: web/src/setup.tsx
claim: null
archive: null
created_at: 2026-10-17T21:19:54Z
updated_at: 2026-10-17T21:30:21Z
created_by:
  id: agent:terva/mieli
  name: Mieli
updated_by:
  id: agent:terva/mieli
  name: Mieli
extensions: {}
---

## Description

`claude.Driver.Probe` runs `claude auth status` with the environment of the
vuoro process, and `editorEnv` keeps `ANTHROPIC_API_KEY`,
`ANTHROPIC_AUTH_TOKEN`, `ANTHROPIC_BASE_URL`, and `CLAUDE_CODE_OAUTH_TOKEN`
from that same environment. Both are right when a person starts vuoro from a
terminal. Neither is right when the tuohi window starts from the Dock, from
Finder, or from a login item, because macOS gives those launchd's environment
and not the login shell's.

So a person who authenticates Claude Code by exporting a key in `.zshrc`, which
is how an API key with a custom `ANTHROPIC_BASE_URL` is usually held, sees
"Claude Code N.N.N is installed but not signed in" on the setup screen while
`claude` works in their terminal. The setup hint then tells them to run
`/login`, which is wrong for their setup and would replace a working
configuration.

The probe is only the visible half. A review started in that state fails with
`agent.ErrUnauthenticated`, because `editorEnv` copies from the same empty
environment. Fixing the banner without fixing the session environment would
trade a clear refusal for a confusing failure part-way through a run.

### How it was found

Reported as a false negative on a machine running Claude Code 2.1.288. That
machine turned out to hold no credential at all, so the banner was correct
there. The measurements below are from it and stand on their own.

`claude auth status` with an empty environment:

    { "loggedIn": false, "authMethod": "none", "apiProvider": "firstParty" }

With `ANTHROPIC_API_KEY` set, exit 0 and:

    { "loggedIn": true, "authMethod": "api_key", "apiKeySource": "ANTHROPIC_API_KEY" }

With `ANTHROPIC_AUTH_TOKEN` and `ANTHROPIC_BASE_URL` set, exit 0 and:

    { "loggedIn": true, "authMethod": "oauth_token" }

So the probe reads API-key auth correctly. It fails only when the variables do
not reach the process. That is the whole bug.

### A second, smaller finding

A custom endpoint reached with `ANTHROPIC_AUTH_TOKEN` reports
`authMethod: "oauth_token"`, and `signedIn()` in `web/src/setup.tsx:297` words
that as "signed in with a Claude token". For someone pointing at a proxy that
names the wrong thing. Claude Code picks the vocabulary, so vuoro can only
choose how much to trust it. Worth settling while the wording is open.

## Acceptance criteria

- [ ] A person whose Claude Code login comes from a shell export sees that tool as signed in when vuoro was started from the Dock
- [ ] A review started from that state authenticates, so the probe and the session agree on the credential
- [ ] When the login genuinely is absent, the setup hint still says so and names the cause, rather than assuming /login is the fix
- [ ] No credential value reaches a log line, an error message, the setup JSON, or the config file
- [ ] A test covers the Dock-launch environment, with no real credential in the fixture

## Definition of done

- [ ] docs/plan.md records how vuoro finds an agent tool's login
- [ ] The chosen approach is a numbered decision in docs/decisions.md
- [ ] just ci passes

## Implementation plan

The approach is an owner decision, because each option trades away something
different. Written here so the choice is made once rather than inside a diff.

1. **Read the login shell once at startup.** Run the person's `$SHELL -lic env`
   and merge the `ANTHROPIC_*` and `CLAUDE_*` variables it prints. This is what
   most macOS GUI apps do. It costs a subprocess at startup, it runs the
   person's shell profile, and the values then live in vuoro's memory, which
   the tokens rule in AGENTS.md bears on even though these are model
   credentials and not forge tokens.
2. **Let the person name the variables in config.** Explicit and cheap to test.
   It asks a person who has already configured Claude Code to configure it a
   second time, and it must never store the value itself, only where to read
   it.
3. **Detect and explain, fix nothing.** When the probe finds no login and vuoro
   did not start from a terminal, say so and name the cause. Honest and small.
   It leaves the person to launch vuoro from a shell.

Whatever is chosen, the probe and `editorEnv` must read the same source, or the
banner and the run disagree.
