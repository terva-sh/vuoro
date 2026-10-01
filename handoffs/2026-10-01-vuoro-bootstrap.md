# 2026-10-01: vuoro bootstrap

Owning ticket: TKT-01M3W4FGT1GY0X3TWJF81X3XWK (vuoro v0.1.0: one inbox for the
reviews I give and get).
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
  terva for LLM features, and Preact, TypeScript, and Vite for the frontend,
  as git-ticket-canvas does.
- **GitLab first.** The first working slice is gitlab.com. GitHub and Forgejo
  are filed as their own tickets against the `forge` interface, outside the
  v0.1.0 epic.
- **git-ticket integration is optional.** The owner asked for it mid-bootstrap
  as "not required, but a helpful addition for most of my own repos".
  docs/plan.md section 7 describes it, and vuoro must work fully without it.

## Verified state at handoff

- No code exists. The repository has a README, AGENTS.md, docs/plan.md, the
  MIT licence, a justfile whose `ci` recipe is only the store check, and this
  handoff.
- The ticket store has one epic and 21 tickets, all in `draft`.
  `git ticket check --fix --dry-run --strict` and `git ticket doctor` were both
  clean.
- Pins were read from the sibling checkouts on 2026-10-01: terva v0.139.5,
  tuohi v0.1.0-alpha.1, design v0.1.0, and git-ticket v0.23.0 (installed).
  docs/plan.md section 3.2 has the table.

## Where to start

Every ticket is a draft, and promotion is the owner's. These have no
dependencies:

1. **TKT-01M3W4HWB551REE766G3TE2Y7D (Scaffold the Go module, justfile gate, and
   CI without cgo).** Everything with code waits on it. It also decides the Go
   line (see the environment section below).
2. **TKT-01M3W4HWJZ1013S2MPZ69B72AW (Decide the forge model and the turn
   rules).** This is the largest design risk, because the adapter, the turn
   package, and the inbox all take their shape from it. It can run alongside
   the scaffold.
3. **TKT-01M3W4HWM6V2F20KNT03BSBNZY (Decide credential handling and the local
   state format).** It is small, and it gates the GitLab adapter together with
   the model spike.
4. **TKT-01M3W4HWF70KRCRRXZQT2M9C7Y (Add a vuoro accent to terva-sh/design and
   release it).** This is work in the design repository, not here. It is quick,
   and the design stylesheet ticket waits on it.

After the scaffold, TKT-01M3W4HWCNR50KXMCX741QD9XQ (Walking skeleton: a
loopback server shown in a tuohi window) is the riskiest technical step,
because tuohi is an alpha and its macOS engine has been tested only on GitHub's
runners. Do it early.

TKT-01M3W4HWW7FHHC6VD5M0S9MV90 (Decide how vuoro embeds terva for agent
actions) and TKT-01M3W4HX0BNXTDC2DXBH5HQ0FM (Decide how a change finds its
git-ticket store and ticket) also have no dependencies. Nothing depends on them
until the change view exists, so they can wait.

## Environment on the owner's workstation

- **Go.** The workstation has Go 1.26.1. terva's `go.mod` declares 1.27.0, so
  the module needs 1.27 once it imports terva. tuohi declares 1.26.0 and
  design 1.25.0. The scaffold ticket decides between installing 1.27 and
  relying on `GOTOOLCHAIN=auto`.
- **Sibling checkouts.** terva, tuohi, design, git-ticket, and
  git-ticket-canvas are cloned from their public GitHub mirrors under
  `~/Workspace/github.com/terva-sh/`. The clone of git-ticket is behind the
  installed binary.
- **Private repositories.** `terva-sh/terva-action-code-review`, `terva-sh/meta`,
  and `terva-sh/ketju` are not readable with the credentials on this machine.
  The terva spike ticket needs the first of them, so ask the owner for access
  or for a copy.
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
- **Employer policy on day-job code.** Under the employer's PHI policy, LLM calls
  over day-job merge requests go through an API-key provider account covered by
  the employer's BAA, never through a subscription login. Source code is low
  risk, but do not route forge content anywhere the owner did not configure.

## Process

- Name yourself on every `git ticket` write, as `--actor agent:TOOL/SESSION`.
  The bootstrap session was `agent:claude/vuoro-bootstrap`.
- Until a remote exists there are no pull requests. Work on a branch per
  ticket, and let the owner merge. Once Forgejo hosts the repository, follow
  tuohi's `docs/pr-reviews.md`, including the terva review.
- Nothing is pushed anywhere without the owner.
