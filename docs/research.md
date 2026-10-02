# Research

What the bootstrap sessions read, on 2026-10-01 and 2026-10-02, and what each
source offers vuoro. It is a snapshot: versions move, so check the source
before you rely on a detail here. docs/decisions.md records what was decided
from it, and docs/plan.md is the design.

Paths are relative to the owner's workstation checkouts:

- `~/Workspace/github.com/terva-sh/`: terva, tuohi, git-ticket, and
  git-ticket-canvas. They were cloned from the GitHub mirrors, with a `forgejo`
  remote added to each.
- `~/Workspace/git.local.sothr.com/terva-sh/`: terva-action-code-review,
  design, and ketju, cloned from Forgejo.

## Where the source of truth is

Every terva-sh repository's `origin` is the internal Forgejo at
`git.local.sothr.com/terva-sh`, which reads anonymously over HTTPS. GitHub is a
mirror, and on 2026-10-02 it trailed Forgejo by days: tuohi by 22 commits,
git-ticket by 224, and git-ticket-canvas by 171. Read code and versions from
Forgejo.

| Project | Latest release on Forgejo, 2026-10-02 | Note |
|---|---|---|
| terva | v0.139.7 | default branch `sothr-main`; `terva update` installs the latest |
| tuohi | v0.1.0, tagged 2026-10-01 | first release, after v0.1.0-alpha.1 |
| design | v0.1.0 | identical on both forges |
| git-ticket | v0.24.0 | v0.23.0 was installed at bootstrap |
| git-ticket-canvas | v0.8.0 | |

`terva-sh/meta` and `terva-sh/terva-code-review` do not read anonymously.

## tuohi: the window

A Go library that opens native windows over the operating system's web engine
(WKWebView, WebKitGTK, WebView2), without cgo, by loading system libraries at
run time through purego. It is a fork of `malivvan/appkit` v0.1.0, and
`docs/provenance.md` records it.

- **The pattern to copy** is `examples/loopback/main.go`. A `net/http` server on
  `127.0.0.1:0` runs in a goroutine, `tuohi.App` and `tuohi.View{URL: ...,
  Bind: map[string]any{...}}` open the window, and main runs `app.Show` then
  `app.Wait`. Bindings run on their own goroutines. Its `-check` mode, in which
  the page reports back through a binding and the process exits 0 or 1, is a
  model for a GUI smoke test.
- **The macOS main thread.** AppKit must run on the process's main thread. The
  darwin `init` keeps the main goroutine there, and `Show` and `Wait` off it
  return `ErrNotMainThread`. See `docs/architecture.md`.
- **Trusted origins.** The window trusts the origin it is navigated to, and only
  that page can call the bindings. The loopback server itself is reachable by
  every local process, so serve nothing through it that another local user must
  not see. That is the reason for the loopback-protection ticket.
- **Desktop services** live in subpackages: `notify`, `tray`, `autostart`,
  `clipboard`, `dialog`, and `instance` (single instance). The v1.0 tray work
  uses them.
- **Rules worth inheriting**, from its AGENTS.md: build and test with
  `CGO_ENABLED=0`, cross-build every platform on every change, test macOS and
  Windows on the GitHub mirror's hosted runners, and keep attribution in
  NOTICE.
- **Handoffs** are written to `~/.local/state/agent-handoffs/<repo>/` first,
  and promoted into `handoffs/` when a ticket needs one. vuoro copies this.

## design: the styles

The shared foundation for terva-sh web apps (`README.md`, `tokens.json`).

- **The contract.** Components read `--ui-*` roles and never a literal colour:
  grounds, lines, text, status, accent, effects, type, shape, and space. Every
  text role meets WCAG AA in both schemes, and the tests allow no exceptions.
- **Schemes.** With no attribute on the root, `prefers-color-scheme` decides.
  `data-scheme="light|dark"` on `<html>` wins in both directions.
  `data-theme` is reserved for palette presets.
- **Adopting it.** Pin the module, then run `go run
  github.com/terva-sh/design/cmd/design-tokens export -app NAME -out
  web/.../design.css`, commit the output, and run the same command with
  `-check` in CI. A server with no frontend build can serve `design.CSS(app)`
  instead.
