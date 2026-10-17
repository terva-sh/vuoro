---
schema: 3
id: TKT-01M55VRM16QASJ8CER793CR9NF
title: Find the agent tool login when vuoro starts from the Dock
type: bug
status: done
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
updated_at: 2026-10-17T21:59:08Z
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

- [x] When the login genuinely is absent, the setup hint still says so and names the cause, rather than assuming /login is the fix
- [x] No credential value reaches a log line, an error message, the setup JSON, or the config file
- [x] A test covers the Dock-launch environment, with no real credential in the fixture
- [x] When the probe finds no login but the person's login shell has one, the setup hint names that as the cause and points at the tool's own settings.json
- [x] The shell check runs only on the signed-out path, and a shell that hangs, fails, or is absent leaves the hint exactly as it is today

## Definition of done

- [x] docs/plan.md records how vuoro finds an agent tool's login
- [x] The chosen approach is a numbered decision in docs/decisions.md
- [x] just ci passes

## Implementation plan

The owner chose explain-with-evidence on 2026-10-17, over recovering the
credential or naming it in config. vuoro never reads, holds, or passes a model
credential. It establishes one fact, that the person's login shell has one and
this process does not, and says what to do about it.

The reason is in the note above: a key in the tool's own `settings.json`,
either an `env` block or an `apiKeyHelper`, authenticates from any launch, and
vuoro already passes that through untouched. The person has a real fix
available. vuoro's job is to stop hiding it.

### The check

`Probe` keeps reading `claude auth status` exactly as it does. Nothing about
how a credential is obtained changes.

When a probe comes back signed out, and only then, the driver asks the person's
login shell whether any of its login variables is set. It never asks what they
hold:

    $SHELL -lic 'echo "vuoro-shell-login:${ANTHROPIC_API_KEY:+1}${ANTHROPIC_AUTH_TOKEN:+1}${CLAUDE_CODE_OAUTH_TOKEN:+1}"'

The marker is matched in the output rather than the output being read whole,
because an interactive shell writes prompts and escape sequences to stdout. A
`1` after the colon means set. No value crosses the boundary, so there is
nothing to leak into a log, an error, or the setup JSON.

`loginVars` in `agent/claude/claude.go:346` already names these three
variables for a different reason, so the list has one home.

### Why -lic rather than -lc

`-l` alone sources the login profile, which on zsh is `.zshenv` and
`.zprofile`, and misses `.zshrc` where most people put an export. `-i` reaches
`.zshrc` and costs the risk of a profile that blocks. It runs bounded by a
context timeout, only on the signed-out path, and never on Windows, which has
no `$SHELL` to ask. A shell that hangs or fails leaves the hint exactly as it
is today, since absence of evidence is not evidence here.

### What it changes

`agent.Probe` gains a field saying the login shell has a credential this
process cannot see. `toolState` keeps returning `signed_out`, because the state
machine is unchanged and the tool really is unusable right now. The field rides
beside it in `toolView`, and `web/src/setup.tsx:312` picks the hint from it.

The two hints:

- Shell has one. Claude Code is signed in in your terminal, but vuoro started
  from the Dock and cannot see your shell's environment. Move the key into
  `~/.claude/settings.json`, as an `env` block or an `apiKeyHelper`, and it
  works from any launch.
- Shell has none. Today's wording, run `claude` and sign in with `/login`,
  which is then correct.

### Also in scope

`signedIn()` at `web/src/setup.tsx:297` words `oauth_token` as "signed in with
a Claude token", which misnames a custom endpoint reached with
`ANTHROPIC_AUTH_TOKEN`. Fixed here while the wording is open.

## Notes

**agent:terva/mieli** at 2026-10-17T21:46:13Z

Measured against claude 2.1.288 before planning. Two findings change the
options the ticket was filed with.

### Claude Code already has a launch-independent place for a key

