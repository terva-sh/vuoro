# vuoro plan

This is the design of record. Read it before you change anything, and change it
in the same pull request as code that moves the design. Open questions are
marked as such and each one has a ticket. Do not settle one in code without
closing its ticket.

The owner's decisions behind this design, with the alternatives they rejected,
are numbered in [decisions.md](decisions.md). What the sibling projects offer is
in [research.md](research.md).

## 1. The problem

Review traffic arrives from several forges: merge requests on gitlab.com, and
pull requests on GitHub and on a Forgejo instance. Each forge has its own inbox,
its own notion of "requested", and its own notification noise. Two failures
follow.

- **Giving reviews stalls.** A requested review sits unseen, because nothing
  puts all the requests in one list ordered by how long they have waited.
- **Getting reviews stalls.** A reviewer replies, the ball comes back to the
  author, and nothing says so. Weeks pass before the author notices.

Both failures are the same missing fact: **whose turn it is.** A forge records
events. It does not answer "what is waiting on me, and since when".

## 2. What vuoro is

*Vuoro* is Finnish for "turn", as in "it is your turn".

vuoro is a desktop application. It gives one person one list of every review
they give and get, across every forge they use, ordered by whose turn it is and
for how long. From that list they read the change, comment, reply, resolve,
approve, and submit, without opening the forge's web interface.

It is also the front door to agentic review. vuoro runs a review with the agent
tool the person chooses, terva or Claude Code, and keeps the conversation open
beside the change. The person asks the agent questions, redirects it, and edits
what it drafts. The person decides what gets posted.

### 2.1 Positions

- **Turn is derived, and it shows its reasons.** "Your turn: Mason replied on
  two threads, 3 days ago" is the product. A bare "needs attention" badge is
  not.
- **An agent drafts. A person posts.** No agent output reaches a forge without
  an explicit action by the person in the UI. Agent output lands as pending
  draft comments that look and edit like the person's own, marked as drafted.
- **The person chooses the agent.** vuoro drives agent tools the person
  already has and contains none of its own. Section 6 explains how.
- **The forge is the record of the review.** vuoro keeps a cache, the person's
  private annotations, such as a snooze, and its own agent work: review runs,
  their conversations, and drafts not yet posted. It never holds a posted
  comment, a verdict, or a thread state that the forge does not also hold.
- **Link out for what vuoro does not do.** Pipelines, merge trains, and
  settings stay in the forge. Every item carries its web URL.

### 2.2 Out of scope

- Serving other people. vuoro is one person on one machine. Unlike
  git-ticket-canvas, it has no server mode.
- Writing to a forge in anybody's name but the person running vuoro.

Section 10 says what lands in the MVP and what lands in v1.0.0.

## 3. Shape

One Go binary, `vuoro`, built with `CGO_ENABLED=0` and cross-compiled, as every
terva-sh release is.

```text
vuoro            opens the desktop window (tuohi) over the loopback server
vuoro -browser   serves on loopback and prints the URL, for a browser or debugging
```

The process runs a `net/http` server on `127.0.0.1` and points a tuohi window at
it. This is the pattern in `tuohi/examples/loopback`. The frontend is Preact
and TypeScript, built with Vite, as in git-ticket-canvas. Its built assets are
committed in `web/dist` and embedded, so `go build` needs no Node toolchain.
Styles come from `terva-sh/design` through `design-tokens export -app vuoro`,
with the `-check` form in CI.

### 3.1 Packages (target)

| Package | Holds |
|---|---|
| `forge` | The forge-neutral model (section 4) and the `Forge` interface. No HTTP. |
| `forge/gitlab`, `forge/github`, `forge/forgejo` | One adapter each. All the API knowledge lives here. |
| `turn` | Turn derivation: pure functions from model to turn and its reasons. No I/O, so it is tested from fixtures. |
| `state` | The cache and the person's annotations, under the state directory. |
| `server` | The HTTP API and the embedded frontend. |
| `agent` | The driver interface and normalised events (section 6). `agent/terva` and `agent/claude` are the drivers, and they are the only code that knows a tool's protocol. |
| `review` | Review runs: worktrees, profiles, finding validation, and the drafts channel the agent writes through. |
| `tickets` | The optional git-ticket integration (section 7). The only package that imports git-ticket. |
| `web` | Frontend source and the committed `web/dist`. |