- **Accents.** v0.1.0 has terva, lampi, ketju, and git-ticket-canvas. lampi and
  ketju are both blue. **There is no vuoro accent**, so one must be added in
  that repository (`tokens.json`, then `just generate` and `just ci`) and
  released before vuoro can export its stylesheet.

## git-ticket-canvas: how a terva-sh app is laid out

It is the closest sibling application.

- **Go and Preact.** A Go binary serves a Preact and TypeScript frontend built
  with Vite. `web/dist` is committed and embedded, so `go build` needs no Node.
  CI checks that the committed bundle matches its source.
- **The justfile** has `build`, `install`, `run`, `test` (`go test -race
  -cover`), `fmt-check`, `vet`, `tickets-check`, `web-setup`, `web-build`,
  `web-test`, and browser tests through Playwright. Its header explains a
  measured gotcha: forward arguments as `*args` with `"$@"`, not `{{args}}`.
- **CI.** `.forgejo/workflows/ci.yml` runs in a Go Alpine container and installs
  git-ticket at a pinned version for the store check. The GitHub mirror has
  `ci.yml` and `release.yml`, and goreleaser handles releases.
- **The state directory.** `$XDG_STATE_HOME/<app>` or `~/.local/state/<app>` on
  Linux, `~/Library/Application Support/<app>` on macOS, and
  `%LOCALAPPDATA%\<app>` on Windows. vuoro uses the same layout.
- **Library, not CLI.** It imports git-ticket's `ticket` package, and it writes
  ticket files but never commits them. vuoro's git-ticket integration follows
  both rules.
- **Licence and identity.** MIT, "Copyright (c) 2026 Drew Short". Commits are
  authored as `warrick@sothr.com`.

## terva: one of the two agent tools

vuoro drives terva over `terva rpc` (`docs/rpc.md`) and does not import it (D7).

- **Process model.** One process serves one working directory, one model, and
  one session. One prompt runs at a time, and others queue. The process exits
  when stdin closes.
- **Wire format.** NDJSON in both directions. A command such as `prompt`,
  `abort`, `get_state`, `get_messages`, `set_model`, `get_models`, or `ping`
  carries an `id`. A `response` echoes it. Events have no `id`: `turn_start`,
  `text_delta`, `tool_call`, `tool_result`, `assistant_message`, `usage` (with
  `cost_usd`), `turn_end`, and `done`, which is the one terminal event of each
  operation.
- **Auth.** With `TERVACORE_RPC_TOKEN` set on the child, the first frame must be
  `hello` with the matching token.
- **The flags vuoro needs.** `--provider`, `--model`, `--cwd`, `--tools` or
  `--no-tools`, `--no-mcp`, `--no-ext`, `--approval plan`, and `--session PATH`
  for a resumable session. MCP servers load with `--mcp`. Headless runs are
  unconfirmed by default. With an approval mode, a call that needs confirmation
  is refused unless `--rpc-approvals` is on.
- **Versioning.** `hello` returns `protocol_version`, which is the schema's
  major version. Within a major version, events and commands are only added.
- **The Go API, not used.** `docs/embedding.md` compares RPC, the SDK
  (`packages/agent/sdk`), and the engine (`packages/core` with
  `packages/provider`). `.api/packages.txt` lists the stable packages. The
  module is `terva.sh/terva` at `go 1.27.0`. vuoro avoids that line by not
  importing it.
- **Prior art inside terva.** The `review-crew` personas, such as Vartija for
  security, are in `packages/agent/persona/personas/builtin/review-crew`. The
  `raati` deliberation panel (`docs/raati.md`) runs three seats blind, then a
  cross-examination, and its rule is "the minority report is the product".

## Claude Code: the other agent tool

Read from `claude --help` on version 2.1.285. Check it again when you build the
driver.

- **Headless and streaming.** `claude -p --input-format stream-json
  --output-format stream-json`, with `--include-partial-messages` for deltas
  and `--replay-user-messages` to echo input. This is the protocol Claude Code's
  own SDKs speak. A Go program drives it as a subprocess.
- **Sessions.** `--session-id <uuid>` names a session, and `--resume <id>` or
  `--continue` picks it back up.
