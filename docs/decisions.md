# Decisions

Every decision the owner made while vuoro was being scoped, in the order it was
made. docs/plan.md is the design that follows from them. This file is the
record of what was asked, what else was on the table, and why it went the way
it did. Read it before you propose reopening something.

A superseded decision stays here, marked as such, with the decision that
replaced it. A new decision gets the next number. Never renumber.

Decisions D1 to D7 were made on 2026-10-01 and 2026-10-02 during the bootstrap.
D8 to D27 come from a question-by-question review with the owner on 2026-10-02.

## Bootstrap

### D1. The name is vuoro

*Vuoro* is Finnish for "turn", as in "it is your turn". terva-sh names are
Finnish nouns. The alternatives were katselmus ("review", precise but long for
a binary), seula ("sieve"), and pino ("pile"). The name puts the core idea in
front: knowing whose turn each review is.

### D2. It lives at `~/Workspace/github.com/terva-sh/vuoro`

The module is `github.com/terva-sh/vuoro`, beside tuohi and git-ticket, because
vuoro may be contributed to terva-sh. The alternative, a top-level personal
checkout, kept the module path and lost the grouping.

### D3. Commits use the owner's personal identity

The repository's `.git/config` sets `Drew Short <warrick@sothr.com>`, as every
terva-sh repository uses. The global identity on this workstation belongs to
the employer, and it must not appear in a terva-sh history.

### D4. GitLab first

The first forge is gitlab.com, because that is where the owner's day-job
reviews are, and `~/Workspace/code-reviews/` already has working GitLab review
tooling to learn from. A forge interface comes first, so that Forgejo and GitHub
are additions and not a rewrite. The alternatives were Forgejo and GitHub first,
or all three at once.

### D5. git-ticket integration is optional

The owner asked for it as "not required but a helpful addition for most of my
own repos". vuoro must work fully without a ticket store. D27 says what lands
when.

### D6. vuoro does not use terva-action-code-review (supersedes a bootstrap proposal)

The bootstrap session proposed drafting reviews by running the action's local
mode and reading its `local-1.x` JSON. The owner rejected that on 2026-10-02.
The action is an internal proof of concept. **vuoro is the front door** to that
kind of work, with the core job of running reviews directly and letting the
person interact with the agents doing them. The action is prior art and
inspiration only.

### D7. vuoro drives agent tools as subprocesses, and embeds none

vuoro drives terva over `terva rpc` and Claude Code over its stream-json mode,
and it imports neither, so the person chooses the tool. The alternative was
importing terva's Go SDK or engine. The decision had three consequences:

- vuoro holds no model credential.
- The Go line stays at tuohi's 1.26 instead of terva's 1.27.
- A tool release cannot break vuoro's build, though it can break a driver.

### D8. The employer's policy does not restrict which login a review uses (correction)

The bootstrap session read the employer's PHI policy as forbidding a
subscription login for any LLM work on day-job code, and it proposed making
vuoro refuse one. The owner corrected that on 2026-10-02. The policy is about
PHI, not automation, and it treats GitLab source code as low-risk. vuoro is an
orchestrator: a review in vuoro is the person running several review sessions
by hand, not an automated in-and-out pipeline. vuoro shows the account kind on
every run and leaves the choice to the person. The real remaining risk, PHI
pasted into a change, is the same as in a manual session.

## Agents

### D9. Agents write drafts through an MCP server that vuoro serves

The question was how a driven agent creates and changes pending drafts. The
answer is a small MCP server served to each session, with tools to list,
propose, revise, and withdraw drafts. The alternatives:

- **Structured output** that vuoro parses. Claude Code offers it with
  `--json-schema`, terva does not, and edits mid-conversation are awkward.
- **Both channels.**

MCP behaves the same for both tools, shows the agent each result at once, and
makes the boundary structural: an agent's only write tool writes to a draft.

### D10. Reviewers are read-only, always

A reviewer gets Read, Grep, and Glob over its worktree, plus the drafts server,
and nothing else. It gets no shell, no web access, and none of the person's MCP
servers. A reviewer never runs the change's code. Three alternatives were
rejected:

- read-only for the MVP with a shell later;
- a shell from the start;
- each tool's own permission prompts.

D17 puts running code in a separate agent instead.

### D11. The MVP orchestrates: reviewers in parallel, then a synthesizer

Several reviewer sessions run on one change at once, on any mix of tool, model,
and profile. Each runs in a pane the person can talk to. A synthesizer session
then lists every reviewer's drafts and proposes merged drafts that name their
sources. With D9 in place, a synthesizer is just one more session.

The alternatives were parallel sessions synthesized by hand, or one session in
the MVP. The owner's comparison is the reason: the manual workflow being
replaced is several Claude Code sessions plus a synthesis.

### D12. The orchestrator comes first, and the inbox second

The first usable slice opens a GitLab merge request by URL, runs the parallel
reviewers and the synthesis, lets the person talk to them and edit the drafts,
and submits the review. The whose-turn inbox comes second and feeds the
orchestrator.

The orchestrator is useful the day it lands, and it proves the riskiest parts
first: the drivers, the drafts server, and tuohi on macOS. The alternatives
were the inbox first, or both thin together.