`agent`, `review`, and `tickets` are kept apart so the inbox builds and tests
without any of them. A test holds that boundary, in the same way as
`terva/examples/harness/imports_test.go`.

### 3.2 Pins

| Dependency | Pin at bootstrap | Note |
|---|---|---|
| Go | 1.26 | tuohi's line. vuoro imports no agent tool, so terva's 1.27 does not apply (section 6.1). |
| terva (a tool, not a module) | v0.139.7 | Driven over `terva rpc`. The protocol major version is in its `hello` response. |
| Claude Code (a tool) | 2.1.285 | Driven over stream-json. The version on this workstation at bootstrap. |
| `github.com/terva-sh/tuohi` | v0.1.0 | The first release. Its README still warns that the API can change. |
| `github.com/terva-sh/design` | v0.1.0 | Has no `vuoro` accent yet. |
| `github.com/terva-sh/git-ticket` | v0.24.0 | Library and CLI. This workstation had v0.23.0 installed. |

These were read from the internal Forgejo (`git.local.sothr.com/terva-sh`), which
is every terva-sh repository's `origin`. The GitHub mirrors trail it by days, so
read a pin from Forgejo and never from a mirror.

## 4. The model

> Open question: TKT-01M3W4HWJZ1013S2MPZ69B72AW (Decide the forge model and the
> turn rules). This section is the starting position, not the answer.

- **Change**: one merge request or pull request. Forge, project, number,
  title, author, web URL, source and target branch, head SHA, draft flag,
  state (open, merged, closed), mergeability.
- **Role**: what the person is to a change. One or more of author, requested
  reviewer, assignee, participant.
- **Review**: one reviewer's verdict at a SHA. approved, changes requested,
  commented, or dismissed.
- **Thread**: a discussion, optionally anchored to a file, line, and SHA, with
  its notes, and resolved or not.
- **Draft**: a pending comment that the forge has not published yet. GitLab
  draft notes, a GitHub pending review, a Forgejo pending review. A draft can be
  authored by the person or drafted by an agent.
- **Turn**: for each change and the person, `mine` or `theirs`, a `since`
  time, and the reasons that produced it.

### 4.1 Turn rules (starting position)

As a requested reviewer, it is my turn when any of these is true:

- I have not reviewed the change.
- Commits were pushed after my last review.
- A thread I started or replied in has a newer note from somebody else.

As the author, it is my turn when any of these is true:

- An unresolved thread has a reviewer note newer than my last note in it.
- A reviewer requested changes, and nothing was pushed since.
- The change is approved and mergeable. It is my turn to merge.

Otherwise it is their turn, and the inbox says whose. `since` is the time of
the event that made it so, which is what lets the inbox sort by "waiting
longest".

## 5. Forges

GitLab comes first, because that is where the owner's day-job reviews are.
GitHub and Forgejo follow, each as its own ticket against the `forge`
interface. If the interface does not fit a forge, that is a finding against the
model, not a reason to leak forge detail through the interface.

GitLab notes for the adapter:

- Discover changes with `GET /merge_requests?scope=all&reviewer_username=` and
  `author_username=`, and read each one's discussions, approvals, and draft
  notes.
- Comments go out as draft notes and publish together with
  `POST .../draft_notes/bulk_publish`. That matches what a reviewer expects
  from "start a review".
- Inline positions need `base_sha`, `start_sha`, and `head_sha` from the
  change's diff refs.

### 5.1 Credentials

> Open question: TKT-01M3W4HWM6V2F20KNT03BSBNZY (Decide credential handling and
> the local state format).

- A token never goes in a config file, in `argv`, or in a log.
- The starting position is a configured `token_command`, run without a shell
  with its output read from stdout. An environment variable is the fallback.
  This lets any secret store supply the token without vuoro knowing which one
  it is.
- A keychain is attractive. Without cgo, it means calling `security` on macOS
  and the Secret Service over D-Bus on Linux. The spike decides whether that is
  worth it for the MVP.

### 5.2 The loopback server is local attack surface

Every process on the machine can reach `127.0.0.1`. The server carries the
person's forge tokens, so it refuses, with 403, any request that does not
prove it came from vuoro's own page. TKT-01M3W4HWDY8XV92MXXQWA0AES7 (Refuse
loopback API requests that did not come from vuoro's window) settled the
mechanism on 2026-10-02, and `server/guard.go` holds it:

- **It listens on 127.0.0.1 only.** `server.Listen` refuses every other
  address, including `localhost`, `::1`, and `0.0.0.0`.
