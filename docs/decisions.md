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

## After the driver spike

### D28. The MVP drives Claude Code only, and terva waits past it

The driver spike found on 2026-10-02 that terva 0.139.7 cannot run as a locked
reviewer. It has no way to take vuoro's drafts server for one run while it
keeps the person's login, and the person's config hooks always load (plan
section 6.6). The owner chose to defer terva past the MVP. The MVP's reviewers
run on Claude Code. The terva driver moves to v1.0.0 and waits on a spike that
proves its lockdown against a terva release that allows one. No change to terva
is asked for now.

This narrows D7 and D11 for the MVP only. vuoro still drives tools as
subprocesses and embeds none. Two alternatives were rejected:

- asking terva upstream now for an exclusive per-run server flag and a way to
  skip the person's hooks;
- shipping the drafts tools as a terva extension through `--ext`, which departs
  from D9 and still leaves the hooks loaded.

### D29. A token comes from a command, and state is JSON files

The owner chose on 2026-10-02 how vuoro gets a forge token. vuoro's config
names a `token_command`, run without a shell, whose stdout is the token, with a
`token_env` variable as the fallback. The owner's command is `ritual-secret get
gitlab-pat`. The token lives in memory only. Two alternatives were rejected:

- an environment variable only, which is visible to whatever launches vuoro;
- the macOS keychain through `security`, which is macOS-only and adds a setup
  step, when a token command can already reach a keychain.

The same ticket settled the rest of plan sections 5.1 and 8:

- The config is JSON, in the platform config directory.
- State is JSON files. The derived cache is kept apart from what the person
  wrote, so deleting the cache loses nothing of theirs.
- `modernc.org/sqlite` waits until a query needs it.

### D30. The turn model adds review requests and pushes, and `since` is the earliest unanswered event

Settled on 2026-10-02 by the agent working under the owner's deputy for the
MVP, from real API responses on all three forges (plan section 4). It is open
to the owner to overrule. Four choices:

- **Two facts join the model.** A review request is a fact of its own,
  because every forge records re-requests. A push is one too, because a
  commit's date is when it was written, not when it arrived.
- **Bots and system notes count for nothing** in the turn rules.
- **A thread's turn is its last real note.** The alternative was a recorded
  "expected responder", which no forge holds and which would be state the
  forge does not.
- **`since` is the earliest unanswered event**, so noise does not reset the
  wait. The alternative was the latest event.

GitLab is read with GraphQL and written with REST. Anonymous REST refuses a
merge request's discussions, and GraphQL returns reviewer states, positions,
and diff versions in one request.

### D31. A verifier marks each finding's confidence, kept findings make a hand-off report, and a reviewer can draft a suggestion

Decided by the owner on 2026-10-02, after the first review of a real merge
request (next-gen-compute!144), and added to the MVP:

- **A verifier rates every finding.** After the synthesizer, or after the
  reviewers when there is none, a read-only verifier session checks each
  finding the person works from against the code. It marks it confirmed,
  plausible, or doubtful (✅, ❔, ⛈), with a one-line reason. The person can
  override any mark. The alternatives were the verifier's mark standing alone,
  and the person marking with no agent pass.
- **Kept findings make a hand-off report** for an agent that fixes them. It is
  Markdown, copied with one button and written to `runs/<id>/report.md`. The
  alternative of writing into the person's checkout was not chosen.
- **A reviewer can draft a suggestion.** When the person asks, the session that
  drafted a finding adds replacement lines, and keeping it puts a GitLab
  ```` ```suggestion ```` block in the comment, which the author can apply. This
  is text that a read-only reviewer writes, so it is not the editor (D17). The
  editor's multi-file patches on others' changes stay in
  TKT-01M3YMSVPTE1QGMV63GCF6HPW1. The alternatives were a patch that rides
  only in the report, and both.

### D32. Vuoro uses violet on the shared Birch and Tar foundation

The owner chose violet on 2026-10-02 after comparing the existing blue with
violet and teal in both schemes. The accent is `#7546A4` on Birch and
`#B89AE3` on Tar, with hover and text-on-accent roles supplied by
`terva-sh/design`. Components keep reading the shared `--ui-*` roles.

