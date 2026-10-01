# vuoro plan

This is the design of record. Read it before you change anything, and change it
in the same pull request as code that moves the design. Open questions are
marked as such and each one has a ticket. Do not settle one in code without
closing its ticket.

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
approve, and submit, without opening the forge's web interface. It is also where
agentic help attaches: an agent can summarise a change, draft a review, or draft
a reply. The person edits the draft and decides whether to post it.

### 2.1 Positions

- **Turn is derived, and it shows its reasons.** "Your turn: Mason replied on
  two threads, 3 days ago" is the product. A bare "needs attention" badge is
  not.
- **An agent drafts. A person posts.** No agent output reaches a forge without
  an explicit action by the person in the UI. Agent output lands as pending
  draft comments that look and edit like the person's own, marked as drafted.
- **The forge is the record.** vuoro keeps a cache and a person's private
  annotations, such as a snooze or a note. It never holds review state that
  the forge does not also hold. Deleting vuoro's state loses the annotations
  and nothing else.
- **Link out for what vuoro does not do.** Pipelines, merge trains, and
  settings stay in the forge. Every item carries its web URL.

### 2.2 Out of scope for v0.1

- Serving other people. vuoro is one person on one machine. Unlike
  git-ticket-canvas, it has no server mode.
- Notifications, tray, and autostart. tuohi has all three, and they come after
  the inbox is trustworthy.
- Writing to a forge in anybody's name but the person running vuoro.

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
| `assist` | The terva integration (section 6). The only package that imports terva. |
| `tickets` | The optional git-ticket integration (section 7). The only package that imports git-ticket. |
| `web` | Frontend source and the committed `web/dist`. |

`assist` and `tickets` are kept apart so the inbox builds and tests without
either of them. A test holds that boundary, in the same way as
`terva/examples/harness/imports_test.go`.

### 3.2 Pins

| Dependency | Pin at bootstrap | Note |
|---|---|---|
| Go | 1.27 | terva's `go.mod` declares 1.27.0, so a module that imports terva must too. |
| `terva.sh/terva` | v0.139.5 | Pre-1.0. A minor release can break the stable packages, with a note in `docs/migrating.md`. |
| `github.com/terva-sh/tuohi` | v0.1.0-alpha.1 | Pre-release, and its README says to expect API changes. |
| `github.com/terva-sh/design` | v0.1.0 | Has no `vuoro` accent yet. |
| `github.com/terva-sh/git-ticket` | v0.23.0 | Library and CLI. |

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
  worth it for v0.1.

### 5.2 The loopback server is local attack surface

Every process on the machine can reach `127.0.0.1`. The server carries the
person's forge tokens, so it must refuse a request that does not prove it came
from vuoro's own window: a per-launch secret, an origin check, or both. tuohi's
trusted-origin bindings are part of the answer. TKT-01M3W4HWDY8XV92MXXQWA0AES7
(Refuse loopback API requests that did not come from vuoro's window) settles
it, and it must land before any forge write exists.

## 6. Agents (terva)

> Open question: TKT-01M3W4HWW7FHHC6VD5M0S9MV90 (Decide how vuoro embeds terva
> for agent actions).

terva offers two in-process entry points (`terva/docs/embedding.md`):

- **The SDK**, `packages/agent/sdk`. It reads the person's `$TERVA_HOME`
  config, credentials, and permission rules. It needs little code, and it
  shares terva's model setup.
- **The engine**, `packages/core` plus `packages/provider`. It reads nothing.
  vuoro passes the client, the model, the tools, and a permission gate. It is
  more code, and vuoro gets full control.

Both are in `.api/packages.txt` and count as stable. Import nothing outside that
list.

The first actions, in order of value:

1. **Summarise a change**, including what changed since my last review.
2. **Draft a review** into pending draft comments. terva's `review-crew`
   personas, such as Vartija for security, are the starting point.
3. **Draft a reply** to a thread.

Tools an agent gets are read-only over the change: the diff, the threads, and
files at the head SHA through the forge API or a local checkout. An agent gets
no tool that writes to a forge. Posting is the person's action (section 2.1).

`terva-sh/terva-action-code-review` already runs terva as a pull-request
reviewer in Forgejo CI. Read it before designing the review prompt. It is
private, so the bootstrap session could not read it.

The model provider is the person's choice, through terva's configuration.
vuoro never picks one, and it never sends a change's content anywhere the person
did not configure.

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
- `go test -race ./...`
- cross builds for linux, darwin, and windows
- the frontend typecheck and tests, and a check that `web/dist` matches source
- `design-tokens export -check`
- `git ticket check --fix --dry-run --strict`