- **Every route checks where the request came from.** `Host` must be exactly
  `127.0.0.1:PORT`, so a page that rebinds a DNS name of its own to
  127.0.0.1 is refused. `Origin`, when sent, must be vuoro's own origin, and
  `Sec-Fetch-Site`, when sent, must be `same-origin` or `none`.
- **The API needs a per-launch token.** Every route under `/api/` needs the
  token in the `X-Vuoro-Token` header, compared in constant time. It is 32
  random bytes, made at launch and never written down. The window reads it
  through the `vuoro.token` binding, which tuohi offers only to the origin the
  view trusts. `vuoro -browser` prints it in the URL fragment, which a browser
  never sends to a server. The page moves it into `sessionStorage`, which is
  scoped to the origin and port, and takes it out of the address bar.
- **No cookie.** Browsers do not isolate cookies by port, so another local
  server on 127.0.0.1 would receive one.
- **The frontend's own files need no token.** They are the embedded build
  output, the same for every launch, and a page has to load before it can
  hold a token. They still pass the origin checks.

A server added later, such as the drafts MCP endpoint, uses the same guard
with a credential of its own.

## 6. Agents

> The drafts channel, the two agent classes, and the orchestration shape were
> decided with the owner on 2026-10-02. The driver spike,
> TKT-01M3YK0CFFKAPM2QHZGGZMDHND (Decide the agent driver interface for terva
> and Claude Code), settled the interface and Claude Code's lockdown in section
> 6.6. Open question: terva's lockdown, which terva 0.139.7 cannot provide.
> Section 6.6 says why, and D28 defers terva past the MVP.

### 6.1 Drive the tools, do not embed them

vuoro runs reviews itself, and the person talks to the agent while it does.
vuoro does not contain an agent. It drives agent tools that the person already
has, each as a subprocess over the tool's own protocol, so the person picks the
tool, the model, and the account:

| Tool | Protocol | Resume | Lockdown |
|---|---|---|---|
| terva | `terva rpc`: NDJSON commands and events on stdin and stdout, `hello` with `TERVACORE_RPC_TOKEN` | `--session PATH` | Not available in 0.139.7 (section 6.6) |
| Claude Code | `claude -p --input-format stream-json --output-format stream-json`, the protocol its SDKs speak | `--session-id`, `--resume` | Proven (section 6.6) |

Each tool sits behind one driver interface in `agent`. It covers starting a
session in a working directory with a system prompt, a tool policy, and a
model, sending a message, streaming events, cancelling, closing, and resuming.
A third tool is a third driver. Nothing else in vuoro knows which tool runs.

This decision, made by the owner on 2026-10-02, has three consequences:

- **vuoro holds no model credential.** Each tool uses its own login or key.
  vuoro records the tool, its version, the model, and the account kind where
  the tool reports it, and it shows them on every run. The person can always
  see where their code went.
- **vuoro imports neither tool.** The Go line stays at tuohi's 1.26, and a
  terva or Claude Code release cannot break vuoro's build. It can break a
  driver, so each driver checks the tool's version and says when it is too old.
- **The tools are prerequisites.** vuoro works without either, with every
  agent action off. With one installed, that one is offered.

### 6.2 A review is an orchestrated conversation

A review run checks out the change's head in a worktree under the state
directory, and computes the diff against the merge base. Then it starts the
sessions that the person's preset names. Each session is a tool, a model, and a
profile, and they run in parallel, each in a pane the person can talk to. The
person asks why, sends a reviewer somewhere else, or asks it to drop or reword a
finding. When the reviewers finish, a synthesizer session reads every
reviewer's drafts. It proposes merged drafts that name their sources, and it
says what it set aside and why. A finding only one reviewer raised is kept and
labelled. It is not voted away.

A finding has severity, title, body, evidence, and an optional anchor (path,
side, SHA, line range). Each valid finding is a pending draft comment, marked
with the tool and model that drafted it.

**Decided 2026-10-02: agents write drafts through an MCP server that vuoro
serves** to each session. Its tools list, propose, revise, and withdraw pending
drafts. A reviewer sees its own drafts, and the synthesizer lists them all. It
behaves the same for both tools, and the agent sees the result of each edit at
once. It also makes the boundary structural: an agent's only write tool writes
to a draft. A proposed draft is validated, and a bad one comes back with the
rule it broke, so the agent can correct it.

