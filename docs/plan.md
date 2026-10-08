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
vuoro review     runs one review headless and prints its findings (section 6.7)
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
frontend assets. The stylesheet follows the system's colour scheme unless the
root element names one in `data-scheme`. The top bar offers System, Light,
and Dark, and the choice is kept in `prefs.json` in the state directory,
because the loopback port, and so the window's local storage, changes every
launch.
Diffs take their colours from the shared status roles, `--ui-ok` for an
added line and `--ui-danger` for a removed one (D46). Each row also carries
its sign and a gutter bar, and each file its added and removed counts, so the
colour is never the only signal.

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
| `local` | The Local tab's reading of the person's checkouts: added projects and scan roots, each project's branches beyond its base with their upstreams, its worktrees, and its remotes (section 6.8). It reads only, and imports no forge, agent, review runner, API, or network. |
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

`github.com/godbus/dbus/v5` is a direct dependency, at the version tuohi pins,
for one call: asking the session bus whether a tray host runs (section 3.3).

### 3.3 The tray and nudges

TKT-01M3YMSVBDTFZ17PS86GXAC4ZK (Tray, background refresh, and quiet turn
notifications) carries out D24 with the owner's choices in D55. Each part is
the platform's own, through tuohi, without cgo.

- **One vuoro per person.** A window launch takes tuohi's single-instance
  lock. A later launch hands it nothing but the request, and the running one
  opens its window. Forwarded arguments are untrusted, so none are read.
- **The tray.** Where the desktop shows a tray icon, closing the window leaves
  vuoro running. Its menu holds Open vuoro, Refresh now, and Quit vuoro.
  macOS and Windows always have a tray. On Linux and the BSDs the icon is a
  StatusNotifierItem, and tuohi's tray starts without error even when no
  StatusNotifierWatcher runs, so the icon never shows. vuoro therefore asks
  the session bus itself, with `NameHasOwner`. Where there is no watcher, or
  the bus cannot say, closing the window quits as before, so the window is
  never lost. Whether closing hides or quits is fixed when the first window
  opens, so on Linux and the BSDs the tray, which needs no UI loop there, is
  set before it: if setting it fails, closing quits and vuoro does not start
  hidden. On macOS and Windows the tray needs tuohi's loop and is set as it
  starts, after closing is fixed to hide; a failure there, which takes a
  broken session, is reported and vuoro quits, rather than run on with no
  icon to quit it from. The toggles live in setup rather than in the tray menu, because
  tuohi's menu is fixed for its lifetime and a checkbox there would go stale.
- **Background refresh.** `App.Background` refreshes the inbox every
  `inbox.refresh_minutes`, 15 by default, whether the window is open or not.
  It reads the interval again after each wait, so a change in setup applies
  without a restart. The window's own refresh, once a minute while it shows
  the inbox, is unchanged.
- **Nudges.** After every inbox refresh, from the window or the background,
  the changes that are the person's turn are compared with the turns already
  told, which are kept in `nudges.json` in the state directory.
  - A change is told when it was not the person's turn before, or when its
    turn began again.
  - A refresh sends at most one notification, at low urgency. For one change
    it gives the change and the first line of why; for several, a count and
    the first three.
  - Until an inbox refresh has recorded the person's turns, a refresh only
    takes note, so a new install does not announce its whole inbox. A
    review's end, kept in the same file, does not count as that record. A
    launch at login tells what turned overnight.
  - With `inbox.notify` off, nothing is shown and what turned is still noted,
    so turning notifications on later announces nothing old.
  - A notification the desktop does not show is not recorded, so the next
    refresh, or a review's next finished turn, tries it again.
  - What was told is read from the file once and kept in memory, so a state
    directory that cannot be written repeats nothing while vuoro runs; the
    failure is logged.
- **A long review finishing.** `review.Manager.Finished` is told when a turn's
  end leaves a run idle with no session about to start, so not between the
  reviewers and the synthesizer, nor before a verifier is sent back. The app
  tells, once per run and across restarts, a review that took two minutes or
  more to complete. The turn that first completes the review decides: a short
  review is recorded untold, so a later conversation with one of its sessions
  cannot make it look long.
- **Start at login** is off until the person turns it on in setup. It
  registers `vuoro -background`, which starts with the window hidden. The
  first window is still made at launch, on the main thread, because on Linux
  that pins tuohi's UI thread, so a later launch can open the window from
  another goroutine.
- **The mark.** vuoro's mark is a white lowercase v on a violet tile, with an
  amber dot at the tip of its right arm: the turn that is the person's (D56).
  It is drawn at launch rather than shipped as a file, and it is the window's,
  the taskbar's, the dock's, the tray's, and the notifications' icon. Wayland
  has no per-window icon, so vuoro has tuohi write a hidden desktop entry and
  a themed copy of the mark under the person's data directory, which is how a
  GTK3 window there finds its icon. Windows notifications take no PNG from
  memory and use the stock information icon.

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
authorship times never stand in for it. Undated notes and human verdict history retain adapter chronological order;
fully dated histories sort by time. Repeated verdicts retain the first
unanswered age in their uninterrupted run, including an unknown age. Last activity can include forge noise,
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
writing capabilities when authenticated. Forgejo supplies drafts, reply, approve
and request changes when authenticated and given a draft store, and never
resolve, withdrawal or applicable suggestions (section 5.3, D43). An anonymous
client of either supplies none, so a URL read without a configured account
offers no posting. Agent findings remain local drafts.

