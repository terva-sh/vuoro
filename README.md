# vuoro

vuoro is one desktop window for every code review you give and get. It covers
merge requests on GitLab and pull requests on GitHub and Forgejo. It orders them
by **whose turn it is**, and for how long, and you read, comment, reply, approve,
and submit without leaving it.

*Vuoro* is Finnish for "turn", as in "it is your turn".

> **Status: reviews and a combined GitLab/Forgejo inbox are implemented.** Open
> changes by URL or discover your reviews automatically, with turn reasons,
> unanswered ages, account refresh status and snoozes. Run reviews with Claude
> Code from the same workspace. On your own changes, triage human threads into
> Must fix, Discuss and Nit, then edit and accept drafted replies explicitly.
> [Author interactions](docs/author-triage.md) describes the flow.
> GitLab posting awaits live validation; Forgejo
> supports reading and agent reviews. Personal-account GitLab discovery parity
> passed for an empty inbox; live validation limits are in the roadmap.
> [docs/plan.md](docs/plan.md) is the design
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

Paste a GitLab merge-request URL, or a Forgejo or GitHub pull-request URL, into
the window. Public GitLab and Forgejo changes can open anonymously; a GitHub
change needs your GitHub account set up first. To read your own reviews, add your forges
in the window: a fresh install opens at a guided setup that picks a forge,
connects its token, checks your agent tools, and asks how vuoro runs in the
background. Settings, at the top right, adds, edits, and removes forges later,
and shows how far each forge's token reaches. vuoro never asks for a
token. You keep it in your keychain, a password manager, or an environment
variable, and setup tests that source against the forge before it saves.

Setup writes vuoro's `config.json` in the platform's configuration directory,
which you can also edit by hand:

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

Claude reviewers deliberately skip user and project settings to preserve their
read-only tool restrictions. If your authentication is supplied by those
settings, provide it through Vuoro's launch environment instead, using Claude's
authentication and endpoint environment variables. Vuoro does not read or copy
your Claude settings file, and authentication values must stay out of Vuoro's
configuration file.

Forgejo posts reviews too. Its pending comments stay in Vuoro until you
submit, because Forgejo would show them to others sooner, and it cannot
resolve a thread or withdraw an approval, so Vuoro offers neither there.
GitHub works through a configured account, since GitHub refuses anonymous
reads of reviews. Your pending comments go into a GitHub pending review, which
only you can see until you submit; a comment on the whole change stays in Vuoro
until then, because GitHub cannot change a pending review's summary. A reviewer
cannot withdraw an approval on GitHub, so Vuoro does not offer it. An editor's
fixes on a GitHub pull request go up as a draft pull request onto its branch,
or straight to the branch when the pull request is yours.

Closing the window leaves Vuoro in the tray, where it keeps refreshing your
inbox, every 15 minutes unless you choose otherwise. It sends one quiet
notification when changes become your turn, and one when a long review
finishes. Launching Vuoro again opens the window of the one already running.
On a desktop with no tray, such as GNOME without the AppIndicator extension,
closing the window quits. Under "In the background" in the window you can
change how often it refreshes, turn notifications off, or have Vuoro start in
the tray when you log in.

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
