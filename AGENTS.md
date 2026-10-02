<!-- git-ticket:begin -->

## Tickets

Work is tracked as Markdown tickets in `.tickets/`, managed with `git ticket`.
`git ticket help` lists every command.

`git ticket instructions` prints the long form of this block, carrying the same
rules with the reason for each one and the failure it prevents. Read it when a
rule here surprises you, when you are about to work around one, or when you want
the parts this summary leaves out.

Name yourself on every command that writes, as `agent:tool/session`. Without it
the write is attributed to the first actor in `config.yml`, usually a person, and
your claim then tells other agents that a human is holding the ticket.

```sh
git ticket note TKT-01ABCD "..." --actor agent:yourtool/session-3
```

### Finding work

`git ticket ready` is the queue: open, unblocked, every dependency closed. It
and `list` both take `--label` to narrow and `--not-label` to exclude, which is
how you ask for the queue minus work this project has labelled as not startable
today.

`git ticket list --status draft` is the rest of the backlog and is usually the
larger half, because everything filed lands in `draft` and stays there until a
person promotes it. Read both before reporting that there is nothing to pick up.

Do not promote a draft yourself. Name the ones that look startable, say what
makes each startable, and let the person you are working with choose.

`git ticket show ID` reads one ticket. It prints the newest note in full and
replaces older ones with a line naming how many there are; `--list` and
`--show N` on the `note` command print those.

`git ticket search QUERY` matches a substring of the title, the body, and the
references. Add `--regex` to search by pattern instead.

Any unique ID prefix works, down to four characters. Copy one from a listing
rather than shortening it yourself, because a ULID opens with a timestamp and
tickets filed together are identical that far in.

### Doing the work

A draft cannot be claimed. If you were asked to pick up something still in
`draft`, that request is the promotion, so run `git ticket status ID ready`
first. A ticket already on the queue needs no such step.

Then `git ticket claim ID` and `git ticket status ID in-progress`.

Read the code before you plan, then write the approach with
`git ticket plan ID "..."`. It replaces rather than appends, so revising it
leaves one plan instead of a stack.

While you work, `git ticket note ID "..."` records what the next person will
need and does not have, and `git ticket ac ID --check N` ticks an acceptance
criterion, counting checkbox lines from one.

Leave unticked any criterion you could not satisfy, and say in a note what
stopped you. Nothing reports an empty box, so an honest one costs nothing, while
a tick you did not earn costs the next reader their trust in every other box.

Finish with `git ticket summary ID "..."` saying where it landed, then
`git ticket status ID done` and `git ticket release ID`.

If you cannot proceed, `git ticket status ID blocked --reason "..."`. The reason
is required.

`note` appends. `plan` and `summary` replace, so correct a note by adding another
that says which one it supersedes.

### Filing new work

`git ticket create --title "..." --type bug --priority high` files a ticket.
Types are task, bug, chore, spike, and epic. Add `--parent` to file it under an
epic.

Run `git ticket config` before you invent a label. It prints what this store
permits, and a label outside that set is a warning that fails
`check --strict`. Give every ticket you file at least one, since a title is
otherwise the only thing saying what it is about; `doctor` reports the ones
carrying none. `git ticket schema` prints the types, priorities, statuses and
error codes every store shares.

Write prose longer than a line to a file and pass the file: `--description-file`
on `create` and `update`, and `--file` on `plan`, `note`, `comment`, and
`summary`. A path of `-` reads stdin.

Inside that prose write subheadings as `###`, because a line opening with `## `
starts a new section and everything below it lands somewhere you did not intend.
The exception runs the other way. If the subheading names a section the format
owns, `### Acceptance criteria` is prose no command can reach. Use `--ac` and
`--dod`, or write `## `, which opens the real section. `check` reports the
mistake as `section_heading_demoted`.

A ticket you file lands in `draft` and stays there. File it, say that you filed
it, and go back to what you were doing.

Record structure rather than describing it in prose:
`git ticket link ID --depends-on OTHER` says this waits on that, and
`git ticket deps ID` walks the chain.

Keep the title under 72 characters. Over that `check` warns, and over 120 the
write is refused.

When you mention a ticket in prose, put its title beside the ID the first time,
then the bare ID is enough. A ULID tells a reader nothing on its own.

### When check fails

`git ticket check --fix --dry-run --strict` plans every repair, prints what it
would do, and writes nothing. Run `git ticket check --fix` and commit what it
changed.

If this project runs the check in CI, it reports the repair and does not commit
it for you.

### Hygiene, which is a different question

`git ticket doctor` reports what is untidy rather than what is broken: a ticket
that is valid and that nobody could pick up. It exits zero however much it has
to say, unless you pass `--strict`, which exits 20 when only soft findings fired
and 21 when a hard one did.

A **hard** finding is a fact you can act on. A **soft** finding is a question the
tool cannot settle, so answer it with your judgement or leave it, but do not edit
a ticket only to silence one: reordering labels without deciding anything loses
the question rather than answering it.

If a rule is wrong for this project, `.tickets/config.yml` turns it off or
retunes it. That is a change to propose, not one to make while tidying something
else.

### Driving it from a script

`--json` on any command gives one envelope on stdout with a stable error `code`
to switch on. Read the vocabulary from `git ticket schema` rather than
hard-coding it.

A write answers with `mutation-result`, whose `ticket` is an `{id, revision}`
stub rather than the ticket, so read the body back with `show --json` when you
need it. Body sections come back camelCase, as `implementationPlan` and
`acceptanceCriteria`.

Every write takes `--if-revision R` and refuses if the ticket moved since you
read it. Pass it whenever you read, decide, and then write.

Text that opens with a dash goes after a bare `--`.

