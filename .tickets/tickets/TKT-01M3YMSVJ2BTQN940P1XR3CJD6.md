---
schema: 3
id: TKT-01M3YMSVJ2BTQN940P1XR3CJD6
title: Set up forges and check agent tools in the window
type: task
status: in-progress
status_reason: null
priority: normal
due_on: null
labels:
  - ui
  - credentials
assignees: []
milestone: v1.0.0
parent: TKT-01M3YMSTYMFRDFXD4QSVWHMM8P
origin: null
dependencies:
  - TKT-01M3W4HWM6V2F20KNT03BSBNZY
blocks_on: none
references: []
claim:
  actor: agent:claude/forgejo-parity-20261006
  branch: tickets/ui-setup
  worktree: /home/sothr/.t3/worktrees/vuoro/t3code-8cba4816
  commit: b2fb4ceea860ea096b9055a2a0c72d6185ea495c
  session: null
  claimed_at: 2026-10-06T14:42:05Z
  expires_at: null
archive: null
created_at: 2026-10-02T15:47:52Z
updated_at: 2026-10-06T15:15:21Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
extensions: {}
---

## Description

For somebody who is not the owner: add a forge and its token source, detect
terva and Claude Code and their versions, and build a first reviewer preset,
all from the window. Hand-edited config keeps working.

## Acceptance criteria

- [ ] A fresh install reaches a first review without editing a file
- [x] The UI never displays a stored token
- [x] Hand-edited config keeps working, and a save never overwrites a file changed or broken since it was read
- [ ] A forge's token source can be tested from the window, and the test names the account it signs in as

## Implementation plan

The owner decided on 2026-10-06 (to be recorded as D45). The window helps the person choose a **token source**, never takes a token itself, and D29 stands. Rejected: pasting a token for vuoro to store in the OS keychain. That needs a store on every platform, WSL usually has no Secret Service, and it would reopen D29. Scope of this slice: forges and the agent-tool check. Preset editing is a follow-up ticket.

### Config: write it safely (`config/`)
- **Save.** `config.Save(path, c, base)` writes the file, mode 0600, by temp file and rename. It checks the result with `Parse` before writing.
  - It refuses if the file on disk is no longer the one the window read. `base` is the sha256 of the bytes loaded. A hand edit made meanwhile is never overwritten, and the person is told to reopen setup.
- **Broken files.** A file that does not parse is never overwritten. Setup shows the error and the file's path, and nothing else.
- **What a save keeps.** A save keeps every field the form does not show (`bots`, presets, profiles, inbox), because it edits the loaded `Config`.
  - Hand formatting is lost. The file is re-encoded with two-space indentation.

### Token sources (`app/setupsources.go`)
- **Templates.** These are offered only where their program is on PATH (`exec.LookPath`). Each gives the read command that goes into `token_command`, and the store command for the person to run in a terminal. vuoro never runs the store command.
  - macOS Keychain. Read: `security find-generic-password -s vuoro -a HOST -w`. Store: `security add-generic-password -s vuoro -a HOST -w`, which prompts.
  - Secret Service. Read: `secret-tool lookup service vuoro host HOST`. Store: `secret-tool store --label="vuoro HOST" service vuoro host HOST`, which prompts.
  - pass. Read: `pass show vuoro/HOST`. Store: `pass insert vuoro/HOST`.
  - 1Password. Read: `op read op://VAULT/ITEM/credential`, with the reference filled in by the person.
  - Environment variable. `token_env`, suggesting `VUORO_<NAME>_TOKEN`. The form shows only whether it is set in vuoro's environment, never its value.
  - Custom command. The person types it, and it is split like a shell would split it, honouring quotes, with no expansion. The resulting argv is shown before saving.
- **Creating a token.** A link to the forge's token page:
  - GitLab: `URL/-/user_settings/personal_access_tokens?name=vuoro&scopes=api`.
  - Forgejo: `URL/user/settings/applications`, with the scopes named.
  - The scopes are checked against what the adapters call, during implementation.

### Testing a source (`POST /api/setup/test`)
- **What it does.** It resolves the token from the source in the request, through the same `config.Forge.Token`, then asks the forge whose token it is.
  - This uses a new `forge.Account` interface, `Me(ctx) (string, error)`. GitLab and Forgejo already read `/user`.
- **The reply.** It answers with the username, and whether it matches the username in the form. The form fills the username in when it is empty.
  - The token never reaches the reply, a log, or an error. Errors keep config's scrubbing: program name and exit status only.
- **When it runs.** Only on the person's click. Like every route, it sits behind the loopback guard (TKT-01M3W4HWDY8XV92MXXQWA0AES7).
  - Saving a `token_command` is a program vuoro will run, so the form shows the exact argv and the save confirms it.

### Saving and reloading (`PUT /api/setup/forges`, `DELETE /api/setup/forges/{name}`)
- **Save.** Add or edit one forge, with `base` from `GET /api/setup`, which gains the config path, its `base` hash, and each forge's token source. The source is the command or variable name, never a value.
- **Delete.** Removes one forge.
- **Live reload.** After a save, the app swaps its config. `config()` reads under a lock, and the cached clients and the inbox service are dropped so they rebuild. Runs already open keep their clients until reopened.

### Agent tools
- **Check again.** `POST /api/setup/tools/check` probes again, with probeOnce replaced by a lock and a refresh.
- **Hints.** Each tool shows a hint for its state: not found (where to install it), too old (the version needed), or not signed in (run `claude`, then `/login`).

### UI
- **First run.** With no forges and no config error, the home screen shows a setup card in place of the bare sentence.
  - Step 1 adds one or more forges, each with Test and Save.
  - Step 2 is the tool check.
  - "Skip, and open public changes anonymously" stays possible.
