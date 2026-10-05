# The editor's sandbox: spike findings

Status: decided, 2026-10-05 (D40). This page is the comparison for
TKT-01M3YMSVD4Y1ME5QST9BY818XD (Decide the editor agent's sandbox). The editor
runs locally under Claude Code's own sandbox with `--restricted`, which is the
follow-up section. It runs under `srt` when the person configures an API key
for it.

## What the boundary must hold

The editor (D17, D39) reads, edits, builds, and tests somebody else's change.
It must not:

- read anything outside its worktree that could hold a credential, such as
  the age store, `~/.ssh`, the glab config, or the Keychain
- see a forge token in its environment
- write outside its worktree and its caches
- reach a forge, or anywhere but the model endpoint and package registries

It still has to build and test the change, and commit in its worktree.

## Candidates

1. **Claude Code's sandboxed Bash** (`sandbox` in settings). It covers shell
   commands only. Read, Edit, MCP servers and hooks run outside it, under
   permission rules instead, and reads default to most of the machine. The
   boundary would be the tool's own permission logic, not the operating
   system's. It is not enough for an unattended editor, and Claude Code's
   own documentation says so.
2. **The sandbox runtime** (`@anthropic-ai/sandbox-runtime`, `srt`, 0.0.78).
   It wraps the whole process with Seatbelt on macOS and bubblewrap on
   Linux, with a domain-allowlisting proxy, and it uses the host's
   toolchains. It is a beta from npm.
3. **A container** (podman 6 here, libkrun machine) with only the worktree
   mounted.
4. **macOS native** (`sandbox-exec` and a profile vuoro writes). This is the
   mechanism `srt` already drives on macOS. Writing it ourselves means
   rebuilding `srt`'s profile and proxy, and gives nothing for Linux.

## What was run

`probe.sh` ran each check as a shell command inside the boundary and printed
PASS when the boundary held. The canaries were fake: a file in the home
directory and a token in the environment. No real credential was read. The
spike repository was a Go module with a failing test.

| Probe | srt, inherited env | srt, env built by vuoro | podman, `--network=none` | podman, default network |
|---|---|---|---|---|
| read a home-directory file | PASS | PASS | PASS | PASS |
| list home, `~/.ssh`, glab config, age store | PASS | PASS | PASS | PASS |
| see a token in the environment | **LEAK** | PASS | PASS | PASS |
| write outside the worktree | PASS | PASS | PASS | PASS |
| reach gitlab.com | PASS | PASS | PASS | **LEAK** |
| reach gitlab.com around the proxy | PASS | PASS | PASS | **LEAK** |
| query the Keychain | PASS | PASS | PASS | PASS |
| run `go test` | needs caches | ran | ran | ran |
| reach the Go module proxy | ran | ran | **blocked** | ran |
| commit in the worktree | needs git env | ran | ran | ran |

The `srt` config denied reads of `~/`, re-allowed the worktree, allowed
writes to the worktree and a per-run cache, and allowed
`api.anthropic.com`, `proxy.golang.org` and `sum.golang.org`.

## What it showed

- **`srt` holds the boundary, and the work still runs**, provided vuoro
  builds the environment. That means an allowlist, with Go and git caches
  in a per-run directory, and `GIT_CONFIG_GLOBAL=/dev/null`. `srt` passes
  the environment through, so the environment is vuoro's job.
- **A container isolates files completely, but not the network.** Podman
  offers all or nothing. A domain allowlist needs an egress proxy that vuoro
  runs and the container is forced through. The image must also carry the
  change's toolchain, and Go here would be Node or Terraform elsewhere. That
  is a per-project image, which is friction this feature exists to remove.
- **Inside Seatbelt, Claude cannot reach the macOS Keychain**, so it reports
  "Not logged in". Any boundary that isolates the process, `srt` or a
  container, needs a model credential handed in. `claude setup-token` makes
  a long-lived token for `CLAUDE_CODE_OAUTH_TOKEN`. It is a credential a
  script reads, so it belongs in the age store, for example as
  `claude-oauth-token`, and vuoro reads it the way it reads a forge token.
  It is the only credential the editor holds, and the model endpoint is the
  only place it can send it.
- **The editor needs its own Claude config directory.** The person's
  `~/.claude` holds every other session's transcripts, so it is not
  re-allowed. `CLAUDE_CONFIG_DIR` points at a per-run directory instead.

## Follow-up: Claude Code on the local machine, as vuoro already runs it

The owner asked whether the editor can run on the local machine like a normal
session. It can. The test launched Claude Code the way vuoro launches
reviewers: no user settings, no MCP servers, `dontAsk`, and `--restricted`.
It added Edit, Write and Bash, and its own sandbox for Bash, with
`allowUnsandboxedCommands: false`, `failIfUnavailable: true`, reads of `~/`
denied, and only the Go module proxy on the network. It used the person's
normal login, so no token was needed. Each probe below is Claude using its
tools against fake canaries.

| Probe | Outcome |
|---|---|
| Read a home-directory file with the Read tool | refused |
| Read it from Bash | `Operation not permitted` |
| Grep (content mode) or Glob outside the worktree | refused: "--restricted confines the file tools to the working directory" |
| Read through a symlink in the worktree that points at a home-directory file | refused: "resolves through a symlink … outside" |
| `cat` the same symlink from Bash | `Operation not permitted` |
| Write to the home directory from Bash | `Operation not permitted` |
| Reach gitlab.com from Bash | proxy refused, 403 |
| Fix the bug, `go test`, commit | all ran |
| A token in the inherited environment | **seen**. vuoro must build the environment, as with `srt` |

One trap found on the way: **without `--restricted`, Grep and Glob ignore
the sandbox's `denyRead` and search anywhere**, even when Read is scoped to
the worktree by permission rules. vuoro's reviewers already pass
`--restricted`. A test there checked that Read, Grep and Glob are confined,
and they are.

The trade against `srt`: here the file tools are confined by Claude Code
itself, which resolves symlinks before checking. Shell commands are
confined by the operating system. Under `srt`, the operating system confines
both, but it needs an npm beta, a separate token, and its own config
directory. The local mode needs nothing new installed on macOS. On Linux,
Claude Code's sandbox needs `bubblewrap` and `socat`.

## Recommendation

**Superseded by the follow-up above.** The local mode above is now the
recommendation, and `srt` becomes the stronger option. This section
describes `srt` as it would be used.

Use **`srt`** for the editor, with the boundary written by vuoro for each run:

- **Reads:** deny `~/`. Re-allow only the worktree, the editor's config
  directory, the Claude Code install, and the per-run caches. Toolchains
  under `/opt`, `/usr` and `/nix` stay readable.
- **Writes:** only the worktree, the per-run caches, and the editor's
  config directory.
- **Network:** the model endpoint, plus the package registries a preset
  names (Go's proxy, npm, PyPI, the Terraform registry). Forges are
  explicitly denied.
- **Environment:** built from an allowlist. It holds PATH, HOME, the cache
  directories, git's identity and `GIT_CONFIG_GLOBAL=/dev/null`, and
  `CLAUDE_CODE_OAUTH_TOKEN`. It holds no forge token and no inherited
  variables.
- **Version:** pinned like terva and tuohi, and bumped in its own commit.
  vuoro refuses to start an editor when `srt` is missing or a run's config
  does not validate, and never falls back to running unsandboxed.

A container stays available later as a stronger mode for code the person
does not trust at all. It would need vuoro's egress proxy and a toolchain
image, and the Claude Code documentation points at a VM for that case.

## Still to prove

The ticket's last criterion is a recorded run of the editor itself, Claude
inside `srt`, trying to read a canary and reach a forge. That needs the token
above, which only the owner can create (`claude setup-token`, then
`ritual-secret set claude-oauth-token` in a terminal).