Violet gives vuoro its own identity and stays apart from review status
colors. The former blue was an accessible stand-in from git-ticket-canvas,
but lampi, ketju, and git-ticket-canvas already use blue. Teal also passed
contrast checks, but sits closer to Dayroom's green and the shared success
color. The comparison and measured contrast live in
`terva-sh/design/docs/vuoro-accent.md`.

### D33. Bring Forgejo reading and the shared forge boundary into the MVP

On 2026-10-02 the owner selected a shared forge boundary plus Forgejo
open-by-URL support as the next work. This brings the read/review slice forward
from D21's v1.0 schedule: open a PR, read its diff and discussions, fetch its
head and run the existing agent review pipeline. Forgejo discovery, the inbox
and human-directed writes remain separate follow-ups; GitHub still follows
Forgejo. GitLab's live write validation does not block this read-only slice.

All code-host integrations use the neutral forge model and provider contracts.
Agent tools continue using agent.Driver, and git-ticket retains its separate
boundary. A universal external-integration interface was considered but would
combine unrelated operations and permissions. The second forge validates the
contract against actual API and checkout differences before adding discovery.

### D34. The next inbox includes GitLab and Forgejo together

On 2026-10-02 the owner chose “one inbox for GitLab and Forgejo, ordered by
whose turn it is” as the next outcome after the Windows CI fixes. This brings
Forgejo discovery and inbox support forward from D33’s later-work schedule into
the MVP. Forgejo posting remains separate; GitHub still follows Forgejo (D21).

Build a shared, narrow discovery/refresh contract, both forge discovery adapters,
and the pure turn engine, then integrate the inbox with the existing review
workspace. Read-only discovery must not wait for blocked GitLab live-write
validation. The person sees reasons and the age of the earliest unanswered
event; bot/system activity never hands over the turn.

The alternatives offered were completing Forgejo posting first, and adding
more agent review tools and workflows first. The owner selected the inbox,
which delivers Vuoro’s remaining core promise across both forges already read.
Agent tools and ticket stores keep their separate interfaces. No implementation
ticket is promoted merely by recording this execution plan.

### D35. A command-line review joins the MVP, for dogfooding

On 2026-10-05 the owner asked for a way to run a review from the command line
against whatever they name, so that vuoro reviews its own development. This
brings a headless first slice of "reviews before a push" forward from the
v1.0 list (section 10.2) into the MVP. Carrying a local review's findings
into the merge request opened later stays in v1.0.

`vuoro review` takes a git range in a local checkout or a change URL. It runs
a preset, or one profile, through the same runner, drivers, lockdown,
synthesis and verification as the window. It prints the findings as Markdown,
or as one JSON envelope with `-json`. The default preset is `quick` when the
config file defines it. The run is saved like any other, under the same
retention (D26).

The command has no write path to a forge. A headless process has no person in
the loop to post, so "an agent drafts, a person posts" holds here because
nothing posts. A test proves that a run against a recording forge sends no
write. Its drafts MCP server is a loopback server of its own, behind the same
guard, with a per-session credential.

The alternatives were waiting for v1.0, and leaving dogfooding to the
terva-review Forgejo workflow alone. The command complements that workflow: it
reviews before a push, and the workflow reviews the pull request.

### D36. One unreadable change does not fail an account's refresh