- **Lockdown candidates.** `--tools Read,Grep,Glob` (`""` disables every
  tool). `--restricted` removes the tools that run commands or code, removes
  WebFetch, and ignores user, project, and local settings files, though managed
  settings and `--settings` still apply. `--strict-mcp-config` with
  `--mcp-config` loads only the given MCP servers. Also `--permission-mode`
  (`plan`, `dontAsk`, and others), `--allowedTools`, and `--disallowedTools`.
- **Structured output.** `--json-schema`. D9 chose MCP instead, but it remains
  available.
- **Prompts.** `--system-prompt`, `--append-system-prompt`, and `--model`.
- **The open question** for the driver spike: do claude.ai connectors and
  plugins stay out of a `-p` session under `--strict-mcp-config` and
  `--restricted`? This workstation has a GitLab connector with write tools, so
  the spike has to prove it.

## terva-action-code-review: prior art, not a dependency

A Node (`.mjs`) Forgejo action that runs terva as a pull-request reviewer
(`README.md`, `docs/`). It is an internal proof of concept. D6 rules out using
it, and these parts are worth reading:

- **Review profiles and prompts** in `templates/`: code, security,
  documentation, design, and custom, plus a summary template.
- **The finding shape** in `schemas/review-result.v1.schema.json` and
  `docs/review-result-contract.md`:
  - severity (critical, high, medium, low, info), title, description, evidence;
  - an optional anchor `{path, side: base|head, commitSha, startLine,
    endLine}`;
  - completion states (`completed`, `failed`, `cancelled`, `incomplete`), where
    completed does not mean approved;
  - `major.minor` versioning, under which a reader keeps unknown fields.
- **Validation.** The schema is not enough on its own: `src/review-result.mjs`
  adds cross-field rules. When a reply breaks the schema, the error names the
  rule it broke, and vuoro's drafts server does the same.
- **Base-commit config.** `.terva/review.yml` and `.terva/reviewers/*.yml` are
  read at the base commit, never the head, so a change cannot pick its own
  reviewer. D14 adopts the rule.
- **Panels.** Two to six members run in parallel, a synthesizer merges what
  they find and says what it set aside, gates are `union`, `consensus`, or
  `synthesized`, and path triggers run a specialist only on matching paths.
  This is prior art for D11.
- **Dispositions.** Accepted, declined, or deferred, with `fixed:SHA` and a
  reason, kept in one maintained pull-request comment. A later review does not
  raise a declined finding again.
- **Lockdown.** `src/terva.mjs` runs `terva rpc --portable=strict --no-tools
  --no-memory --no-ticket --no-project --no-session --approval plan
  --max-steps 1`. The child gets only `PATH`, `TERVA_HOME`, and one provider
  key.
- **Fallback models.** Up to three, tried in order when a model is withdrawn
  or its endpoint is down. A run records which one actually ran.
- **A dashboard** is proposed in its tickets, TKT-01M2VQM5TKMZ9761Z69S3KPZ56
  (a draft spike), as terva-sh/terva-code-review. It would receive results from
  the action. That is an organisation-wide record of action runs, not a
  personal queue across forges.

## Other sources

- **ketju** is a message board: a Go binary with a Preact client, and pure-Go
  SQLite for coordination. It does not overlap with vuoro. Its pure-Go SQLite
  is a precedent if the state spike needs one.
- **`~/Workspace/code-reviews/`** is the owner's existing GitLab review workflow,
  in Python, from `gitlab.com/drewshort/code-reviews-template`. Two tools are
  worth reading:
  - `tools/fetch_mr.py` fetches metadata, diffs, and discussions, and falls back
    when a diff is truncated.
  - `tools/post_inline_suggestions.py` posts positioned inline suggestion
    threads and records the discussion IDs.

  Its token lives in a `.env` file. Do not read it.
- **GitLab API, from that workspace and the plan.**
  - Discover changes with `GET /merge_requests?scope=all&reviewer_username=`
    and `author_username=`.
  - Read a change from `/merge_requests/:iid/changes`, `/discussions`,
    `/approvals`, and `/draft_notes`.
  - Publish a review with `POST .../draft_notes/bulk_publish`.
  - An inline position needs `base_sha`, `start_sha`, and `head_sha`.
- **The employer's PHI policy** is summarised in D8.
