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
with the `-check` form in `just ci` and both forge workflows. The exporter
is pinned as a Go tool to design v0.3.0 in `go.mod`.
Vuoro uses violet on Birch (`#7546A4`) and lavender on Tar (`#B89AE3`),
chosen in D32. Components read the shared `--ui-*` roles. `just design-export`
refreshes the committed stylesheet; `just web-build` rebuilds its embedded
frontend assets.

### 3.1 Packages (target)

| Package | Holds |
|---|---|
| `forge` | The forge-neutral model (section 4), `Provider`/`Opener` and optional `Writer` contracts. No HTTP. |
| `forge/adapters` | Parsers and factories select a provider by configured kind, or anonymous URL shape. No API orchestration. |
| `forge/gitlab`, `forge/github`, `forge/forgejo` | One adapter each. All the API knowledge lives here. |
| `inbox` | Account-scoped discovery, cached snapshots, scrubbed refresh status and authored snoozes. No agent or ticket dependency. |
| `turn` | Turn derivation: pure functions from model to turn and its reasons. No I/O, so it is tested from fixtures. |
| `state` | The cache and the person's annotations, under the state directory. |
| `state` | Whole-file, private writes under the state directory. |
| `server` | The loopback server, its guard, and the embedded frontend. |
| `app` | The API under `/api/`: opening changes and driving review runs. The server mounts it behind the guard. |
| `agent` | The driver interface and normalised events (section 6). `agent/terva` and `agent/claude` are the drivers, and they are the only code that knows a tool's protocol. |
| `review` | Review runs: worktrees, profiles, finding validation, and the drafts channel the agent writes through (`review/drafts`). |
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
| `github.com/terva-sh/design` | v0.1.0 | Bootstrap version; the current stylesheet and exporter pin v0.3.0, with vuoro's violet preset. |
| `github.com/terva-sh/git-ticket` | v0.24.0 | Library and CLI. This workstation had v0.23.0 installed. |

These were read from the internal Forgejo (`git.local.sothr.com/terva-sh`), which
is every terva-sh repository's `origin`. The GitHub mirrors trail it by days, so
read a pin from Forgejo and never from a mirror.

## 4. The model