### D13. On my own change, the MVP triages and drafts replies

An agent groups the open threads as must fix, discuss, or nit, and drafts
replies. The bootstrap answer also planned a hand-off to the person's own
coding session at v1.0. D17 supersedes that part. The alternatives were triage
without any hand-off, or no agent help on the author's side at all.

### D14. Presets are the person's, and a repository adds instructions at v1.0

In the MVP, presets and profiles live in vuoro's own config, with built-in
code, security, and documentation profiles. At v1.0, a repository can add a
checklist and conventions, layered on the preset and read at the base commit,
so that a change cannot steer its own review. A repository never picks the
reviewers. The alternatives were personal config only, or the repository first,
like the review action's `.terva/review.yml`.

### D15. Runs are pinned to a head SHA, and nothing re-runs on its own

In the MVP, a run whose change has moved is flagged as "reviewed at abc123,
2 commits behind". At v1.0, drafts on unchanged lines are re-anchored, and an
incremental run reviews only the new commits, with the earlier findings as
context. The alternatives were a flag only, or an automatic re-run on every
push.

### D16. Cost is shown in the MVP and capped at v1.0

Every session shows tokens and cost as its tool reports them, and every run
shows a total. v1.0 adds an optional per-run budget that stops sessions at the
cap, and a monthly total. The alternatives were showing cost and never capping
it, or caps in the MVP.

## The editor agent

### D17. Mutation is a separate class of agent: one editor per review

The owner raised this during the review on 2026-10-02. Allowing mutation brings
its own problems, parallelism above all. Each review may get one dedicated
editor agent, with a full sandbox, that works from the reviewers' priors.

Reviewers stay read-only and parallel. The single editor is the only writer,
and that is what keeps parallelism safe. The editor replaces D13's hand-off to
an outside session.

### D18. The editor arrives at v1.0, on the author's side first

On the person's own change, the editor fixes code from the triaged threads and
the reviewers' findings. Reviewer-side use, which turns a finding on somebody
else's change into a suggested patch, comes after v1.0. The alternatives were
both sides at v1.0, or the editor after v1.0 altogether.

### D19. The editor commits locally, and the person decides what is pushed

The editor commits on a local branch, one commit per finding or thread it
addresses, with a message that names what it answers. vuoro shows those commits
as a diff for review. The person accepts them, and vuoro pushes on that action,
or discards them. The editor never holds a forge credential and never pushes.
The alternatives were an uncommitted patch, or an editor that pushes once its
plan is approved.

### D20. The editor's sandbox is a container by default, and a spike decides

The default is podman or docker with only the editor's worktree mounted, no
forge credentials, and network limited to the model endpoint and package
registries. Before the editor is built, a spike compares that with terva's
lock, Claude Code's sandbox, and the native options on macOS. The alternatives
were each tool's own sandbox, or a separate worktree with no OS sandbox.

## Scope and release

### D21. v1.0 supports all three forges: Forgejo next, then GitHub

Forgejo goes second because it hosts terva-sh's work, and it is where vuoro
gets reviewed with vuoro. GitHub goes third. The alternatives were GitHub
second, or GitLab and Forgejo only.

### D22. Local branch review arrives at v1.0

The person points vuoro at a checkout and a range, such as
`origin/main..HEAD`. It runs the same orchestrator, and the findings become a
local checklist that carries into the merge request. It is not in the MVP,
because the MVP's job is proving the forge loop. The alternatives were the
MVP, or never.

### D23. macOS for the MVP, and releases for every platform at v1.0

The MVP installs with `just install`, and CI still cross-builds Linux and
Windows. v1.0 publishes goreleaser archives for all three platforms and
`vuoro self-update`, and it decides macOS signing. The alternatives were macOS
and Linux at v1.0, or macOS only through v1.0.

### D24. Nudges arrive at v1.0

In the MVP, vuoro refreshes only while its window is open. v1.0 adds the tray
and a background refresh, and it sends quiet, batched notifications when it
becomes the person's turn or when a long run finishes. The alternatives were
nudges in the MVP, or never: a place you go, fed to rituals by
`vuoro inbox --json`.

### D25. The MVP is the owner's, and v1.0 is a terva-sh release

The repository moves to terva-sh/vuoro on Forgejo once the walking skeleton
lands, so that its CI and review norms apply early, with no public mirror. v1.0
is a public release with a GitHub mirror, setup in the UI, and documentation
for somebody new. The alternatives were personal through v1.0, or public from
the first commit.

### D26. Review history is kept while the change is open, then 30 days

After that, the transcripts are deleted and one line per run remains: tools,
models, cost, and finding counts. The period is configurable. Day-job code then
does not linger in local files. The alternatives were keeping everything until
the person deletes it, or deleting on close.

### D27. git-ticket: read in the MVP, write at v1.0

In the MVP, a linked ticket is shown beside the change, and its acceptance
criteria go to every reviewer, so that the review checks the change against
what was asked. At v1.0, finding dispositions are written back as ticket notes,
and the editor sees the ticket too. The alternatives were all of it at v1.0, or
all of it in the MVP.