With `ANTHROPIC_API_KEY` and `ANTHROPIC_AUTH_TOKEN` both unset in the
environment, a key in the tool's own `settings.json` still authenticates:

    { "env": { "ANTHROPIC_API_KEY": "..." } }
    -> loggedIn true, authMethod api_key, apiKeySource ANTHROPIC_API_KEY

    { "apiKeyHelper": "/bin/echo ..." }
    -> loggedIn true, authMethod api_key_helper, apiKeySource apiKeyHelper

Both were run with `CLAUDE_CONFIG_DIR` pointed at a temporary directory and a
dummy key, so neither touched the real configuration.

This matters because vuoro does not interfere with either. `Driver.ConfigDir`
is only a scratch directory for the MCP config and sandbox settings that the
driver writes and passes by path with `--mcp-config`. vuoro never sets
`CLAUDE_CONFIG_DIR`, and `editorEnv` passes the person's through when they set
it themselves. So a key held this way reaches a vuoro-driven session from any
launch, with no change to vuoro at all.

A person hitting this bug therefore has a fix available to them today that
costs vuoro nothing and that vuoro never tells them about. That makes the third
option on this ticket, detect and explain, much stronger than it looked when I
filed it.

### Start at login is affected, not just the Dock

`desktop.go:160` registers a login item through tuohi on darwin, windows, and
linux, and `main.go` documents `-background` as "start in the tray with the
window closed, as start at login does". A login item gets launchd's
environment on macOS, the same as a Dock launch.

So this is not an edge case reached by people who avoid terminals. It is the
mode D55 steers everyone toward. Anyone who turns on start at login and keeps
vuoro in the tray is in it, including people for whom reviews worked when they
first tried vuoro from a shell.

### What this does to the acceptance criteria

Criteria 1 and 2 are written as though vuoro recovers the credential, which
only the first two options do. If the owner picks detect-and-explain they want
rewording to something like: a person whose login is invisible to a Dock launch
is told why, and told where to put it so that it is not. Criteria 3, 4, and 5
hold whichever option wins.

**agent:terva/mieli** at 2026-10-17T21:59:04Z

Task worklog for this ticket, from the session task board.

### Generation 1 — 2026-10-17 — Superseded AC wording, before the owner chose explain-with-evidence

- [ ] task-1 A person whose Claude Code login comes from a shell export sees that tool as signed in when vuoro was started from the Dock
- [ ] task-2 A review started from that state authenticates, so the probe and the session agree on the credential
- [ ] task-3 When the login genuinely is absent, the setup hint still says so and names the cause, rather than assuming /login is the fix
- [ ] task-4 No credential value reaches a log line, an error message, the setup JSON, or the config file
- [ ] task-5 A test covers the Dock-launch environment, with no real credential in the fixture

### Tasks