Decided on 2026-10-02 by TKT-01M3W4HWJZ1013S2MPZ69B72AW (Decide the forge
model and the turn rules). The model was checked against real API responses
from public changes on gitlab.com (gitlab-org/cli !3958 and !3991), GitHub
(cli/cli #13899, #9083, and home-assistant/core #184070), and Forgejo on
Codeberg (forgejo/forgejo #4212). The GitLab responses are the fixtures in
`forge/gitlab/testdata`.

- **Change**: one merge request or pull request. Forge, project, number,
  title, author, web URL, source and target branch, base and head SHA, draft
  flag, state (open, merged, closed), and mergeability as the forge reports
  it. The authenticated person’s merge permission is nullable (`can_merge`):
  unknown is never treated as allowed.
- **Role**: what the person is to a change. One or more of author, requested
  reviewer, assignee, and participant.
- **Review request**: a reviewer was asked, at a time. A re-request is a new
  one. All three forges record it, and it is the clearest "your turn" signal
  they give.
- **Review**: one reviewer's verdict, at a time: approved, changes requested,
  or commented, and whether it was dismissed or marked stale. It carries the SHA it was given
  at when the forge records one. GitHub and Forgejo do. GitLab keeps only each
  reviewer's current state, so the adapter reads the time from the state's
  system note.
- **Push**: new commits on the change, at a time, with the head SHA when the
  forge gives it. A commit's own date is when it was written, not when it
  arrived, so the model never uses it.
- **Thread**: a discussion, optionally anchored to a file, line, side, and
  SHA, with its notes, and whether it can be resolved and is. A note has an
  author, a time, and two marks: *system* (the forge wrote it, such as
  "changed this line in version 3") and *bot* (an automated account wrote it).
- **Draft**: a pending comment the forge has not published: GitLab draft
  notes, a GitHub pending review, a Forgejo pending review. The person writes
  one, or an agent drafts it.
- **Turn**: for each change and the person, `mine` or `theirs`, a `since` time,
  the reasons that hold, and the time of the last activity.

**Bots and system notes count for nothing.** A note or review by a bot, or a
system note, never makes it anyone's turn. Bots are recognised by the forge's
own marks (GitHub's `[bot]` suffix, GitLab's bot user type) and by a list in
the config. GitLab Duo, for example, appears both as a reviewer and as a
thread author.

### 4.1 Turn rules

Each rule is a reason. A change is my turn when any reason holds, and
otherwise it is theirs. The inbox names whose: the reviewers who have not
answered, or the author.

| Rule | GitLab (GraphQL) | GitHub | Forgejo |
|---|---|---|---|
| **R1** I am a requested reviewer and have not reviewed | `reviewers.nodes[].mergeRequestInteraction.reviewState == UNREVIEWED` | I am in `requested_reviewers` and have no review | a `REQUEST_REVIEW` review, or `requested_reviewers`, with no later review of mine |
| **R2** pushed since my last review | `mergeRequestDiffs.nodes[].createdAt` newer than my verdict's system note | my last review's `commit_id != head.sha` | `stale`, or `commit_id != head.sha`; the push time is the `pull_push` timeline entry |
| **R2b** re-requested since my last review | system note "requested review from @me" newer than my verdict | timeline `review_requested` naming me | timeline `review_request` naming me |
| **R3** the author answered in a thread I am in | the last note that is neither system nor bot is the author's, and it is newer than mine | the same, over the `in_reply_to_id` chain of `/pulls/:n/comments` | the same, over a review's comments with one `path` and `position` |
| **A1** an unresolved thread ends with a reviewer's note | `resolvable && !resolved`, and the last note that is neither system nor bot is not mine | the last comment in the chain is not mine. Resolution needs GraphQL `reviewThreads.isResolved` | `resolver == null` on the first comment, and the last comment is not mine |
| **A2** changes requested, and nothing pushed since | reviewer state `REQUESTED_CHANGES`, and no diff version newer than its system note | the reviewer's last non-comment review is `CHANGES_REQUESTED` at `head.sha`, and not dismissed | `REQUEST_CHANGES`, not `dismissed`, not `stale` |
| **A3** approved and mergeable, and I can merge | `detailedMergeStatus == MERGEABLE` and `canMerge` | `mergeable_state == clean` with no open change request, and I can merge | `mergeable`, an approval and no open change request, and I can merge |

R1 to R3 apply when I am a requested reviewer, and A1 to A3 when I am the
author. R3 counts the author's notes only, so a co-reviewer's "+1" does not
make it my turn.

**A thread's turn is its last real note.** Leaving out system and bot notes,
when the last note is the author's, it is the reviewers' turn, and when it is
a reviewer's, it is the author's. That covered every thread in the fixtures,
so a thread does not record who is expected to answer. An author's deferring
reply, such as "I'll fix it in a follow-up", hands the turn back, which is
right: the reviewer resolves the thread or answers.

**`since` is the earliest unanswered event** among the reasons that hold. It
answers "how long have I owed this", which is what the inbox sorts by. Bot
nudges, "+1"s, and a second push do not move it, and it moves on only when
the person acts. The latest event is shown as last activity, and the inbox
never sorts by it. `turn.Derive` returns structured rule/text/thread reasons for
both `mine` and `theirs`. Resolved threads carry no unanswered reply reason.
Current reviewer state supplements dated verdict history, and the last human
non-comment verdict supersedes earlier ones; stale or dismissed verdicts cannot
resurrect an earlier verdict. Merge readiness requires explicit authenticated
permission, an approval, no current change request, and a non-draft change.

Explicit current change requests remain actionable with an unknown age when
the historical verdict timestamp is unavailable. A known re-request also remains
actionable when undated request history makes its earliest age unknown.

A reason’s time and the overall `since` are nullable. When an unanswered event
has no recorded timestamp, its age remains unknown; creation, update and commit
authorship times never stand in for it. Undated notes retain adapter order;
fully dated notes sort chronologically. Last activity can include forge noise,
but noise does not alter unanswered reasons or their age. Closed and merged
changes carry a readable inactive reason and no actionable turn.

### 4.2 What each forge needs

- **GitLab: read with GraphQL, and write with REST.** The GraphQL query reads a
  change's reviewer states, threads, positions, system notes, and diff
  versions in one request: `forge/gitlab/query.graphql`.
  REST is for draft notes and publishing. A trap: in REST, `reviewers[].state`
  is the account's state (`active`), not the review state.
- **GitHub** needs GraphQL for thread resolution (`reviewThreads.isResolved`),
  and it puts a dismissed review's original verdict only in its timeline.
- **Forgejo** records real push events. Replies have no parent ID, and they are
  grouped by review, `path`, and `position`.

## 5. Forges

GitLab came first because it hosts the owner's day-job reviews. D33 brings
Forgejo open-by-URL reading and agent reviews into the MVP. D34 brings Forgejo
discovery and the shared GitLab/Forgejo inbox into the MVP as the next milestone.
Forgejo writes remain later work, with GitHub after Forgejo (D21).

`forge.Provider` reads a change and reports implemented actions available with
its credentials. `forge.Writer` is a separate optional contract reached only
through the person's protected API actions. Capability flags are not a promise
of repository permission: the forge authorizes each write. GitLab supplies
writing capabilities when authenticated; Forgejo supplies none in this slice.
The UI uses these flags, so configuring a Forgejo account enables private reads
without enabling posting, keeping findings on the forge, or inline suggestions
that would post there. Agent findings remain local drafts.

`forge/adapters` owns the parser/factory registry. A configured host's kind wins;
unconfigured URLs use the GitLab `/-/merge_requests/` or Forgejo `/pulls/` path
shape and read anonymously. GitHub's public host is refused until its adapter
exists. Each host has one configured account, and clients and credentials are
cached by host and kind. Application code consumes neutral errors and metadata;
it imports no individual adapter. A new forge extends the registry and contracts
without adding API details to the application or review runner.

Checkout metadata comes from the adapter: HTTPS and SSH clone URLs and the
change's head ref in its target repository. GitLab uses
`refs/merge-requests/N/head`; Forgejo uses `refs/pull/N/head`, including fork
heads. Advertised Forgejo SSH URLs retain explicit ports. The application
validates hosts, credentials in URLs, refs and branch names before fetching,
and git uses the person's own transport credentials. Run records retain forge
kind for display; existing GitLab records and Ref identity keys stay compatible.

Forgejo reads the PR, paginated file inventory, reviews, review comments and
issue timeline. The separate `.diff` response supplies patches. Binary,
missing or incomplete patches are marked omitted, including incomplete hunk
counts or a mismatch with the inventory's addition/deletion totals. Pages run
to an empty response, respecting server-clamped limits, with bounded response
sizes and page counts. Absolute pagination links and redirects are never
followed with the token. A final PR read refuses mixed snapshots when the head
or target moved while paginated data and patches were read. Review anchors use `position` for the new-side line
and `original_position` for the old-side line. Comments group by review, path
and those line coordinates; the first comment supplies thread resolution.
Pending reviews are drafts rather than published threads. Push times come from
`pull_push` timeline entries, never commit authorship dates.

The current Forgejo API supplies no bot-user marker. Vuoro does not infer bot
status from account names. Each configured forge may name automated accounts
in `bots`; the inbox applies those names and native bot markers to users and
reviews before turn derivation. Configured marks are applied to copies of raw
cached facts, so removing a configured bot does not leave a permanent mark.
If a forge does not fit the contracts, revise the model rather than leaking its
API details through the application. Agent and ticket integrations keep their
own contracts (D33).

`forge.Discoverer` is optional beside `Provider`. It returns complete open
membership for a username, with all roles and forge update stamps; pagination
and API details stay in adapters. Complete membership makes removals safe.
The `inbox` service deduplicates stable `Ref.String()` identities, merges roles,
and reuses a cached change only when its update stamp matches a nonzero
membership stamp. Missing stamps always cause a new read. Discovery and all
reads must succeed before replacing an account snapshot. A change that closes
between discovery and reading drops out of that successful snapshot.

Refresh runs only when called by the window, with at most four accounts in
parallel and a two-minute context per account. Calls are serialized. Failed,
cancelled, timed-out and rate-limited accounts retain their items and last
success, and expose fixed credential-free messages. Healthy accounts replace
their own snapshots independently. The service launches no background poller.

GitLab notes for the adapter:

- Discover changes with `GET /merge_requests?scope=all&reviewer_username=` and
  `author_username=`, and read each one's discussions, approvals, and draft
  notes.
- Comments go out as draft notes and publish together with
  `POST .../draft_notes/bulk_publish`. That matches what a reviewer expects
  from "start a review".
- Inline positions need `base_sha`, `start_sha`, and `head_sha` from the
  change's diff refs.

### 5.1 Credentials and configuration

Decided on 2026-10-02 (D29), and held in `config/`.

- **A token never goes in the config file, argv, a log, or an error.** The
  file names where the token comes from. A `token` field is refused, as is any
  field the format does not know.
- **A token command.** `token_command` is a program and its arguments, run
  without a shell, whose stdout is the token. A leading `~/` in the program is
  the home directory. Any secret store fits: the owner's is `ritual-secret get
  gitlab-pat`. `token_env` names an environment variable instead, for a forge
  with no command.
- **In memory only.** The token is held as a value that prints as
  `[redacted]` under every format, JSON, and `slog`. A failed command is
  reported by its program name and exit status. What it printed, on stdout or
  stderr, never appears in a message.
- **No keychain in the MVP.** Without cgo it means `security` on macOS and the
  Secret Service over D-Bus on Linux, and a token command already reaches
  either.

The config file is `config.json` in the platform config directory:
`~/Library/Application Support/vuoro` on macOS, `$XDG_CONFIG_HOME/vuoro` or
`~/.config/vuoro` on Linux, and `%AppData%\vuoro` on Windows. In the MVP the
person writes it by hand. Setup in the UI arrives at v1.0. An example, with no
secret in it:

```json
{
  "forges": [
    {
      "name": "gitlab.com",
      "kind": "gitlab",
      "url": "https://gitlab.com",
      "username": "drewshort",
      "token_command": ["~/.claude/scripts/ritual-secret", "get", "gitlab-pat"]
    }
  ]
}
```

Four fields are optional:

- `bots` on a forge names automated usernames on that host, supplementing the
  forge’s native markers without guessing from names.

- `clone` on a forge is the git transport a review fetches the project over:
  `ssh`, the default, or `https`. Either uses the person's own keys or
  credential helper, and vuoro never holds a git credential.
- `presets` name the reviewers a run starts, each a `profile` with an
  optional `tool` and `model`. With none, a run starts one code reviewer on
  the tool's default model.
- `profiles` are the person's own, beside the built-in code, security, and
  documentation profiles, each a `name`, a `title`, and a `prompt`.

With no config file, a change on a public project still opens, anonymously.
The person's draft notes on the forge need a token, so they are not read.

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

**Keeping a draft is the person's act.** An agent's draft stays in the run
until the person keeps it, edits and keeps it, or drops it. Keeping it
creates one of the person's own pending comments on the forge (a GitLab draft
note), which no one else sees. Submit publishes all of them at once
(`draft_notes/bulk_publish`), with an approval if the person asks for one.
Submitting, approving, and revoking an approval each ask first, saying what
others will see and as whom, with Cancel focused, because one stray click
otherwise reaches the forge. The person can waive asking for submitting or for
approving. The waiver is kept in `prefs.json` in the state directory, not the
config file, and the home page can take it back. A
kept or dropped draft can no longer be changed by its agent. An inline
comment lands on the last line of the draft's range, because GitLab's
multi-line positions need a line code that vuoro does not build yet. A draft
anchored on a head that has since moved is refused rather than placed on the
wrong line. The agent and review packages depend on no forge adapter, and a
test holds that.

An anchor must name lines that the change's diff shows, inside one hunk, on
the side it names, because a forge places an inline comment only there. A
draft anchored anywhere else comes back with the rule `anchor.notInDiff` and
the ranges it may use, or `anchor.notInChange` for a file the change does not
touch. A finding about any other place drops its anchor and names the place
in its description.

A run fetches into a bare, partial clone under `repos/` (`--filter=blob:none`,
so history costs commits and trees), taking the merge request's head ref
(`refs/merge-requests/N/head` on GitLab, which also covers a change from a
fork) and the target branch. git runs with hooks off and prompts off. The
first message carries the change's title, description, open threads, and the
forge's diff, up to 400 KiB, and lists the files past that for the reviewer
to read.

**Decided 2026-10-02: agents write drafts through an MCP server that vuoro
serves** to each session. Its tools list, propose, revise, and withdraw pending
drafts. A reviewer sees its own drafts, and the synthesizer lists them all. It
behaves the same for both tools, and the agent sees the result of each edit at
once. It also makes the boundary structural: an agent's only write tool writes
to a draft. A proposed draft is validated, and a bad one comes back with the
rule it broke, so the agent can correct it.

The synthesizer runs when a run has more than one reviewer. It waits until
every reviewer's first turn has ended, however it ended, then reads the same
change and every draft. Its drafts name their sources, and its closing reply
ends with a "Set aside" section giving each draft it did not carry forward
and why. The person works from the synthesizer's drafts once there are any.
A merged draft whose sources all come from one reviewer is labelled as
raised by one reviewer. A preset's `synthesizer` names its tool and model,
and the default is the first reviewer's tool on its default model.

Every run ends with a verifier (D31). It waits until every reviewer and any
synthesizer has reported, then gets the findings the person works from, by
id, with the change. It reads the worktree as a reviewer does, and marks each
finding confirmed, plausible, or doubtful (✅ ❔ ⛈), with a one-line reason,
through `rate_draft`. That is the only thing it can write: it cannot propose,
revise, or withdraw. When its turn ends with findings unmarked, vuoro sends it
back once with their ids. The person can set their own mark on any finding,
which wins, and the verifier's stays beside it. A preset's `verifier` names
its tool and model, and the default is the synthesizer's, or else the first
reviewer's.

A reviewer can draft a suggestion (D31). On a finding on head-side lines that
the person has not yet decided, "Ask for a suggestion" sends the session that
drafted it a request to revise it with `suggestion`: the exact text that
replaces the anchor's lines. Validation refuses a suggestion without a
head-side anchor (`suggestion.anchor`). Keeping the finding puts a GitLab
```` ```suggestion:-N+0 ```` block in the comment, reaching back from the
anchor's last line, where the comment lands, over the whole range, so the
author can apply it. Every fence is longer than any run of backticks inside
it. This is text a read-only reviewer writes, not the editor's patch (D17).

The kept findings make a hand-off report (D31): Markdown for an agent that
fixes them, giving the change, the reviewed head, and for each kept finding
its severity, place, standing confidence mark, description, and evidence. One
button writes it to `runs/<id>/report.md` and copies it, and when the
clipboard refuses, the text is shown to copy by hand. Nothing writes into the
person's checkout.

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

**The drafts channel**, settled by TKT-01M3YMSSRC81M0BYHK15TW9VDY (Serve the
drafts MCP server to each agent session) and held in `review/drafts`:

- It is MCP over Streamable HTTP, at `/mcp/` on the loopback server, behind
  the same guard as every route. It answers each request with one JSON
  response and opens no event stream.
- Each session registers with the run and gets a bearer token of its own,
  which says which session it is and whether it is a reviewer, the
  synthesizer, or the verifier. The token reaches Claude Code in the MCP config file of mode
  0600, never in `argv`.
- Its tools depend on the role, and none reaches a forge. A reviewer and the
  synthesizer have `list_drafts`, `propose_draft`, `revise_draft`, and
  `withdraw_draft`. A reviewer lists and changes only its own drafts. The
  synthesizer lists every draft, and each draft it proposes names its
  sources. The verifier has only `list_drafts` and `rate_draft`, and the
  driver's allowlist is built from the same per-role list, which a test
  holds.
- A draft has a severity, a title, a description, optional evidence, an
  optional anchor (path, side, start and end line), and, from the synthesizer,
  sources. These follow terva-action-code-review's review-result.v1. A
  proposal that breaks a rule comes back as a tool error that names the rule
  and the field, such as `rule anchor.range at anchor/end_line`, and never
  quotes the proposal. A withdrawn draft stays in the store, marked, so the
  person can see what was set aside.

**What a run records:** the tool, its version, the model, and the account kind.
For Claude Code, the account kind is `apiKeySource` from `init`, plus
`authMethod` and `subscriptionType` from `claude auth status`. vuoro reads only
those fields, because the same output carries the email address. For terva,
`hello` gives `version`, `protocol_version`, `provider`, and `model`.

**A run outlives vuoro.** Its record, `runs/<id>/run.json`, holds the
sessions' transcripts, the drafts, each session's resume handle, and the
diff's anchor index. vuoro reads every record when it starts. A session comes
back ended, and the person's next message resumes it in the same
conversation. When vuoro exits, it ends every live session first, so no
agent outlives it.

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

The format is JSON files, decided on 2026-10-02 (D29). One person's inbox
and runs need no queries that justify a database. `modernc.org/sqlite`
remains the cgo-free option if that changes. The state directory holds:

| Path | Holds | Kind |
|---|---|---|
| `cache/` | forge data: changes, threads, diffs | derived. Deleting it loses nothing the person wrote |
| `annotations/` | the person's snoozes and notes | authored |
| `runs/<id>/` | a review run: its sessions, transcripts, and drafts | vuoro's own record, under the retention above |
| `repos/` | a bare clone of each reviewed project, fetched over the person's own git transport | derived |
| `worktrees/<run>/` | the checkout a run's reviewers read | derived, removed with the run |

`cache/inbox/<account-hash>.json` holds each account’s derived snapshot and
last-attempt/last-success/error metadata. Its key includes kind, host and
username. Only configured accounts are loaded. `annotations/inbox.json` holds
snoozes by the existing stable change key; cache replacement or deletion never
touches it. In-memory views are independent copies.

Files are written whole, to a temporary name and then renamed, so a crash
leaves the old file or the new one and never half of either. Directories are
mode 0700, and files mode 0600.

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
[roadmap.md](roadmap.md) maps the current implementation, execution slices and
completion gates. D34 selects one GitLab-and-Forgejo inbox as the next outcome;
implementation tickets stay in draft until selected.

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
   submit the review. Forgejo open-by-URL reading, checkout and agent reviews also
   land here (D33); its posting controls remain unavailable. Presets and profiles live in vuoro's own config, and code,
   security, and documentation profiles are built in. Cost is shown, and runs
   are pinned to their head SHA and flagged when it moves.
3. **The shared inbox, next (D34).** Discover the open GitLab changes and
   Forgejo PRs I review, wrote or am assigned to through one discovery/refresh
   boundary. Derive whose turn it is with reasons and the earliest unanswered
   age, and open any row in the existing orchestrator. It refreshes only while
   the window is open. Account failures retain visible stale results rather
   than hiding work; snoozes stay separate from the derived cache. Read-only
   discovery does not wait for GitLab live-write validation.
4. **The author's side.** Triage the threads on my change, and draft replies.
5. **git-ticket, read-only.** Show a change's linked ticket, and give its
   acceptance criteria to the reviewers.
6. **History.** Keep runs until the change closes, then 30 days, then one line
   per run.

### 10.2 v1.0.0

TKT-01M3YMSTYMFRDFXD4QSVWHMM8P (vuoro v1.0.0: every forge, the editor agent,
and a terva-sh release).

- **Every forge.** Complete Forgejo writes after the MVP read/review and inbox
  slices, then add GitHub through the same contracts.
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
