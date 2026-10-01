# vuoro

vuoro is one desktop window for every code review you give and get. It covers
merge requests on GitLab and pull requests on GitHub and Forgejo. It orders them
by **whose turn it is**, and for how long, and you read, comment, reply, approve,
and submit without leaving it.

*Vuoro* is Finnish for "turn", as in "it is your turn".

> **Status: bootstrap.** There is no code yet. [docs/plan.md](docs/plan.md) is
> the design of record, and the work is tracked as tickets in `.tickets/`.

## Why

A forge records events. It does not tell you what is waiting on you. A
requested review sits unseen in one forge's inbox. A reviewer's reply hands the
ball back to the author in another, and nothing says so. vuoro derives the turn
for each change from the forge's own record, and it shows the reason: "your
turn: two replies on your threads, 3 days ago".

## What it will do

- **One inbox** across GitLab, GitHub, and Forgejo, sorted by whose turn and
  how long it has been.
- **Review in place:** the diff, the threads, inline comments, replies,
  resolve, approve or request changes, and submit as one batch.
- **Agentic help, built on [terva](https://github.com/terva-sh/terva):**
  summarise a change, draft a review, draft a reply. An agent only drafts.
  Nothing reaches a forge until you post it.
- **Optional [git-ticket](https://github.com/terva-sh/git-ticket) integration:**
  show the ticket a change implements, and record review dispositions on it.

## How it is built

- One Go binary, `CGO_ENABLED=0`, cross-compiled like every terva-sh release.
- A loopback `net/http` server, shown in a native window by
  [tuohi](https://github.com/terva-sh/tuohi) over the operating system's own
  web engine. There is no bundled browser.
- A Preact and TypeScript frontend, built with Vite. Its assets are committed
  and embedded, so `go build` needs no Node toolchain.
- Styles from the shared [terva-sh design](https://github.com/terva-sh/design)
  foundation.

## Working on it

Read [AGENTS.md](AGENTS.md) and [docs/plan.md](docs/plan.md). `git ticket ready`
lists startable work, and `git ticket list --status draft` lists the rest of the
backlog. [handoffs/](handoffs/) has orientation for a session that picks the
work up.

## License

MIT. See [LICENSE](LICENSE).