- **The accounts panel.** It lists the forges with Edit and Remove, plus "Add a forge", and shows the tools with Check again.
- **Config errors.** A config error shows the message and the path, and offers no editing.

### Checks
- **Config tests.**
  - A save round-trips.
  - A changed file is refused, and so is a broken one.
  - The file is written 0600, and fields the form does not show are kept.
- **App tests.**
  - The routes are guarded.
  - A test reports a username match and a mismatch.
  - A sentinel token is never in any reply or log.
  - A save reloads the inbox's accounts.
  - The shell-style split handles quotes.
- **Web tests.** The first-run card, the form, the argv preview, and that no token field exists.
- **Live.** Add the sandbox Forgejo through the window on this machine, with a token from Secret Service, and open the inbox.
- **Docs.**
  - D45 in docs/decisions.md.
  - plan.md section 5.1 loses "the person writes it by hand".
  - The README points to setup.

## Notes

**agent:claude/forgejo-parity-20261006** at 2026-10-06T14:52:50Z

Implemented on branch tickets/ui-setup.

### Built
- **config.** `config.Read` and `config.Save` with base-hash checking, plus `Forge.Check`.
- **forge.** `forge.Account` (`Me`) on both adapters.
- **app.** `app/setup.go` with the setup view, the token-source templates, and the test, save, remove and tools routes.
  - Config is read under a lock, and the inbox and clients are rebuilt after a save.
  - Opening setup re-reads a hand-edited file.
  - Tools can be probed again.
- **web.** `web/src/setup.tsx`: the first-run card, ForgeEditor, Tools, and Forges in the accounts panel.
- **Docs.** D45, plan 5.1, and the README.

### Changes from the plan
- **Opening setup reloads a changed file.** Otherwise a save refused after a hand edit could never succeed: the app only read the file at launch, so "open setup again" would show the same stale hash. A file that no longer loads keeps the running config, shows its error, and blocks saving.
- **Save waits for a passing test.** It is enabled only after a test whose account matches the username. Editing a forge's clone setting therefore needs a retest. That is accepted, since it re-proves the source.

### Verified
- **Go.** `just ci` passes.
  - The config tests cover round-trip, mode 0600, a refused hand edit, a removed file, a broken file, and an invalid forge.
  - The app tests cover add, test, edit and remove with a sentinel token that never appears in a reply or in config.json. They also cover the inbox picking up the account, the broken-file refusal, the token sources per OS, and the tool states.
- **Web.** 77 tests, including the command splitter and the form's test-then-save payload, which carries no token.
- **A real binary.** It ran in browser mode against an empty config dir, with a headless screenshot of first run. Claude Code was detected and signed in.
- **The real test route.**
  - Against git.local.sothr.com with an unset variable: a clear error.
  - With a dummy token: 401 from Forgejo, and from gitlab.com too. The token was not in either reply.
  - A failing command: program and status only.
  - A request without the launch token: 403, and the command never ran.

### Not verified
- A successful live test and save with a real token. That needs a token this session does not hold.
- Reaching a first review from a fresh install. That is AC 1, left unticked until the owner runs it, for instance in WSL.

**agent:claude/forgejo-parity-20261006** at 2026-10-06T14:59:21Z

terva-review run bbc9a5fe: three findings, all fixed. (high) Two saves from one base could both pass the check, because the check and the write were separate steps. config.Save now holds a lock across read, check and write, and the app holds saveMu across each whole save and reload. TestConcurrentSavesFromOneBaseLetOnlyOneThrough fails 12 runs in 20 without the lock and passes 20 in 20 with it. An external edit inside the write itself can't be seen without file locking, so D45 and the comments now say exactly that instead of 'never'. (medium) main.go kept ConfigPath only when the file loaded, so a file broken at launch could not be fixed without a restart. The path is now kept whatever the read result, with a main-package test. (medium) setupView read the config and its base hash under separate locks. It now takes one snapshot, and its presets come from that snapshot too.

**agent:claude/forgejo-parity-20261006** at 2026-10-06T15:05:33Z

terva-review run b516d51a: two findings, both fixed. (high) The terminal store commands put the host into shell text unquoted, so an address like https://$(id).example would run a command when pasted. Store is now argv, shown through joinCommand's shell quoting. hostOf also accepts only a plain host name and port, with no credentials, path, query or fragment, matching what config already enforces. (medium) Reopening the accounts panel never fetched setup again, so the server-side reload of a hand edit or repaired file never ran. The panel now reads setup on every open. Tests cover hostOf refusing $(id), backticks, userinfo and paths, the quoted store command, and the panel's refresh.

**agent:claude/forgejo-parity-20261006** at 2026-10-06T15:09:10Z

terva-review run a9dd2ea9 (high): an editor left open while the panel refreshed kept its old fields but saved with the new base, so it could pass the check and overwrite a hand edit. Each ForgeEditor now captures setup.base when it opens and saves against that, so a stale editor is refused with ErrChanged. Rejected: remounting editors on every new base, which would throw away what the person was typing whenever any save or refresh happened. A test re-renders an open editor with a new snapshot and checks that the save carries the old base.

**agent:claude/forgejo-parity-20261006** at 2026-10-06T15:15:21Z

terva-review run 511ede38 (medium): a failed Check again was swallowed and left stale results on screen. Tools now catches it and says the check did not run, with a test for a rejected request. The same review asked whether replacing drivers live is synchronized with the run manager. It wasn't: Manager.driver read m.Drivers unlocked while a reprobe now replaces the map. Manager.SetDrivers now guards it with an RWMutex, which driver() reads under. TestDriversCanBeReplacedWhileRunsStart exercises this, and -race catches a regression on CI's darwin leg.