A run is pinned to the head SHA it reviewed. When the change moves, the run and
its drafts show "reviewed at abc123, 2 commits behind". Nothing re-runs on its
own.

Every session pane shows its tool, model, tokens, and cost as the tool reports
them, and the run shows a total.

### 6.3 Two classes of agent

**Reviewers are read-only, always** (decided 2026-10-02). A reviewer reads the
worktree through Read, Grep, and Glob, and it writes only through the drafts
server. It gets no shell, no web access, no extensions, and none of the person's
own MCP servers. Those can include a forge connector that writes, and a shell
can run `glab` or `gh`. A reviewer never runs the change's code. The driver
spike proves the lockdown for each tool by asking the agent to run a command and
recording the refusal. A driver that cannot prove its lockdown is not offered.

**The editor is the only agent that writes code** (v1.0.0). Mutation is a
different class of agent with its own rules, so it is kept apart:

- A review gets at most one editor. Reviewers run in parallel because they
  cannot write. The editor is never parallel with another writer.
- It works from the reviewers' priors: on the person's own change, the triaged
  threads and the findings.
- It runs in a sandbox, by default a container with only its worktree
  mounted, no forge credentials, and network limited to the model endpoint and
  package registries. A spike decides the sandbox before the editor is built.
- It commits on a local branch, one commit per finding or thread it addresses.
  The person reviews those commits as a diff, and then accepts them, which
  makes vuoro push to the merge request branch, or discards them. The editor
  never holds a forge credential and never pushes.

### 6.4 Other actions

Summarising a change, summarising what changed since my last review, and the
author's triage are reviewer-class work: the same drivers and the same
lockdown. Triage groups the open threads on the person's own change as must
fix, discuss, or nit, and drafts replies through the drafts server. None of it
is posted by an agent.

### 6.5 Prior art

terva-sh/terva-action-code-review is an internal proof of concept that runs
terva as a Forgejo pull-request reviewer. vuoro is the front door to that kind
of work, not a consumer of it, and it neither runs nor depends on the action.
The action is worth reading for:

- its review profiles and prompt templates (`templates/`)
- its finding and anchor shapes (`schemas/review-result.v1.schema.json`)
- its rule that reviewer settings come from the base commit, so a change
  cannot pick its own reviewer
- its panels, which run several models and reconcile what they find
- its dispositions: accepted, declined, deferred
- its `terva rpc` lockdown flags in `src/terva.mjs`

terva's `raati` deliberation panel is prior art for the synthesizer, and for
keeping a minority finding rather than voting it away.

### 6.6 Drivers

Settled by the driver spike on 2026-10-02, against Claude Code 2.1.285 and
terva 0.139.7. The evidence and the throwaway program are in
`spikes/agentdriver/`.

**The interface.** A driver starts a session and the session carries turns.

- `Probe` finds the binary on `PATH` (`exec.LookPath`), reads its version, and
  says plainly when the tool is missing or older than the pin in section 3.2.
  It also reports the account kind where the tool exposes it.
- `Start` takes a working directory, a system prompt, a model, the drafts
  endpoint for this session, and an optional resume reference. There is one
  tool policy, the reviewer's. The editor's arrives with the editor.
- A session has `Send(text)`, which starts a turn, an event stream,
  `Cancel`, which stops the turn in flight, `Close`, and `Ref`, the handle
  that a later `Start` resumes. stream-json cannot abort one turn and keep the
  process, so cancelling a Claude Code turn ends the session. Resuming it from
  `Ref` continues the conversation.
- A driver checks the lockdown at run time as well. If Claude Code's `init`
  event offers any tool outside the allowed set, the driver ends the session
  before any of the session's events reach vuoro.
- The events both tools honour: text delta, tool call, tool result, denied
  (the tool and its input), usage, turn done (with an error when it failed),
  and session info (tool, version, model, account kind). Usage is per turn.
  Claude Code reports `total_cost_usd` as a running total for the process, so
  its driver reports the difference.
- No approval request reaches the person. A reviewer's policy is fixed, so the
  driver answers or refuses every request itself and reports a refusal as a
  denied event.

**Claude Code's lockdown**, proven:

```text
claude -p --input-format stream-json --output-format stream-json --verbose
  --include-partial-messages --model M --system-prompt P
  --tools Read,Grep,Glob --restricted
  --strict-mcp-config --mcp-config <vuoro's drafts server only>
  --permission-mode dontAsk --permission-prompts none
  --allowedTools Read,Grep,Glob,mcp__vuoro__<each drafts tool>
  --setting-sources "" --disable-slash-commands
```