On 2026-10-05 the owner's GitLab inbox stayed empty: discovery found 51
changes, but one could not be read, and the plan's rule that "discovery and all
reads must succeed before replacing an account snapshot" discarded the other
fifty. The owner asked for the filed fix, TKT-01M46M51GK4WQT7SZSDFA5BWJA (One
unreadable change blanks an account's whole inbox refresh).

Discovery must still succeed, because complete membership is what makes a
removal safe. Reading each discovered change may fail on its own. A change read
before keeps its last facts, marked with a fixed credential-free reason. One
never read is listed by its reference in a "could not read" group, with no turn,
because nothing about it is known. The snapshot is replaced, and the account
carries a warning that names how many changes could not be read. A timeout or
cancellation of the whole account still keeps the previous snapshot.

The alternative was to keep all-or-nothing snapshots and fix each read failure
as it appears. That hides every other change behind any one forge oddity, and
the inbox is where the person looks first.

### D37. Reviewers draft suggestions with their findings, and asking for one starts a small session

On 2026-10-05 the owner found "Ask for a suggestion" slow, and saw that each
ask resumed the drafting session with its whole context. That context was
about 100k tokens on a real merge request, and an ask took 8 to 18 seconds.
The owner asked for TKT-01M46PWG2Z425KSDH07BTDRNP6 (Draft suggestions with
findings, and ask for one in a small session). This supersedes the part of
D31 that drafts a suggestion only "when the person asks".

- **A reviewer drafts the suggestion with the finding.** On a critical, high,
  or medium finding whose fix is a replacement of the anchored head lines,
  about 20 lines at most, the reviewer adds the replacement while it still
  has the code in mind. It adds none for a low or info finding, or when the
  fix is elsewhere, spans files, or is a matter of judgement.
- **The synthesizer carries it** when its merged anchor covers exactly the
  lines the suggestion replaces, and drops it otherwise. It writes none of
  its own.
- **The verifier checks it.** Its reason ends with "suggestion: …" when
  applying the suggestion would not fix the finding.
- **A suggestion must fit its range.** A suggestion replaces every line of
  its range, so one that repeats the line just before or after the range is
  refused with a rule the agent can correct. The first live run, before
  this rule, drafted two suggestions that would each have left a closing
  brace twice, and the verifier passed one of them.
- **Asking starts a fresh, small session.** It gets the finding and where it
  sits, and reads the file itself. Its drafts role, Suggester, may read that
  one draft and set its suggestion, widening the anchor in the same file at
  most, and nothing else. It does not resume the drafting session.

The alternatives were to keep asking the drafting session, which pays for its
whole context on every ask, and to draft a suggestion on every finding,
which spends output on low findings the person rarely keeps. A suggestion is
still text that a read-only agent writes, and keeping it is still the
person's act, so D17 and "an agent drafts, a person posts" are unchanged.

### D38. The inbox is a list with a detail pane, "Your turn" splits into lanes under a summary, quiet changes fold away, and snooze has three lengths

Decided by the owner on 2026-10-05, after the first real inbox (docs/inbox-design.md)
proved overwhelming as one long list. The owner chose each of the recommended
options:

- **A list and a detail pane.** Each row is two lines: an age badge, the
  title and a short project reference, then the role and one summarised
  reason. Selecting a row fills a pane on the right with what is waiting:
  the threads awaiting a reply, with author, place, and first line, and the
  change's actions. The alternative was a dense single column.
- **A summary strip and lanes.** A strip counts what needs the person by
  kind: changes requested on their changes, ready to merge, replies owed,
  and reviews requested. Each count filters the list. "Your turn" splits
  into lanes in that order: their own changes, then replies owed, then
  reviews. A row sits in the lane of its most pressing reason. Within a lane
  the oldest unanswered comes first, so D30's ordering holds inside each
  lane. The alternative was D30's single list with only the strip.
- **Dormant.** A change with no human activity for 90 days folds into a
  "Dormant" group, whoever's turn it is. Bots, system notes, and the
  change's own updated stamp do not count as activity. The threshold is
  `inbox.dormant_days` in the config. Any new human event brings the change
  back. The alternatives were 30 days and no dormant group.
- **Snooze has three lengths:** until tomorrow morning, for a week, and until
  something changes. The last has no date, and only a new forge event ends
  it. The alternative was keeping the fixed day.

"Their turn" and "Dormant" fold by default. Each reason kind gets one line on
the row, with counts, rather than one line per thread.

### D39. The editor fixes a review's kept findings in a stacked draft merge request, first

On 2026-10-05 the owner asked to give a completed review's hand-off report to
an editor agent. The editor addresses the concerns on a new branch from the
review target, and a merge request goes against the original merge request.
The goal is less friction to get code in. A reviewer who agrees with their
comments can review the fixes and merge them without writing the changes
themselves. One who does not can iterate from the proposal or start from
scratch. This changes three earlier decisions:

- **It brings reviewer-side editing forward.** It supersedes D18's order,
  which put the editor on the person's own change first and others' changes
  after v1.0. This comes first, and the author's side stays in v1.0.
- **The fixes go to a new branch, not the merge request's branch.** It
  supersedes D19's push target for this case. The author's own branch is
  never pushed to. The editor still commits locally, one commit per finding,
  and never holds a credential. vuoro pushes only when the person accepts
  the commits after reviewing them as a diff.
- **vuoro opens a Draft merge request** whose source is the new branch and
  whose target is the original merge request's source branch. Its
  description lists each finding addressed and links the original. The
  person edits the title and description before it opens. The alternatives
  were a ready merge request, and pushing the branch only.

The sandbox comes first. The owner chose to run D20's spike before the
editor is built, so the boundary is proven before an agent runs someone
else's code. The alternatives were starting on Claude Code's own sandbox, or
going straight to a container.

### D40. The editor runs locally under Claude Code's own sandbox, and under `srt` when it has an API key

Decided by the owner on 2026-10-05, from the spike in docs/editor-sandbox.md.
This supersedes D20's container default.

- **By default the editor runs on the local machine** as Claude Code, with
  the person's normal login. It runs with no user settings, hooks, or MCP
  servers. `--restricted` confines its file tools to the worktree, and
  symlinks are resolved before the check. Claude Code's sandbox confines
  every shell command: no reads or writes of the home directory, no
  unsandboxed retry, and no start at all when the sandbox is unavailable.
  The network allows only the package registries the preset names, so
  forges are refused. vuoro builds the environment from an allowlist, so it
  holds no forge token. The spike recorded every probe holding, against
  fake canaries.
- **When the person configures an API key for the editor**, vuoro wraps the
  whole Claude Code process in the sandbox runtime (`srt`). The operating
  system then confines the file tools too, not only shell commands. Under
  Seatbelt the macOS Keychain login is unreachable, so this mode needs a
  credential handed in. With a key there is nothing to work around. The key
  comes from a `token_command` or `token_env`, like a forge token, and only
  the model endpoint is reachable with it. The owner raised this case. The
  spike ran `srt`'s boundary with shell probes, and a run of Claude itself
  under `srt` waits for a key to test with.
- **A container stays a later opt-in.** It isolates files completely, but
  it needs a toolchain image for each project and an egress proxy run by
  vuoro, because podman's network is all or nothing.

The alternatives were D20's container, and `srt` for everyone, which needs
a separate long-lived token from `claude setup-token`.

### D41. On the person's own change, the editor fixes triaged threads and pushes to the change's branch

Decided by the owner on 2026-10-05, after the reviewer-side editor (D39) was
used to hand back terraform!2487. This is D18's author side, and it keeps
D19's push target, which D39 set aside only for other people's changes.

- **The editor fixes the threads the person picks** from a triage run. Must
  fix threads are picked by default, and discuss and nit threads are not. It
  gets each thread's discussion and the triage reason, makes one commit per
  thread, and may decline a thread and say why.
- **vuoro pushes to the change's own branch**, fast-forward only, on the
  person's confirmed action after they review the commits as a diff. A branch
  that moved since triage is refused, never forced. The alternatives were a
  stacked draft merge request as on others' changes, and asking each time.
- **A fixed thread's reply cites its commit.** Once pushed, vuoro sets the
  thread's drafted reply to name the pushed commit, unless the person edited
  the reply. The commit ID comes from vuoro, never from the agent. The reply
  stays a local draft until the person adds it to pending comments and
  submits, and vuoro does not resolve the thread. The alternatives were
  replying and resolving on submit, and leaving replies to the person.
- **Editor fixes come before applying reviewers' suggestion blocks**, which
  needs no agent and stays a later slice.