<!-- git-ticket:end -->

## This repository

vuoro is a desktop application and the front door to agentic code review. It
runs reviews with the agent tools the person already has, terva and Claude
Code, with reviewers in parallel and a synthesizer, and it lets the person talk
to the agents while they work. It also gives one person one inbox for every
review they give and get, across GitLab, Forgejo, and GitHub, ordered by whose
turn it is.

Read these in order:

1. [README.md](README.md)
2. [docs/plan.md](docs/plan.md), the design of record. It is not background
   reading.
3. [docs/decisions.md](docs/decisions.md), what the owner decided and what was
   rejected
4. [docs/research.md](docs/research.md), what the sibling projects offer, with
   paths
5. [handoffs/](handoffs/), for orientation

A change that moves the design updates the plan in the same pull request. A
new owner decision gets the next number in docs/decisions.md. Do not reopen a
recorded decision without the owner. A decision that turns out to be wrong is
superseded by a new entry and is never edited away.

It follows the terva-sh pattern for applications. A Go binary serves a loopback
web interface, [tuohi](https://github.com/terva-sh/tuohi) shows it in a native
window, and [terva-sh/design](https://github.com/terva-sh/design) supplies the
styles. The structure is in git-ticket-canvas and `tuohi/examples/loopback`, and
those two are the closest references.

### Rules

- **No cgo, ever, in what ships.** Build and test with `CGO_ENABLED=0`.
  terva-sh releases are cross-compiled from one host, so a dependency that
  needs a C toolchain for any target is not acceptable. A SQLite driver means
  `modernc.org/sqlite`, and a keychain means calling the platform's own tool or
  bus, not a cgo binding.
- **An agent drafts. A person posts.** No code path lets agent output reach a
  forge without an explicit action by the person in the UI. Agent output lands
  as pending drafts marked as drafted. A test holds this, and a change that
  weakens it is refused whatever else it does.
- **Tokens stay out of everything.** A forge token never goes in a config
  file, in `argv`, in a log line, in an error message, in a fixture, or in a
  handoff. Recorded API fixtures are scrubbed before they are committed.
- **The loopback server is attack surface.** Every local process can reach
  it, and it holds the person's forge tokens. Every route goes through the
  protection from TKT-01M3W4HWDY8XV92MXXQWA0AES7 (Refuse loopback API requests
  that did not come from vuoro's window). There are no exceptions for
  convenience.
- **vuoro contains no agent.** It drives agent tools the person already has,
  terva and Claude Code, as subprocesses over their own protocols, and imports
  neither. Only the drivers under `agent/` know a tool's protocol. docs/plan.md
  section 6 explains why.
- **Reviewers are read-only, always, and that is proven.** A reviewer reads its
  worktree and writes only through vuoro's drafts MCP server. It gets no
  shell, no web access, no extensions, and none of the person's own MCP
  servers, because those can include a forge connector that writes. A driver
  whose lockdown has no recorded refusal is not offered. Do not add a "may run
  commands" option to a reviewer, because that is the editor's job.
- **The editor is the only agent that writes code.** There is at most one per
  review, and it runs in its sandbox. It commits locally, never holds a forge
  credential, and never pushes. vuoro pushes its commits only when the person
  accepts them (docs/plan.md section 6.3).
- **Boundaries are tested, not trusted.** Only `tickets` imports git-ticket.
  `forge` and `turn` import neither `agent` nor `tickets`, and they do no
  network I/O in tests. The inbox builds and works with every agent tool
  absent and with no ticket store.
- **The forge is the record.** vuoro's state is a cache plus the person's own
  annotations. Never hold review state that the forge does not also hold.
- **git-ticket integration writes and never commits.** A disposition note
  lands in the working tree of the person's checkout, and they commit it.
- **Pins are deliberate.** terva and tuohi are both pre-1.0, and tuohi's
  first release was v0.1.0. Bump one in its own commit, and read terva's `docs/migrating.md`
  first.

### Commands

`just` lists the recipes, and `just ci` is the gate that CI runs: go vet for
this platform, darwin, and windows, the gofmt check, the tests, the cross
builds, and the ticket-store check, all with `CGO_ENABLED=0`. The steps are
repeated inline in `.forgejo/workflows/ci.yml`, because its container has no
just, so change both together. docs/plan.md section 9 lists the steps still to
come.

The Go line is 1.26, tuohi's line. vuoro imports no agent tool (decision D7),
so terva's 1.27 does not apply. Install Go 1.26 or later. Do not rely on
`GOTOOLCHAIN` to fetch one.

With cgo off, the race detector works on darwin and refuses on linux, so
`just test` adds `-race` on darwin only. Do not turn cgo on to get it on linux.

The frontend is Preact and TypeScript in `web/src`, built by Vite into
`web/dist`, which is committed and embedded, so `go build` needs no Node.
After changing `web/src`, run `just web-build` and commit `web/dist`.
`just ci` fails when the two differ. `just web-setup` installs the locked
packages first.

`just test` skips the test that opens a window. `just test-gui` runs it, and
the GitHub workflow's macOS leg runs it in CI.

### Commits and pull requests

Commit or push only when the person you are working with asks. Commit messages
follow the terva-sh style: a plain declarative sentence, with the ticket ID
first when there is one, as in
`TKT-01M3W4HWB551REE766G3TE2Y7D Scaffold the module and the CI gate`.
Commits use the repository's own identity, which is set in `.git/config`. Do
not override it.

### Handoffs

A session handoff is written to `~/.local/state/agent-handoffs/vuoro/` first.
One that is attached to a ticket is promoted into [handoffs/](handoffs/), which
has its own README. A handoff is orientation, and the tickets are the record.