- The session's `init` event lists exactly the three read tools and the drafts
  tools, and vuoro's server as the only MCP server. None of the person's MCP
  servers or claude.ai connectors loads, and no skills do.
- A tool that is not defined cannot be called. When `Bash` is defined but not
  allowed, `dontAsk` denies it, and the `result` event lists the denial. That
  is a second layer if a later release stops honouring `--tools`.
- `--restricted` confines the file tools to the working directory. A read of
  `/etc/hosts` is refused by the harness.
- A `CLAUDE.md`, an `AGENTS.md`, or a `.claude/settings.json` hook in the
  reviewed worktree does not reach the session. Without these flags, the
  change's hook runs. A repository's own instructions reach a reviewer only
  through vuoro, read at the base commit (D14).
- `--bare` is not used, because it refuses a subscription login.
- The driver drops `CLAUDECODE` from the child's environment.

**terva's lockdown is not available in 0.139.7.** terva cannot be offered as a
reviewer until it is, and D28 moves it past the MVP. Two gaps:

- **No server can be supplied for one run.** terva reads MCP servers only from
  `$TERVA_HOME/config.json` and a trusted project's config. `--mcp` narrows
  that list and cannot add to it. `--team-socket` replaces it with the
  hard-wired Talkoot bridge. Pointing `TERVA_HOME` at a directory vuoro owns
  moves `auth.json` with it, so the person's login goes too. A symlinked
  `auth.json` is replaced by the first OAuth refresh. `--ext DIR` loads an
  extension even under `--no-ext`, but it speaks terva's extension protocol,
  not MCP (D9).
- **The person's hooks always load.** Hooks in `$TERVA_HOME/config.json` run
  shell commands, and no flag turns them off. A reviewer would carry them.

The flags that hold today, for when those gaps close: `--tools
read,grep,glob --no-ext --no-mcp --no-skill --no-lore --no-memory --no-ticket
--portable=strict --system-prompt P`. `--tools` filters built-in tools only.
`activate_tools` survives it, but it reveals only tools already registered, so
it cannot bring back `bash` or `write`. Under `--approval plan`, an MCP tool
registers only if it declares `readOnlyHint`. Otherwise, use `--approval
auto-edit --rpc-approvals`, under which the driver allows only the drafts
tools.

**The drafts channel.** The spike served a stub over stdio, and the agent's
proposals arrived and were validated. TKT-01M3YMSSRC81M0BYHK15TW9VDY (Serve the drafts MCP
server to each agent session) decides the transport. The starting position is
Streamable HTTP on the loopback server, at a per-session path with a
per-session bearer. The config that names it is written to a file with mode
0600 and passed by path, so the bearer never appears in `argv`.

**What a run records:** the tool, its version, the model, and the account kind.
For Claude Code, the account kind is `apiKeySource` from `init`, plus
`authMethod` and `subscriptionType` from `claude auth status`. vuoro reads only
those fields, because the same output carries the email address. For terva,
`hello` gives `version`, `protocol_version`, `provider`, and `model`.

**Retention.** Claude Code keeps its own transcript of every session under
`~/.claude/projects/`, keyed by the working directory. Section 8's retention
deletes it with the run's, or day-job code lingers there.

## 7. git-ticket (optional)

> Open question: TKT-01M3W4HX0BNXTDC2DXBH5HQ0FM (Decide how a change finds its
> git-ticket store and ticket).

Most of the owner's repositories carry a git-ticket store, and the terva-sh
review process asks that each finding's disposition be recorded in the ticket.
vuoro can do that work when it is available. It must work as well without it.

When a change's repository has a known local checkout that contains a
`.tickets` store, vuoro can:

- **Link a change to its ticket.** Use a `TKT-` ID in the title, branch, or
  description, or a `pr:` or `mr:` reference on the ticket.
- **Show the ticket beside the change.** Description, acceptance criteria,
  plan, and notes, so a reviewer reviews against what was asked.
- **Record review dispositions as a ticket note,** attributed with
  `--actor`-style identity, and written but never committed or pushed. This
  matches git-ticket-canvas: the person commits.
- **Show the person's own ready queue** beside the review queue, read-only, as
  a later option.

It uses the `ticket` library package, as git-ticket-canvas does. It does not
shell out to the CLI.

## 8. State

