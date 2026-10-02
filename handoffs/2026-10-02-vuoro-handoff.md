# 2026-10-02: vuoro, ready for its first working session

Owning ticket: TKT-01M3W4FGT1GY0X3TWJF81X3XWK (vuoro v0.1.0 (MVP): orchestrated
GitLab reviews, then the inbox).
Baseline: the commit that added this file. Find it with
`git log --oneline -1 -- handoffs/2026-10-02-vuoro-handoff.md`. Run
`git log --oneline` to see what has landed since.

This is orientation for the agent that starts building vuoro. The tickets are
the record, the plan is the design, and the decision log is the reasoning. Read
those three rather than trusting this page when they disagree.

## What vuoro is

The owner finds it hard both to give and to get code reviews. The reviews are
spread across gitlab.com (the day job), an internal Forgejo (terva-sh), and
GitHub. Nothing puts them in one place or says whose turn each one is. Running
agents over a review means several Claude Code sessions opened by hand, then a
synthesis done by hand.

vuoro is the **front door** to all of that. It is a desktop application, a Go
binary with a loopback web UI in a tuohi window, that:

- **orchestrates reviews.** It runs reviewers in parallel on the agent tools the
  person already has, terva and Claude Code, and it does not embed either. A
  synthesizer merges their drafts. The person talks to any agent while it
  works, edits the drafts, and posts. An agent never posts.
- **shows one inbox** across forges, sorted by whose turn it is, with the
  reasons.
- **at v1.0, adds an editor agent.** There is one per review, it runs in a
  sandbox, and it fixes the person's own change from the reviewers' findings.
  It commits locally, and it never pushes.

## Read these first, in order

1. `README.md` and `AGENTS.md`. The rules in AGENTS.md are hard rules: no cgo,
   an agent drafts and a person posts, reviewers are read-only, tokens stay out
   of everything, and the loopback server is attack surface.
2. `docs/plan.md`. Section 10 is the roadmap, and section 6 is the agent
   design.
3. `docs/decisions.md`, decisions D1 to D27. Do not reopen one without the
   owner.
4. `docs/research.md`: what tuohi, design, git-ticket-canvas, terva, Claude
   Code, and terva-action-code-review offer, with paths and versions.

## State at handoff

- **No code exists yet.** The repository holds the documents above, the MIT
  licence, `.gitignore`, and a justfile whose `ci` recipe is only the ticket
  store check.
- **The store** has two epics and 45 tickets, all in `draft`.
  `git ticket check --fix --dry-run --strict` and `git ticket doctor` are clean.
  - The MVP epic is TKT-01M3W4FGT1GY0X3TWJF81X3XWK, milestone `v0.1.0`.
  - The v1.0 epic is TKT-01M3YMSTYMFRDFXD4QSVWHMM8P (vuoro v1.0.0: every forge,
    the editor agent, and a terva-sh release), milestone `v1.0.0`.
  - One ticket sits after v1.0, with no parent.
- **There is no remote.** It is local only, at
  `~/Workspace/github.com/terva-sh/vuoro`, on `main`. Commits use
  `Drew Short <warrick@sothr.com>` from `.git/config`.

## Your first session

1. **Check the ground.**

   ```sh
   git log --oneline -5
   just ci
   git ticket list --status draft
   go version; terva --version; claude --version
   ```

   Expect Go 1.26, terva 0.139.7 or later, and Claude Code 2.1.285 or later. To
   bring terva up to date, run `terva update`.
2. **Ask the owner which drafts to promote.** Nothing is `ready` yet, and
   promotion is the owner's. These have no dependencies, and they lead to the
   first usable slice, TKT-01M3YK0CPYG132897XVVZ9G4EF (Run reviewers in
   parallel, and synthesize their drafts):
   - TKT-01M3W4HWB551REE766G3TE2Y7D (Scaffold the Go module, justfile gate, and
     CI without cgo). Everything with code waits on it. Start at Go 1.26, and
     read its two notes.
   - TKT-01M3YK0CFFKAPM2QHZGGZMDHND (Decide the agent driver interface for terva
     and Claude Code). It gates every agent ticket, and it can run alongside the
     scaffold. Its hardest question is whether claude.ai connectors and plugins
     stay out of a `claude -p` session. This workstation has a GitLab connector
     with write tools, so prove that they do.
   - TKT-01M3W4HWJZ1013S2MPZ69B72AW (Decide the forge model and the turn rules)
     and TKT-01M3W4HWM6V2F20KNT03BSBNZY (Decide credential handling and the
     local state format). Together they gate opening a GitLab change by URL.
   - TKT-01M3W4HWF70KRCRRXZQT2M9C7Y (Add a vuoro accent to terva-sh/design and
     release it). The work is in the design repository, at
     `~/Workspace/git.local.sothr.com/terva-sh/design`.

   `git ticket deps TKT-01M3YK0CPYG132897XVVZ9G4EF --transitive` prints the
   whole path to the first slice.
3. **Work one ticket at a time,** as AGENTS.md says:
   - Promote only what the owner asked for.
   - Claim, and set `in-progress`.
   - Read the code, then write the plan with `git ticket plan`.
   - Work on a branch per ticket, such as `tickets/scaffold`.
   - Record notes, and tick only the criteria you earned.
   - Write a summary, mark the ticket done, and release it.
   - Pass `--actor agent:TOOL/SESSION` on every write.
4. **After the scaffold, do the walking skeleton early:**
   TKT-01M3W4HWCNR50KXMCX741QD9XQ (Walking skeleton: a loopback server shown
   in a tuohi window). It is the riskiest technical step, because tuohi reached
   v0.1.0 only on 2026-10-01 and its macOS engine has been tested only on
   GitHub's runners. When it lands, TKT-01M3YMST0M467K1FB4J5B2C7B2 (Move vuoro to
   terva-sh/vuoro on Forgejo once the skeleton lands) is the owner's push, not
   yours.

## Where to stop and ask

- **Pushing, or creating a remote.** That belongs to the owner, always.
- **Promoting a draft** the owner did not name.
- **Changing a recorded decision.** Propose it, and let the owner rule. If
  they agree, add a superseding entry to docs/decisions.md.
- **Anything that needs a secret's value.** Credentials reach vuoro through a
  command or the environment at run time (docs/plan.md section 5.1). Never read
  or print a token. The GitLab token is not in the owner's age store yet; once
  the credentials spike settles, the owner adds it as `gitlab-pat`.

## Environment on the owner's workstation

- **Sibling checkouts.**
  - Under `~/Workspace/github.com/terva-sh/` are terva, tuohi, git-ticket, and
    git-ticket-canvas, each with a `forgejo` remote. Run `git fetch forgejo`
    before reading them.
  - Under `~/Workspace/git.local.sothr.com/terva-sh/` are
    terva-action-code-review, design, and ketju.
  - Forgejo is every repository's source, and the GitHub mirrors trail it.
- **Unreadable.** `terva-sh/meta` and `terva-sh/terva-code-review` do not read
  anonymously.
- **Forge CLIs.** `glab` is logged in to gitlab.com. `gh` and `tea` are not
  installed.
- **Prior art for the GitLab adapter** is in `~/Workspace/code-reviews/tools/`
  (Python). Do not read its `.env`.
- **The employer's policy** is about PHI, and decision D8 covers it. The owner's
  own Claude login is fine for reviews. The live risk is PHI pasted into a
  change, and it is the same as in a manual session.
