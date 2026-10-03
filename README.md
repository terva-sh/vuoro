# vuoro

vuoro is one desktop window for every code review you give and get. It covers
merge requests on GitLab and pull requests on GitHub and Forgejo. It orders them
by **whose turn it is**, and for how long, and you read, comment, reply, approve,
and submit without leaving it.

*Vuoro* is Finnish for "turn", as in "it is your turn".

> **Status: reviews and a combined GitLab/Forgejo inbox are implemented.** Open
> changes by URL or discover your reviews automatically, with turn reasons,
> unanswered ages, account refresh status and snoozes. Run reviews with Claude
> Code from the same workspace. GitLab posting awaits live validation; Forgejo
> supports reading and agent reviews. Personal-account GitLab discovery parity
> still awaits local authentication. [docs/plan.md](docs/plan.md) is the design
> of record, and [docs/roadmap.md](docs/roadmap.md) maps remaining MVP/v1.0 work.


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
- **Agentic review, with the agent you choose:** vuoro runs a review with
  [terva](https://github.com/terva-sh/terva) or Claude Code, whichever you
  have, and keeps the conversation open beside the change. Ask it why, point it
  somewhere else, and edit what it drafts. An agent only drafts. Nothing reaches
  a forge until you post it.
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

Read [AGENTS.md](AGENTS.md), [docs/plan.md](docs/plan.md),
[docs/decisions.md](docs/decisions.md), and [docs/research.md](docs/research.md).
`git ticket ready`
lists startable work, and `git ticket list --status draft` lists the rest of the
backlog. [handoffs/](handoffs/) has orientation for a session that picks the
work up.

## Opening changes

Paste a GitLab merge-request URL or a Forgejo pull-request URL into the window.
Public changes can open anonymously. For a private Forgejo, add an account to
vuoro's `config.json` in the platform's configuration directory:

```json
{
  "forges": [
    {
      "name": "internal",
      "kind": "forgejo",
      "url": "https://git.local.sothr.com",
      "username": "YOUR_USERNAME",
      "token_env": "VUORO_FORGEJO_TOKEN",
      "clone": "ssh"
    }
  ]
}
```

Set the named environment variable before launching vuoro, or use a
`token_command` array that retrieves the credential without a shell. Never put
its value in the file. Each host has one configured account. The forge's SSH
clone URL supplies its port; git uses your own SSH keys. `clone: "https"` uses
your git credential helper instead. Agent reviews need the agent tool installed
and logged in separately.

Forgejo posting controls remain unavailable until its write adapter lands.
GitHub is not supported yet.

## Shared styling

Vuoro's violet accent and the Birch/Tar foundation come from
`terva-sh/design`, pinned as a Go tool in `go.mod`. To refresh the committed
stylesheet after updating that pin:

```sh
just design-export
just web-build
```

Commit the exported stylesheet and `web/dist` together. `just design-check`
compares the stylesheet with the pinned design version; `just ci` and both
forge workflows run the same check. Components use `--ui-*` roles, and
`data-scheme` on `<html>` overrides the system's light/dark choice.

## License

MIT. See [LICENSE](LICENSE).