Review runs, their transcripts, and their draft history are vuoro's own
records. They are kept while the change is open, and for 30 days after it
merges or closes (configurable). Then the transcripts go, and one line per run
remains: tools, models, cost, and finding counts. Day-job code does not linger
in local files.

The state directory follows git-ticket-canvas:
`$XDG_STATE_HOME/vuoro` or `~/.local/state/vuoro` on Linux,
`~/Library/Application Support/vuoro` on macOS, and `%LOCALAPPDATA%\vuoro` on
Windows. Configuration lives in the platform config directory.

> Open question: the cache format, in TKT-01M3W4HWM6V2F20KNT03BSBNZY. JSON
> files are enough for one person's inbox. `modernc.org/sqlite` is the cgo-free
> option if queries need it.

## 9. Gates

`just ci` is the gate, and CI runs the same steps:

- `gofmt` and `go vet`, with `CGO_ENABLED=0`
- `go test ./...`, with `-race` on darwin, the only platform where the race
  detector works without cgo
- cross builds for linux, darwin, and windows
- the frontend typecheck and tests, and a check that `web/dist` matches source
- `design-tokens export -check`
- `git ticket check --fix --dry-run --strict`

## 10. Roadmap

Scoped with the owner on 2026-10-02. Each line names its epic.

### 10.1 MVP: v0.1.0

TKT-01M3W4FGT1GY0X3TWJF81X3XWK (vuoro v0.1.0 (MVP): orchestrated GitLab
reviews, then the inbox). For the owner, on macOS, built orchestrator first.

1. **Foundations.** The module and CI gate without cgo, the tuohi window over
   the loopback server and its protection, the Preact frontend, and the design
   stylesheet. The repository moves to terva-sh/vuoro on Forgejo once the
   skeleton lands, with no public mirror yet.
2. **The orchestrator, the first usable slice.** Open a GitLab merge request by
   URL. Run a preset's reviewers in parallel on Claude Code (D28).
   Talk to them, let a synthesizer merge their drafts, edit the drafts, and
   submit the review. Presets and profiles live in vuoro's own config, and code,
   security, and documentation profiles are built in. Cost is shown, and runs
   are pinned to their head SHA and flagged when it moves.
3. **The author's side.** Triage the threads on my change, and draft replies.
4. **The inbox, second.** Discover every GitLab change I review or wrote,
   derive whose turn it is with the reasons, and open any of them in the
   orchestrator. It refreshes only while the window is open.
5. **git-ticket, read-only.** Show a change's linked ticket, and give its
   acceptance criteria to the reviewers.
6. **History.** Keep runs until the change closes, then 30 days, then one line
   per run.

### 10.2 v1.0.0

TKT-01M3YMSTYMFRDFXD4QSVWHMM8P (vuoro v1.0.0: every forge, the editor agent,
and a terva-sh release).

- **Every forge.** Forgejo next, then GitHub.
- **terva as a reviewer**, once a terva release allows its lockdown (section
  6.6, D28).
- **The editor agent** (section 6.3), on the author's side, with its sandbox
  decided by a spike first.
- **Reviews before a push.** Review a local branch or range with no forge, and
  carry the findings into the merge request.
- **Repositories add instructions.** A checklist and conventions, read at the
  base commit, layered on the person's preset. A repository never picks the
  reviewers.
- **Moving changes.** Re-anchor drafts on unchanged lines, and review only the
  new commits, with the earlier findings as context.
- **Cost.** Optional per-run budget caps, and a monthly total.
- **Nudges.** The tray, background refresh, and quiet, batched notifications
  when it becomes my turn or a long run finishes.
- **git-ticket, written.** Finding dispositions as ticket notes, and the
  linked ticket as context for the editor.
- **A terva-sh release.** goreleaser archives for macOS, Linux, and Windows,
  `self-update`, setup in the UI with no hand-edited config, a GitHub mirror,
  and documentation for somebody new. macOS signing is decided then.

### 10.3 After v1.0.0

- The editor on other people's changes, turning findings into suggested
  patches.

### 10.4 Decided against

- **A reviewer that runs code.** Reviewers are read-only, always. Running the
  change's code is the editor's job, in its sandbox.
- **Automatic re-runs on push.** A run never starts without the person.
- **Refusing a subscription login.** The employer policy that prompted the
  idea is about PHI, and a review in vuoro is a person running review sessions
  by hand. vuoro shows which account each run used, and leaves the choice to
  the person.