The flags are `drafts`, `reply`, `resolve`, `approve`, `unapprove` (withdrawing
the person's own approval), `request_changes`, and `suggestions`. The last means
the forge applies a suggestion block; without it, a kept suggestion posts as a
plain fenced replacement under a line naming the lines it replaces (D43). The UI
offers a control only when its flag is set, and never decides what can be
written from the forge's kind. The application refuses a write whose flag is
unset with `forge.ErrUnsupported` before the adapter is called, so a hidden
control cannot be reached by a request either (D44).

`forge.Writer.Submit` takes the person's verdict: commented, approved, or
changes requested, the `forge.Verdict` values the readers already use. The
adapter publishes the pending comments and the verdict as one review. On GitLab,
changes requested is the bulk publish's `reviewer_state=requested_changes`, and
approved is the bulk publish followed by `/approve`. `Approve` remains for a
standalone approval or its withdrawal. The submit control offers comment always,
"Submit and approve" with `approve`, and "Request changes" with
`request_changes`. Requesting changes asks first, as submitting and approving
do, unless the person waived it.

`forge/adapters` owns the parser/factory registry. A configured host's kind wins;
unconfigured URLs use the GitLab `/-/merge_requests/`, Forgejo `/pulls/`, or
GitHub `/pull/` path shape, and github.com is GitHub's. GitLab and Forgejo read
anonymously; GitHub needs a configured account (D54, section 5.4). Each host has one configured account, and clients and credentials are
cached by host and kind. Application code consumes neutral errors and metadata;
it imports no individual adapter. A new forge extends the registry and contracts
without adding API details to the application or review runner.

Checkout metadata comes from the adapter: HTTPS and SSH clone URLs and the
change's head ref in its target repository. GitLab uses
`refs/merge-requests/N/head`; Forgejo and GitHub use `refs/pull/N/head`,
including fork heads. Advertised Forgejo SSH URLs retain explicit ports. The application
validates hosts, credentials in URLs, refs and branch names before fetching,
and git uses the person's own transport credentials. Run records retain forge
kind for display; existing GitLab records and Ref identity keys stay compatible.

Forgejo reads the PR, paginated file inventory, reviews and issue timeline.
Review comments use a single complete-list endpoint: Forgejo ignores page
parameters there. The returned count must match the review’s count. The separate `.diff` response supplies patches. Binary,
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

Forgejo discovery verifies `/user` against the configured account and pages
`/repos/issues/search?state=open&type=pulls` with `created`, `review_requested`
and `assigned` filters. It also examines `reviewed` candidates to retain a
requested-reviewer relationship after a verdict, when a later push may need
another review. Recorded request/removal timeline entries and `REQUEST_REVIEW`
records establish the relationship; voluntary reviews alone do not. Request and
removal events are ordered across both sources per reviewer. Forgejo also
lists in `requested_reviewers` a reviewer whose latest review approved or
requested changes, with no request on record (verified live on 15.0.3, while
TKT-01M3W4HX4CJE62G9R124EV5PT4 was being built). So a listed reviewer with no request record, whose
undismissed verdict on today's head stands, has answered; a real request
always leaves a `REQUEST_REVIEW` record or a timeline event. Undated events
retain an uncertain relationship rather than silently hiding work. Current PR
requests take precedence. All roles deduplicate by the existing change key.
Patch-free fact reads omit pending private reviews. Published review histories
are ordered per reviewer independently of drafts and request records; an
undated published verdict preserves that reviewer’s adapter order. A later dismissed or stale
non-comment verdict supersedes an earlier verdict without reviving its effect.
Signed-in merge permission remains unknown: repository write/admin permission
alone does not prove permission to merge this PR. Paging rejects malformed responses (including null search/review lists)
and moving head, target, state or update stamps. Forgejo’s verified empty
timeline representation, JSON null, is accepted only for timeline pages.

The current Forgejo API supplies no bot-user marker. Vuoro does not infer bot
status from account names. Each configured forge may name automated accounts
in `bots`; the inbox applies those names and native bot markers to users and
reviews before turn derivation. GitLab marks every account whose `type` is
not `HUMAN` as automated, including the ghost user that holds deleted
accounts' notes. Configured marks are applied to copies of raw
cached facts, so removing a configured bot does not leave a permanent mark.
If a forge does not fit the contracts, revise the model rather than leaking its
API details through the application. Agent and ticket integrations keep their
own contracts (D33).

`forge.Discoverer` is optional beside `Provider`. It returns complete open
membership for a username, with all roles and forge update stamps; pagination
and API details stay in adapters. Complete membership makes removals safe.
The `inbox` service deduplicates stable `Ref.String()` identities, merges roles,
and uses optional `forge.FactsReader` for fresh turn facts on each refresh,
without downloading patches or personal drafts. Permissions and merge readiness
can change without a membership update stamp. For providers without that
contract, it reuses a cached change only when its update stamp matches a nonzero
membership stamp. Missing stamps always cause a new read. Discovery must
succeed before an account snapshot is replaced. Reading a change may fail on
its own (D36): a change read before keeps its last facts with a fixed reason,
one never read is listed by reference under "could not read", and the account
carries a warning naming how many. A change that closes between discovery and
reading drops out of the snapshot. Reads of one account run six at a time.

Refresh runs only when called by the window, with at most four accounts in
parallel and a two-minute context per account. Calls are serialized. Failed,
cancelled, timed-out and rate-limited accounts retain their items and last
success, and expose fixed credential-free messages. Healthy accounts replace
their own snapshots independently. The service launches no background poller.

The home window reads a lightweight inbox projection: account refresh status
and `mine`, `theirs`, `dormant` and `snoozed` rows with forge/project, roles,
structured reasons and nullable ages. Each row also carries its lane, a
summary with one line per kind of reason ("8 threads await your answer:
ehlertjd (5), naterichman (3)"), the threads behind the reasons (author,
place, the first line of the last real note), and its last human activity.
Each group sorts by oldest known unanswered age, then unknown ages, with
stable change keys for ties.

The inbox view follows D38. A header line holds each account's refresh state.
Beneath it a summary strip counts what needs the person: changes requested on
their changes (A2), ready to merge (A3), replies owed (A1, R3), and reviews
requested (R1, R2, R2b). Each count filters the list. "Your turn" splits into
lanes, their changes, then replies owed, then reviews, and a row sits in the
lane of its most pressing reason. Rows are two lines: an age badge (blue under
two days, amber under two weeks, magenta beyond, always with the number), the
title and a short reference, then the role and the summary. "Their turn",
"Dormant" and "Snoozed" fold until opened. A change with no human activity for
`inbox.dormant_days` (default 90) is Dormant whoever's turn it is. Bots,
system notes, and the forge's updated stamp do not count as activity. The
selected row fills a detail pane with its actions, its summary, and its
waiting threads. Snooze offers until tomorrow morning, a week, and until it
changes, which the server keeps as 9999-12-31 so that only the event key ends
it. Keys: j/k move, Enter opens in vuoro, o opens on the forge, s snoozes
until tomorrow, r refreshes, and / focuses the URL field. Patches and private drafts
are omitted. `/api/inbox`, `/api/inbox/refresh` and `/api/inbox/snooze` use the
existing window guard and require no agent probing or ticket store. A bad config
or unreadable cache is a visible failure, never an apparently empty inbox.

The inbox polls only while its home view is mounted, a minute after the previous
request completes; manual refresh and snooze actions share one frontend request
gate. Unmount aborts active requests and cancels the timer. Per-account failures
show cached results and the last successful refresh. Rows open the existing
workspace or link to the forge. A separate snoozed list restores a row explicitly.
Snoozes retain a fingerprint of raw forge facts, excluding patches, private drafts
and capabilities. Expiry or changed facts wakes the row, including undated new
notes; config-driven bot classification never changes that fingerprint.

After a human write, a targeted fresh read updates the corresponding cached row
and turn (or removes a closed change). It takes the same gate as full refresh
before reading, so an older in-flight refresh cannot overwrite the new facts.
It preserves the whole-account last-success time; only a full successful account
refresh advances that stamp. No posted state is guessed from the write response.

GitLab notes for the adapter:

- Discover complete open membership with `GET /merge_requests?scope=all&state=opened`
  using `reviewer_username=`, `author_username=` and `assignee_username[]=`.
  Verify the authenticated username, deduplicate roles and paginate within the
  configured host. Delta-only `updated_after` queries cannot safely remove closed
  changes or removed relationships, so membership is read completely. Missing
  paging headers require reading through an empty page; malformed or partial
  paging fails the account refresh.
- Inbox fact reads use GraphQL, including the signed-in person’s
  `userPermissions.canMerge`, without patches or private draft notes. Opening a
  workspace still reads both. Discussion cursors are bounded and checked for
  progress and consistent head/update stamps. Nested notes, reviewers, assignees,
  participants and diff versions request 100 entries and reject `hasNextPage`
  instead of deriving turns from truncated facts. System review events are read
  from mixed as well as system-only threads, chronologically and with bot marks.
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
`~/.config/vuoro` on Linux, and `%AppData%\vuoro` on Windows. The window's
setup writes it (D45): it adds, edits, and removes forges, tests each token
source against the forge, and refuses to save over a hand edit. A fresh install
walks through it one step at a time: the forge, chosen from GitHub, GitLab,
Codeberg, or the person's own; its token, made on the forge's own page, kept in
a store this machine has, and checked; the agent tools; and the background
settings. A forge is saved when its token step ends, so leaving setup early
keeps it. A person can still
write it by hand. An example, with no
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

### 5.3 Forgejo writes

TKT-01M47EG9BHW0HBVFBTSBB1915X (Prove which Forgejo review writes round-trip
against a sandbox PR) exercised every write on the internal instance, Forgejo
15.0.3 (gitea-1.22 API), on 2026-10-06. It used a private sandbox PR with an
author account and a reviewer account. `forge/forgejo/testdata/writes/spike.json`
holds the scrubbed request and response for each case. The writer and the
stacker build on what follows.

**Do not keep a pending review on Forgejo.** A pending review is hidden from the
reviews list and the comment endpoints, but the issue timeline shows its body
to every reader of the repository, once each time the body is set. A body that
is later replaced stays in the timeline after submit. A pending review also
needs a non-empty body. So vuoro keeps Forgejo drafts local until Submit. Submit
then creates and publishes the review in one `POST /pulls/{n}/reviews` call,
carrying the verdict, the body, and every inline comment. That call is the one
event that `forge.Writer.Submit` promises. If the person already has a pending
review, perhaps started in the web UI, the call merges into it, publishes its
comments, and replaces its body. vuoro shows those comments as part of what
Submit will publish, and keeps the old body unless the person changes it.

| Operation | Forgejo route | Notes |
|---|---|---|
| Inline draft | Local until Submit, then `comments[]` on the review | Forgejo does not check that a line exists, so vuoro's anchor check is the guard. |
| General draft | Local until Submit, then the review body | A review has one body, so several general drafts are joined in order. A comment with no path is accepted, but it publishes as a malformed code comment, so vuoro never sends one. |
| Edit, delete a draft | Local | A comment already pending on the forge can be edited with `PATCH /issues/comments/{id}`, which keeps its id, and deleted with `DELETE /reviews/{id}/comments/{c}`. |
| Reply | `POST /pulls/{n}/reviews/{thread's review}/comments` at the thread's path and position | It publishes at once and joins that conversation. A comment at the same line in a new review starts a separate conversation instead. A reply is sent at Submit, after the review, as a separate write. |
| Comment | `event: COMMENT` | Needs a body or at least one inline comment. |
| Approve | `event: APPROVED` | May have an empty body. The author cannot approve their own PR. |
| Request changes | `event: REQUEST_CHANGES` | Needs a body. The author cannot request changes on their own PR. |
| Withdraw an approval | None | A reviewer cannot dismiss their own approval. Dismissal is a repository admin's action on someone else's review. Deleting one's own published review works, but it erases the record. Withdrawal stays unsupported. |
| Resolve, unresolve | None | The API has no route. Resolving stays hidden. |
| Suggestion | None | Forgejo renders a `suggestion` block as plain code, and it cannot be applied. A Forgejo suggestion posts as an ordinary fenced replacement under a line saying so. |
| Stacked draft PR | `POST /pulls` with a `WIP:` title prefix | Both `WIP:` and `[WIP]` give `draft: true`. |
| Fork check | `head.repo.full_name` against `base.repo.full_name` | A fork PR also has `head.repo.fork: true`. |

A later verdict from the same reviewer marks their earlier non-comment verdict
dismissed. The inbox reader already treats that as superseded.

### 5.4 GitHub

TKT-01M3W4HX30MB0D601R3S0ZXSER (GitHub adapter for the forge interface) adds
`forge/github`, in slices: the reader and discovery, then the writer, then the
stacker. D54 records the owner's choices.

- **The API is found from the web host.** A forge is configured by the host
  people browse, `https://github.com`, which a Ref names. Its REST API is at
  `api.github.com` and its GraphQL at `api.github.com/graphql`. Any other host
  is GitHub Enterprise Server, with `/api/v3` and `/api/graphql` on the host.
- **Reading needs a token.** GitHub answers anonymous GraphQL with 403, and
  REST has no thread resolution, which turn rule A1 needs. So a GitHub change
  opens only through a configured account. `Open` and `ReadFacts` send one
  GraphQL query for the pull request, its review requests, reviews, review
  threads with `isResolved`, issue comments, assignees, and the timeline's
  review requests, dismissals and force pushes. A list past a hundred is paged
  by cursor, and a pull request that moves between pages is refused. `Open`
  adds the patches from REST `/pulls/{n}/files` and the merge base from
  `/compare`, then checks that the pull request has not moved.
- **What GitHub's model gives the turn rules.** GitHub lists a reviewer in
  `reviewRequests` only while the request is unanswered, so a listed reviewer
  is unreviewed. A dismissed review's verdict comes from its
  `ReviewDismissedEvent`. GitHub records a time for a force push and for no
  other push, so R2 compares the review's commit with the head. A team request
  names nobody, and is skipped. A pending review is the viewer's draft and
  stays out of the facts.
- **A thread whose lines left the diff** has no current line. It is anchored at
  its original line on its first comment's original commit.
- **Discovery** runs GraphQL search for open pull requests with `author:`,
  `review-requested:`, `reviewed-by:` and `assignee:` the person. `reviewed-by`
  keeps a pull request whose request GitHub dropped when the person answered
  it. A search over GitHub's thousand-result cap is refused, not read in part.
- **Drafts are a GitHub pending review.** GitHub keeps a pending review private
  to its author, unlike Forgejo, so the reason for D43 does not apply. Its
  writes are GraphQL.

#### GitHub writes

A spike on a private sandbox pull request, on 2026-10-08, settled how each
write behaves. `forge/github/write_live_test.go` repeats it.

**Never send a comment without the pending review's id.** A reply or an inline
comment that names no pending review is published at once, and a reply joins
an existing pending review only by chance. So every comment names the person's
pending review, which vuoro starts at the head when there is none, and a write
that comes back published is reported as published. A write naming a review
that was already submitted answers null and publishes nothing.

| Operation | GitHub route | Notes |
|---|---|---|
| Pending review | `addPullRequestReview` with `commitOID` and no event | One per person per pull request: a second is refused, so vuoro uses the one the person started on GitHub. |
| Inline draft | `addPullRequestReviewThread` with the review's id, a line and a side | A line outside the diff answers a null thread, with no error. A removed line is `LEFT`. A comment on an earlier head is refused before any write, and so is one into a pending review started on an earlier commit, since GitHub reads the line against the review's commit. |
| Reply in a review thread | `addPullRequestReviewThreadReply` with the review's id | Pending until Submit. |
| Comment on the whole change, or a reply to one | Kept in vuoro until Submit | A pending review's body cannot be changed once the review exists. Submit sends the comments, joined, as the review's body, after any body started on GitHub, which vuoro shows and cannot edit. A reply to a general comment is posted with `addComment` after the review, as Forgejo's replies are. |
| Edit, delete a draft | `updatePullRequestReviewComment`, `deletePullRequestReviewComment` | Both act on published comments too, so the comment must be in the person's pending review. GitHub has no conditional write, so a review submitted on GitHub in the moment between that check and the write takes the edit or the delete; both answer the comment's state, and vuoro reports it as a change to a published comment. Deleting the last comment of a pending review with no body removes the review. |
| Submit | `submitPullRequestReview` with the event and the body, or `addPullRequestReview` with them when only a verdict is given | An empty review is refused. |
| Approve, request changes | The events `APPROVE` and `REQUEST_CHANGES` | The author can do neither. Approve on its own refuses while pending comments would publish with it. |
| Withdraw an approval | None | Only a dismissal, by someone with write access, takes one back. |
| Resolve, unresolve | `resolveReviewThread`, `unresolveReviewThread` | Review threads only. |
| Suggestion | Off | GitHub applies a `suggestion` block over a line range, and vuoro's anchor holds one line. A suggestion posts as a plain fenced replacement until the anchor can hold a range. |
| Stacked draft PR | `POST /repos/{o}/{r}/pulls` with `draft: true` | A repository whose plan has no drafts answers 422 naming drafts; vuoro then opens an ordinary pull request and reports it as not a draft. A new pull request whose head commit is already in its base is marked merged at once, so the editor's branch must hold commits the target lacks. |
| Fork check | `isCrossRepository` and `headRepository.nameWithOwner` | A pull request from a deleted fork has no head repository and is refused as a fork. |

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
kept or dropped draft can no longer be changed by its agent. While a Keep is
writing to the forge, the store holds the draft: a second Keep or a drop is
refused, an agent cannot revise it, and the button reads "Keeping…", so a
quick second click never writes the comment twice. A write's reply does not
wait for the inbox, which catches up in the background. The GitLab adapter
keeps the patches of the last few changes it read, by diff version, because
reading them is most of the cost of opening a large change, and a Keep
opens the change again to place the comment on today's head. An inline
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

Every review run that drafts new findings ends with a verifier (D31). It waits
until every reviewer and any
synthesizer has reported, then gets the findings the person works from, by
id, with the change. It reads the worktree as a reviewer does, and marks each
finding confirmed, plausible, or doubtful (✅ ❔ ⛈), with a one-line reason,
through `rate_draft`. That is the only thing it can write: it cannot propose,
revise, or withdraw. When its turn ends with findings unmarked, vuoro sends it
back once with their ids. The person can set their own mark on any finding,
which wins, and the verifier's stays beside it. A preset's `verifier` names
its tool and model, and the default is the synthesizer's, or else the first
reviewer's.

A reviewer drafts a suggestion with the finding (D31, D37). On a critical,
high, or medium finding whose fix is a replacement of the anchored head lines,
about 20 at most, it sets `suggestion`: the exact text that replaces the
anchor's lines. It sets none on low or info findings. The synthesizer carries
a source's suggestion only when its anchor covers exactly those lines, and the
verifier's reason ends with "suggestion: …" when the suggestion would not fix
the finding. Validation refuses a suggestion without a head-side anchor
(`suggestion.anchor`), and one whose first or last line repeats the
non-blank line just outside its range at the head (`suggestion.boundary`),
because applying it would leave that line twice. The first live run drafted
two suggestions whose ranges stopped one line short of a closing brace.

On a finding on head-side lines that the person has not yet decided and that
has none, "Ask for a suggestion" starts a fresh session on the drafting
session's tool and model. Its first message is the finding and its place, with
no diff and no transcript, and it reads the file from the worktree. Its drafts
role, Suggester, lists two tools: `read_draft`, which returns its one draft,
and `set_suggestion`, which sets that draft's suggestion and may widen its
anchor within the same file. It cannot propose, revise, withdraw, or rate. The
session shows in the run as "suggestion d<n>", and it does not count toward
the review being complete.

Keeping the finding puts a GitLab
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

The protected head-comparison route fresh-reads the exact change and fetches
its current refs through the person's git transport. An ancestry check permits
a count of commits reachable from the current head but not the reviewed head.
Rewritten history is labelled separately. Missing or shallow history, failed
fetches and cancellation show an unavailable count, never a guessed number.
This status is derived for an exact pair of commits, not persisted as review
state. Comparing history never moves a run's worktree or starts an agent.

The same reviewed-head badge appears on the run and each finding/reply, including
source and withdrawn drafts. Drafts from an older head are shown separately from
the current diff with their original locations; no re-anchoring happens. Stale
anchored findings cannot be kept, and the server independently checks fresh
change identity and head before creating a pending comment. General findings
remain explicitly actionable with their old-head warning. Head-comparison reads
abort on leaving the workspace and cannot overwrite a newer head's status.

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
- It runs in a sandbox (D40). By default it runs on the local machine as
  Claude Code, with the person's login. `--restricted` confines its file
  tools to the worktree. Claude Code's sandbox confines its shell commands,
  with no home-directory reads or writes, no unsandboxed retry, and the
  network limited to package registries. Its environment is built from an
  allowlist and holds no forge credential. When the person configures an API
  key for the editor, the sandbox runtime (`srt`) wraps the whole process
  instead. docs/editor-sandbox.md has the spike's probes.
- It commits on a local branch, one commit per finding or thread it addresses.
  The person reviews those commits as a diff, and then accepts them, which
  makes vuoro push to the merge request branch, or discards them. The editor
  never holds a forge credential and never pushes.

**Fixing a reviewed change in a stacked merge request** (D39) comes first.
After a review of somebody else's change, the kept findings, which the
hand-off report lists, become the editor's task:

1. "Fix these in a branch" is offered on a run whose reviewed head is still
   the change's head, and which has at least one kept finding. A moved
   change is reviewed again first, as Keep requires.
2. vuoro makes a fresh repository at the reviewed head on a local branch
   `vuoro/fix-<number>-<run>`, and starts one editor session in the sandbox
   (D40) with the report as its first message. The repository is seeded by
   a shallow fetch from vuoro's clone and shares no objects or
   configuration with it, and vuoro never runs git inside it: it reads the
   editor's commits back by fetching the branch into its clone, with the
   editor repository's hooks and fsmonitor turned off. The editor's commits
   carry the person's git identity, a `Vuoro-Finding` trailer naming the
   finding, and the caches its builds use live beside it. It may read, edit, build, and
   test inside the worktree. It commits once per finding, with a message
   naming the finding, and says which findings it left alone and why. It
   gets no forge credential and no forge connector.
3. The run shows the editor's commits as a diff, with each commit beside the
   finding it answers, and the person can talk to the editor as to any
   session. Discard deletes the branch and the worktree, and the forge never
   hears of them.
4. "Push and open a draft merge request" asks first, showing the branch, the
   target, and an editable title and description. The description lists
   each finding addressed, with its commit, and links the original merge
   request. On confirming, vuoro pushes the branch to the change's project
   with the person's credential, never in `argv`, and opens a Draft merge
   request onto the original's source branch. The author marks it ready and
   merges it, or does not. A change whose source branch lives in a fork is
   refused for now, with the reason.

   The push goes from vuoro's clone over the person's own git transport,
   the remote vuoro fetches from, so vuoro holds no git credential. It
   pushes the commit the person saw in the dialog and refuses a newer one,
   requires the editor's history to stand on the reviewed head, and never
   forces. The dialog flags a change to CI configuration, which runs when
   the branch lands, and a source branch that moved since the review. A
   later push of new editor commits updates the same merge request. The
   push route is the only caller of the push and of the merge request's
   creation, and a test holds that. On Forgejo the change is a draft pull
   request, marked by a `WIP:` title prefix, and the CI files flagged are
   `.forgejo/`, `.gitea/workflows/` and `.github/workflows/`. On GitHub it
   is a draft pull request, or an ordinary one where the repository's plan
   has no drafts, and the CI files flagged are `.github/workflows/` and
   `.github/actions/`. Forgejo's and GitHub's `TestLiveStack` exercise both
   this route and the own-branch push (D41).

**Fixing the person's own change from its threads** (D41). On the Threads
tab, the person picks triaged threads, must fix ones by default, and
"Fix the picked threads in a branch" starts the same editor with each
thread's discussion and the triage's reading of it. Its commits carry a
`Vuoro-Thread` trailer. "Push to your branch" pushes them to the merge
request's own source branch, fast-forward only, and refuses when that
branch moved since triage, other than by the editor's own last push. Each
thread a pushed commit names then has its reply cite the commit, unless the
person wrote the reply, and the agent can no longer revise it. Nothing is
resolved, and replies stay local until the person adds and submits them.

**Fixing the person's own work from its review** (D42). A review of the
person's own merge request, or of a local range from `vuoro review`, posts
nothing: Keep is refused there, and each finding is picked for the editor,
or dismissed locally. Findings above low severity that the verifier did not
doubt are picked by default. On a merge request, the fixes push to its
branch as D41's do. On a local range, which now opens on its own page, they
become a new branch `vuoro/fix-<run>` in the person's repository: vuoro
writes that one ref with `--force-with-lease` against an empty value or its
own last push, so a branch the person holds is never moved, runs their
receive-pack with hooks off, and touches nothing else there. A local run
records its checkout for this; runs from before that cannot be fixed and
are reviewed again.

**Fixing in waves, and reviewing again** (D50). A change is revisited, and its
findings are rarely all fixed at once, so a review keeps its findings and
decisions across several fixes and several reviews.

- *A fix records its commit.* A finding is fixed when a pushed commit names it
  in its `Vuoro-Finding` or `Vuoro-Thread` trailer, and vuoro records the
  commit on the finding (`FixedIn`), with its subject and whether the trailer
  or the person linked it. Before a push, vuoro checks each picked finding
  against the trailers, and asks the editor, once per finding, in the same
  session, to reword an unpushed commit to add the trailer or say why the
  finding is left alone; that push stops (409) until the person has read the
  answer and pushes again. The person
  can also mark a finding fixed by one of the commits vuoro pushed for the
  review or one of the change's since it was reviewed, which covers a fix made
  by hand, and take back only a link they made. Only vuoro and the person
  record a fix; no MCP tool can.
- *A fixed finding folds away.* The review page lists fixed findings in a
  folded "Fixed" group, each with "fixed in" and its commit, linked to the
  commit on the forge, and without the reviewed-head warning. The warning on
  the rest says when every commit since the review is one vuoro pushed for
  it, so vuoro's own fixes do not read as the change having moved.
- *A review takes several fix waves.* After an editor's work is pushed, the
  person can pick more findings and start another editor on the same review.
  It starts from the last pushed head, on the same branch, so a stacked change
  (D39) or a local fix branch (D42) is updated rather than opened again. A
  wave starts only while every commit since the review is vuoro's own push
  for it; once anyone else pushes, the review is out of date and the person
  reviews again first. The run keeps each wave, its findings, commits, and
  push, and only the newest can be pushed or discarded. Unpushed work blocks a
  new wave until it is pushed or discarded.
- *Reviewing again carries the earlier review.* "Review again" on a moved
  change starts a new run that follows the earlier one. Its reviewers are
  given the earlier findings with where each stands (fixed, dismissed, or
  open) and asked to report what is new or still unfixed. A draft that
  restates an earlier finding names it, and comes in with that finding's
  decision: one the person dismissed arrives dismissed, with the earlier
  dismissal shown. The run links to the run it follows and back, and its page
  shows the earlier findings in a folded group.
- *A fix keeps its patch* (D51). The commit a fix names is not durable on
  its own: closing the run drops the editor's repository and its refs in the
  cache, a pushed branch goes when it is merged and deleted, and a local fix
  branch goes when the person deletes it. So when vuoro pushes the editor's
  work, it saves each pushed commit as a patch, from `git format-patch`, in
  `runs/<id>/patches/<sha>.patch` beside the run record. A commit the person
  links with "Mark fixed…" is saved too, when vuoro's cache has it. A patch
  is read from the cache, with no forge call, and holds only the commit's
  message and diff. It lasts as long as the run record, past the run's close
  and the branch's deletion. A patch over the size cap is not saved; the
  run keeps the commit's list of files and says the patch was too large. The
  run lists the patches it saved, and the window shows each again in the
  diff viewer: from a commit of an earlier wave, from "fixed in" on a fixed
  finding, and from a fix branch on the Local tab. Work the person discards
  is deleted, as Discard says, and no patch of it is kept.

### 6.4 Other actions

Summarising a change, summarising what changed since my last review, and the
author's triage are reviewer-class work: the same drivers and the same
lockdown. Triage groups the open threads on the person's own change as must
fix, discuss, or nit, and drafts replies through the drafts server. None of it
is posted by an agent.

The change's Summary tab starts a separate, single-turn summary session with
Read/Grep/Glob and no drafts MCP server. Its text streams locally, with saved
history, exact range, head status and Stop. The whole-change range is the forge's
base to its freshly read head. “Since a Vuoro review” selects a completed local
review run and uses that immutable reviewed head to the freshly read head.
The latest eligible run is selected by default; the person can choose another.
This is not a claim about the last published forge review: GitLab's normalized
review facts have no reliable reviewed SHA, and timestamps cannot identify one.
Successful initial turns are recorded explicitly; legacy, failed or cancelled
records without completion evidence cannot supply a baseline.

Git verifies full commit IDs, complete history and ancestry before supplying a
commit list and exact net patch. A rewritten or missing baseline is refused;
it never silently substitutes a whole-change range. The patch is capped at
256 KiB and the commit list at 32 KiB; oversized input is refused before a
provider call, never truncated. Diff programs and textconv are disabled.
Descriptions/commit messages/repository text are untrusted data. The current
checkout supplies reading context only; a since-review summary describes just
its selected range. An empty net patch can still describe commits in that range.

Cancellation covers repository waiting, checkout, startup and generation. A late
agent start is stopped without receiving the request. Partial text survives Stop,
and the provider process ends; summaries cannot resume as reviewers or acquire
draft tools. Shutdown stops preparation as well as live calls. Summary runs stay
out of review/triage selectors, and the inbox still needs no agent tool.

The author's Threads tab and its interaction sketch are specified in
[author-triage.md](author-triage.md). One reviewer-class triage session captures
complete unresolved human discussions and writes category/reason/optional reply
through the local drafts MCP server. These discussion suggestions are separate
from D31's new-finding synthesis and verification pipeline. Each proposal is
bound to an allowed thread in the run; unknown threads, ordinary review roles,
retargeted replies and duplicate classifications are refused. Bot/system-only,
author-only and resolved threads do not become triage work.

Local editing and discarding require the window credential and never reach a
forge. Saving wording or category freezes that draft against agent revisions.
Acceptance is serialized against edits/revisions and creates only a private
pending comment through the existing human keep route. It re-reads the forge,
checks the exact change/head and complete human-thread fingerprint, and refuses
changed, resolved or closed work. Submit retains the existing confirmation.
Triage runs, thread snapshots, local replies and conversations survive restart.
Forgejo supports local triage/edit/copy; posting remains capability-gated.

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
  synthesizer, the verifier, or a suggestion session and its one draft. The token reaches Claude Code in the MCP config file of mode
  0600, never in `argv`.
- Its tools depend on the role, and none reaches a forge. A reviewer and the
  synthesizer have `list_drafts`, `propose_draft`, `revise_draft`, and
  `withdraw_draft`. A reviewer lists and changes only its own drafts. The
  synthesizer lists every draft, and each draft it proposes names its
  sources. The verifier has only `list_drafts` and `rate_draft`. A
  suggestion session has only `read_draft` and `set_suggestion`, for its one
  draft (D37). The driver's allowlist is built from the same per-role list, which a test
  holds.
- A draft has a severity, a title, a description, optional evidence, an
  optional anchor (path, side, start and end line), an optional suggestion
  on head-side lines, from the synthesizer, sources, and, in a review that
  follows another, the id of the earlier finding it restates (D50), which a
  synthesizer's draft takes from its sources. These follow terva-action-code-review's review-result.v1. A
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

### 6.7 Reviews from the command line

Decided on 2026-10-05 (D35), so that vuoro reviews its own development.

```text
vuoro review [-C dir] [-preset name | -profile name [-model m]] [-json] [-timeout d] [target]
```

- **The target** is a change URL, opened through the same adapters as the
  window, or a git range in the checkout at `-C`, which defaults to the
  working directory. `A..B` and `A...B` both review `B` against its merge base
  with `A`, as a merge request does. A single revision `A` means `A...HEAD`.
  No target means the checkout's upstream, then `origin/HEAD`, then
  `origin/main`, against `HEAD`.
- **A local range reviews commits only.** The runner fetches the range's
  commits from the checkout into its own bare clone, under the key
  `local/<name>-<hash of the absolute path>`, and checks out a worktree there
  as for a forge change. Uncommitted changes are not reviewed, and a dirty
  tree is reported on stderr. The change is forge-less: it has a title from
  the newest commit, the commit messages as its description, and per-file
  patches from `git diff` with external diff programs and textconv disabled.
  It has no threads.
- **The reviewers** come from `-preset`, or one reviewer from `-profile` and
  `-model`. The default preset is `quick` when the config file defines it,
  and otherwise the first preset, as in the window. Synthesis and
  verification follow the same rules as the window (D11, D31).
- **The output** is the findings report, as Markdown on stdout, with every
  pending finding the person would work from and its verifier mark. There is
  no keep or drop step in a terminal. `-json` prints one envelope with a
  `schemaVersion` and a `kind` of `review-result`. Progress goes to stderr.
- **Exit status** is 0 with no findings, 1 with findings, 2 for a usage
  error or a range with nothing to review, and 3 when the run or any of its
  sessions failed, or it timed out. The findings are printed in every case.
  An interrupt stops every session and exits 130. Flags may follow the
  target.
- **Nothing posts.** The command holds no forge writer, and a test runs it
  against a recording forge and refuses any write. Its drafts MCP server is
  a loopback server of its own, behind the guard of section 5.2, and each
  session has its own bearer credential as in the window.
- **The run is kept** under `runs/<id>/` with the same retention (section 8),
  closed, so its worktree is removed and its record stays. The window lists
  it after its next start, without a link when it has no forge page. A window that is already
  open does not see it. Two processes can fetch into the same bare clone of
  a forge project. git's own ref locks refuse the second fetch rather than
  corrupt the clone, and the command reports that.

### 6.8 Local projects in the window

Decided on 2026-10-06 (D47), so that a branch is reviewed before it is
pushed without leaving the window. Section 6.7's runner does the review. This
section is how the window finds the work and starts it.

**The home page has two tabs.** "Reviews" is the forge inbox of section 4.
"Local" is the person's checkouts. Each tab's label carries the count of what
needs the person there, and the last tab used is kept in `prefs.json`. The
inbox keeps polling behind the Local tab, so its count stays current. Local
reads the checkouts only once it is first shown; until then its count is the
one from the view it last built, kept in `cache/local.json`, and none at all
before the first.

**Projects come from two places.** The person adds a folder, through the
native directory chooser (tuohi's `dialog`, `TypeDirectory`) in the window, or
a typed absolute path in `-browser` mode. The person can also name a root,
such as `~/Workspace`, which vuoro scans for checkouts. The chooser is the
window's `vuoro.chooseFolder` binding, which only answers the chosen path; the
page adds it through the guarded API, which checks it as it checks a typed
one. The tab's Folders list shows what was added and scanned, each with
Remove, and the projects hidden, each with Show. The scan:

- runs when the Local tab opens with a stale list, and on Refresh, never in
  the background;
- descends at most four directories below a root, stops at a checkout rather
  than looking inside it, follows no symbolic link, and skips hidden
  directories and dependency trees (`node_modules`, `vendor`, `.venv`,
  `target`);
- stops at 500 checkouts or ten seconds, and says that it stopped.

A project is a repository, keyed by its absolute git common directory, so a
main checkout and its worktrees are one project however they were found. A
found project can be hidden, and an added one removed. Both stay in the state
directory, never in the config file.

**A row is a branch.** Each project lists its local branches that have commits
beyond the project's base, newest commit first, ten at a time. The base is the
remote's default branch (`origin/HEAD`), then `main`, then `master`, and the
person can change it for the project. A branch checked out in a worktree
carries that worktree as a tag, with its path and a mark when it has
uncommitted changes to tracked files. vuoro's own `vuoro/fix-*` branches are
listed under the branch whose run made them, not as rows of their own. A row
shows the branch, its commits ahead of the base and its newest commit's age,
its worktrees, and its last review: the run's status, whether the branch has
moved since, and its undecided findings.

**A branch needs the person** when it has never been reviewed, when it moved
since its last review, when its last review has undecided findings, or when an
editor's fix branch for it is waiting to be merged. The Local tab's count is
the number of such branches.

**A branch with no commit in 14 days is quiet** (D48). It is still a row, but
being unreviewed or having moved since its review no longer counts, so a
workspace's abandoned branches do not bury the active ones. Undecided findings
and a fix branch waiting to be merged count at any age, because the person
started them in vuoro and each clears when they act. The age is the branch
tip's committer date, so a rebase counts as a commit. The tab lists quiet
branches apart from the ones that need nothing.

**A branch knows its change** (D49). Each remote's URL, SSH or HTTPS, is
read as a host and a project path, and each branch's upstream as a remote and
a branch on it. A branch is tied to an open change when the change's host and
project are one of the checkout's remotes and its source branch is the
branch's upstream branch, on a remote of the same host, or the branch's own
name when it has none. The upstream's remote may be a fork of the change's
project, since a change from a fork names the project it goes into. The
changes come from the inbox's cache (section 4), so the tie costs no forge
call, and it finds every change the person authors, reviews, is assigned, or
takes part in. A change outside that set, such as someone else's branch
checked out to try, is not tied; asking the forge for it was rejected as one
call per pushed branch on every refresh. The `local` package reads remotes
and upstreams from git, and the application joins them to the cache, so
`local` still imports no forge.

A tied row names its change, as "!2 open" or "#14 open", which opens the
change's page as the inbox does, and says how the branch stands against the
change's head: in step, N commits not pushed, behind the change, or diverged.

A tied branch's review is the change's, which the Reviews tab already
counts, so Local counts it only for work the change does not hold: commits
not pushed, which the row names as "N commits not pushed to !2", and, as for
any branch, undecided findings and a fix branch waiting to be merged. Never
reviewed and moved since its review do not count for it. The quiet rule (D48)
still applies to commits not pushed.

**Review starts from the row.** A tied row offers two reviews side by side:
"Review the change", which opens the change's page, where findings become
drafts the person can post; and "Review local commits", below. An untied row
offers the second alone. Reviewing local commits runs section 6.7's local range,
`<base>...<branch>`, with the preset the person picks, and opens the run in
the window, where the person talks to the reviewers, picks findings, and hands
them to the editor as for any review of their own work (D42). It reviews
commits only. A worktree with uncommitted changes says that they are not in
the review. A branch reviewed before is reviewed again: the run follows the
branch's newest review (D50), so its reviewers are told the earlier findings
and the person's decisions carry over. The run stays open, as a window run
does, until the person closes it. The route, `POST /api/local/review`, takes only a branch the tab lists,
reads the project again so the base and tip are today's, and answers with the
run and the worktrees holding the branch with uncommitted changes. Refresh
picks up the CLI's runs once they are over (closed, failed, or cancelled); a
run another process is still writing is left to it, since two writers would
overwrite each other's record.

**Reading a checkout is read-only, and runs none of its code.** Listing uses
`for-each-ref`, `worktree list --porcelain`, and `status --porcelain
--untracked-files=no`, with hooks, fsmonitor, external diff programs, and
textconv disabled, as section 6.7's runner does. A path from the window must
be absolute and is cleaned before use. The routes sit behind the guard of
section 5.2 like every other. Nothing in a checkout changes except the fix
branch's ref, which D42 already allows.

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
| `local.json` | the person's added projects, scan roots, hidden projects, and each project's base (section 6.8) | authored |
| `cache/local.json` | the last scan and each project's branches and worktrees | derived |

`cache/inbox/<account-hash>.json` holds each account’s derived snapshot and
last-attempt/last-success/error metadata. Its key includes kind, host and
username. Only configured accounts are loaded. `annotations/inbox.json` holds
snoozes by the existing stable change key; cache replacement or deletion never
touches it. In-memory views are independent copies.

Files are written whole, to a temporary name and then renamed, so a crash
leaves the old file or the new one and never half of either. Directories are
mode 0700, and files mode 0600.

## 9. Gates

`just ci` is the gate, and CI runs the same steps, on linux on the internal
Forgejo runners only (D52). darwin and windows are vetted and cross-built
there, and tested only where someone runs `just ci` on one:

- `gofmt` and `go vet`, with `CGO_ENABLED=0`
- `go test ./...`, and again with `-race`. Off darwin the race detector
  needs cgo, so that run builds the tests with cgo (D53)
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
   land here (D33); its posting came later, with Forgejo parity (section 5.3). Presets and profiles live in vuoro's own config, and code,
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
7. **Reviews from the command line (D35).** `vuoro review` runs a preset on a
   local range or a change URL and prints its findings, so vuoro reviews its
   own development (section 6.7). It never posts.

### 10.2 v1.0.0

TKT-01M3YMSTYMFRDFXD4QSVWHMM8P (vuoro v1.0.0: every forge, the editor agent,
and a terva-sh release).

- **Every forge.** Complete Forgejo writes after the MVP read/review and inbox
  slices, then add GitHub through the same contracts.
- **terva as a reviewer**, once a terva release allows its lockdown (section
  6.6, D28).
- **The editor agent** (section 6.3), with its sandbox decided by a spike
  first. Fixing a reviewed change in a stacked draft merge request comes
  first (D39), then the author's side.
- **Reviews before a push.** Review a local branch or range in the window, and
  carry the findings into the merge request. The headless command arrives in
  the MVP (D35), and the window's Local tab finds and starts the work
  (section 6.8, D47).
- **Repositories add instructions.** A checklist and conventions, read at the
  base commit, layered on the person's preset. A repository never picks the
  reviewers.
- **Moving changes.** Re-anchor drafts on unchanged lines, and review only the
  new commits, with the earlier findings as context.
- **Cost.** Optional per-run budget caps, and a monthly total.
- **Nudges.** The tray, background refresh, and quiet, batched notifications
  when it becomes my turn or a long run finishes (D55, section 3.3).
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