- [x] task-6 When the login genuinely is absent, the setup hint still says so and names the cause, rather than assuming /login is the fix — The non-shell_login branch of toolHelp keeps today's wording, so a genuinely absent login still reads 'is installed but not signed in: run claude in a terminal and sign in with /login'. The pre-existing setup.test.tsx case 'says what to do about each tool, and checks again' still asserts 'sign in with /login' and passes unchanged, which is what proves the common path did not regress.
- [x] task-7 No credential value reaches a log line, an error message, the setup JSON, or the config file — shellLogin returns a bool and never reads a value: the script it runs expands ${VAR:+1}, so only the variable names reach argv. TestShellLoginNamesTheVariableAndNeverItsValue sets ANTHROPIC_API_KEY to the stand-in 'stand-in-value-9fbe', records the argv the shell received, and asserts it contains 'ANTHROPIC_API_KEY' but not the stand-in. The field is a bool all the way out: agent.Probe.ShellLogin, app probe.ShellLogin as json shell_login, Tool.shell_login in the UI. No fixture holds a real crede…
- [x] task-8 A test covers the Dock-launch environment, with no real credential in the fixture — TestProbeAsksTheShellOnlyWhenSignedOut reproduces the Dock launch end to end: a fake login shell on $SHELL holds a key, the fake claude reports signed out via VUORO_FAKE_CLAUDE_SIGNED_OUT (loggedIn false, authMethod none, exit 1), and Probe comes back with Account empty and ShellLogin true. The fakeShell helper evaluates the script directly rather than sourcing a profile, so the test is hermetic. Every value in the fixtures is a stand-in such as 'stand-in-value'; no real credential appears. go t…
- [x] task-9 When the probe finds no login but the person's login shell has one, the setup hint names that as the cause and points at the tool's own settings.json — web/src/setup.tsx toolHelp takes the shell_login branch for signed_out: it names the launch environment and points at ~/.claude/settings.json (env block or apiKeyHelper) instead of /login. Covered by the setup.test.tsx case 'names the launch environment when the shell has a login that vuoro cannot see', asserting the text contains 'your shell has one' and '~/.claude/settings.json' and does not contain '/login'. 156 frontend tests pass.
- [x] task-10 The shell check runs only on the signed-out path, and a shell that hangs, fails, or is absent leaves the hint exactly as it is today — Only on the signed-out path: Probe calls shellLogin inside `if p.Account == ""`, and TestProbeAsksTheShellOnlyWhenSignedOut asserts the fake shell's argv log does not exist at all after a signed-in probe (errors.Is(err, os.ErrNotExist)). Failures stay silent: TestShellLoginSaysNothingItCannotShow is a table over no SHELL at all, SHELL naming nothing, a shell that blocks past a 100ms deadline, one that answers nothing, and one with nothing set, all expecting false; plus a prompt preceding the ans…

## Summary

Landed as explain-with-evidence (D60). vuoro still never reads, holds, or
passes a model credential.

When a probe comes back signed out, and only then, the Claude Code driver asks
the login shell whether any of `loginVars` is set, with `$SHELL -lic` and a
marker it matches in the output. It reads whether each variable is set, never
what it holds, so the only thing that crosses into vuoro is a bool:
`agent.Probe.ShellLogin`, carried out as `shell_login` on the tool view and
read by `toolHelp`.

The state machine did not move. A tool whose login vuoro cannot see is still
`signed_out`, because it is still unusable in this process. What changed is
what the person is told. Instead of being sent to `/login`, which would have
replaced a login that already worked, they are told that the launch
environment is the cause and pointed at `~/.claude/settings.json`, where an
`env` block or an `apiKeyHelper` holds a key that survives a Dock launch, a
Finder launch, and the login item.

Measured first, against 2.1.288, and the measurement is what chose the
approach. Both of those settings forms report `loggedIn true` with an entirely
empty environment, and vuoro already passes them through untouched, since it
writes its MCP configuration to a scratch directory, names it with
`--mcp-config`, and never sets `CLAUDE_CONFIG_DIR`. Recovering the credential
would have re-solved a problem the tool had already solved, at the cost of
running the person's profile at startup and holding their key in memory.

Also fixed: `signedIn()` called `oauth_token` "a Claude token", which is wrong
for a custom endpoint reached with `ANTHROPIC_AUTH_TOKEN`. It now says "a
token".

Tests are hermetic, with a fake login shell that evaluates the script rather
than sourcing a profile, and stand-in values throughout. They cover the Dock
launch end to end, that the shell is never run while the tool is signed in,
that the variable name and not its value reaches argv, and every way the shell
can fail to answer. `just ci` passes.

Two things a later reader should know. `-lic` is a deliberate compromise: `-l`
alone misses `.zshrc`, where most people write an export, and `-i` buys that
at the risk of a profile that blocks, which the timeout bounds. And this bug
reached further than the title says, because `desktop.go` registers a login
item on darwin, windows, and linux, so start-at-login was affected too, not
just the Dock.
