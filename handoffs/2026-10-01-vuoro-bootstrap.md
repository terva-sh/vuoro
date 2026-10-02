# 2026-10-01: vuoro bootstrap

Owning ticket: TKT-01M3W4FGT1GY0X3TWJF81X3XWK (vuoro v0.1.0 (MVP): orchestrated
GitLab reviews, then the inbox).
Baseline: the repository's first commit on `main`. Verify it with
`git log --oneline -3` before relying on anything here.

This is orientation for the session that picks up vuoro. The tickets are the
record, so read them rather than trusting this page.

## What vuoro is, and why it exists

- **The problem.** The owner struggles to give and to get code reviews.
  Reviews are spread across gitlab.com (the day job), GitHub, and an internal
  Forgejo (terva-sh). Nothing puts them in one place, and nothing says whose
  turn it is. A reviewer's reply can wait weeks for the author to notice it.
- **What it does.** It is one desktop window with one inbox, sorted by whose
  turn and for how long. The review is done in place, and the agentic help is
  built on terva. docs/plan.md has the full design.
- **The name.** *Vuoro* is Finnish for "turn". The owner chose it on
  2026-10-01 from vuoro, katselmus, seula, and pino.

## Decisions already made (do not reopen without the owner)

- **Name, place, module.** vuoro, at `~/Workspace/github.com/terva-sh/vuoro`,
  module `github.com/terva-sh/vuoro`. It follows terva-sh conventions because
  it may be contributed to the org. There is no remote yet. Creating one is the
  owner's call.
- **Identity.** Commits use `Drew Short <warrick@sothr.com>`, set in this
  repository's `.git/config`, as every terva-sh repository uses. The global
  identity on this machine is the employer's, and it must not leak in here.
- **Stack.** Go with no cgo, tuohi for the window, terva-sh/design for styles,
  and Preact, TypeScript, and Vite for the frontend, as git-ticket-canvas does.
- **vuoro is the front door to agentic review, and it drives the tools.**
  Decided by the owner on 2026-10-02. vuoro runs reviews itself, and the person
  talks to the agent doing them. It drives terva (`terva rpc`) and Claude Code
  (stream-json) as subprocesses and imports neither, so the person chooses the
  tool. terva-sh/terva-action-code-review is an internal proof of concept. It
  is prior art to read, never a dependency to run. docs/plan.md section 6 has
  the design.
- **Scope from here to v1.0.0.** The owner settled 17 questions on 2026-10-02.
  docs/plan.md section 10 is the result, and section 10.4 lists what was
  decided against. In short:
  - the MVP is built orchestrator first, with parallel reviewers and a
    synthesizer;
  - agents write only through vuoro's drafts MCP server;
  - reviewers are read-only, always;
  - mutation belongs to a separate editor agent at v1.0.0, one per review, in a
    sandbox, and it never pushes;
  - v1.0.0 is a public terva-sh release covering all three forges.
- **GitLab first.** The MVP's forge is gitlab.com. Forgejo and then GitHub
  are in the v1.0.0 epic, TKT-01M3YMSTYMFRDFXD4QSVWHMM8P (vuoro v1.0.0: every
  forge, the editor agent, and a terva-sh release).
- **git-ticket integration is optional.** The owner asked for it mid-bootstrap
  as "not required, but a helpful addition for most of my own repos".
  docs/plan.md section 7 describes it, and vuoro must work fully without it.

## Verified state at handoff

- No code exists. The repository has a README, AGENTS.md, docs/plan.md, the
  MIT licence, a justfile whose `ci` recipe is only the store check, and this
  handoff.
- The ticket store has two epics and 45 tickets, all in `draft`.
  `git ticket check --fix --dry-run --strict` and `git ticket doctor` were both
  clean.
- Pins were read from Forgejo on 2026-10-02: terva v0.139.7, tuohi v0.1.0,
  design v0.1.0, and git-ticket v0.24.0. v0.23.0 is the installed binary.
  docs/plan.md section 3.2 has the table.

## Where to start

Every ticket is a draft, and promotion is the owner's. The first usable slice
is TKT-01M3YK0CPYG132897XVVZ9G4EF (Run reviewers in parallel, and synthesize
their drafts). `git ticket deps TKT-01M3YK0CPYG132897XVVZ9G4EF --transitive` is
the whole path to it. These have no dependencies, and they unlock it:

1. **TKT-01M3W4HWB551REE766G3TE2Y7D (Scaffold the Go module, justfile gate, and
   CI without cgo).** Everything with code waits on it. It starts at Go 1.26.
2. **TKT-01M3YK0CFFKAPM2QHZGGZMDHND (Decide the agent driver interface for
   terva and Claude Code).** It gates every agent ticket, and it proves each
   tool's read-only lockdown. It can run alongside the scaffold.
3. **TKT-01M3W4HWJZ1013S2MPZ69B72AW (Decide the forge model and the turn
   rules)** and **TKT-01M3W4HWM6V2F20KNT03BSBNZY (Decide credential handling
   and the local state format).** Together they gate opening a GitLab change by
   URL.
4. **TKT-01M3W4HWF70KRCRRXZQT2M9C7Y (Add a vuoro accent to terva-sh/design and
   release it).** This is work in the design repository. It is quick, and only
   the stylesheet ticket waits on it.

After the scaffold, TKT-01M3W4HWCNR50KXMCX741QD9XQ (Walking skeleton: a
loopback server shown in a tuohi window) is the riskiest technical step,
because tuohi only reached v0.1.0 on 2026-10-01, and its macOS engine has
been tested only on GitHub's runners. Do it early. When it lands,
TKT-01M3YMST0M467K1FB4J5B2C7B2 (Move vuoro to terva-sh/vuoro on Forgejo once
the skeleton lands) is the owner's push.

## Environment on the owner's workstation

- **Go.** The workstation has Go 1.26.1, and tuohi declares 1.26.0, so the
  module starts at 1.26. vuoro imports no agent tool, so terva's 1.27 does not
  apply.
- **Agent tools.** Claude Code 2.1.285 and terva 0.139.7 are installed.
  terva was updated on 2026-10-02 with `terva update`, which is how to keep it
  current.
- **Forgejo is the source, GitHub is a mirror.** Every terva-sh repository's
  `origin` is the internal Forgejo at `git.local.sothr.com/terva-sh`, and it
  reads anonymously over HTTPS from this machine. The GitHub mirrors trailed it
  by days on 2026-10-02, with tuohi 22 commits behind and git-ticket 224
  behind. Read code and pins from Forgejo.
- **Sibling checkouts.** terva, tuohi, git-ticket, and git-ticket-canvas are
  under `~/Workspace/github.com/terva-sh/`, cloned from the mirrors, each with
  a `forgejo` remote added. tuohi and terva are checked out at their Forgejo
  heads. git-ticket and git-ticket-canvas were already there and are fetched
  only. terva-action-code-review, design, and ketju are under
  `~/Workspace/git.local.sothr.com/terva-sh/`, cloned from Forgejo. design is
  identical on both forges at v0.1.0.
- **Private repositories.** `terva-sh/meta` and `terva-sh/terva-code-review`
  (a dashboard the action's tickets propose) refuse anonymous reads.
- **Forge CLIs.** `glab` is logged in to gitlab.com. `gh` and `tea` are not
  installed.
- **GitLab token.** The age store (`ritual-secret ls`) has no GitLab token.
  Once the credentials spike settles on `token_command`, the owner can add one
  as `gitlab-pat` and point the command at `ritual-secret get gitlab-pat`.
  Never read or print the value. Let a script resolve it.
- **Prior art.** `~/Workspace/code-reviews/` is the owner's existing GitLab
  review workflow, in Python. `tools/fetch_mr.py` handles the diff-truncation
  fallback, and `post_inline_suggestions.py` posts positioned inline
  suggestions. The adapter tickets should mine both for API detail.
- **Employer policy on day-job code.** The employer's policy is about PHI, not
  automation: no PHI goes into a Claude tool on a claude.ai subscription
  login, and GitLab source code counts as low-risk. A review in vuoro is the
  owner running review sessions by hand, so the owner's own login is fine for
  it. The owner corrected an earlier, stricter reading of the policy on
  2026-10-02. The risk that remains is PHI pasted into a change, such as
  customer logs or a data dump in a fixture, and it is the same risk as in a
  manual session.

## Process

- Name yourself on every `git ticket` write, as `--actor agent:TOOL/SESSION`.
  The bootstrap session was `agent:claude/vuoro-bootstrap`.
- Until a remote exists there are no pull requests. Work on a branch per
  ticket, and let the owner merge. Once Forgejo hosts the repository, follow
  tuohi's `docs/pr-reviews.md`, including the terva review.
- Nothing is pushed anywhere without the owner.
