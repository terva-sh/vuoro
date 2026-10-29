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

### D42. Where the editor's fixes go depends on whose change it is, and a review of your own work posts nothing

Decided by the owner on 2026-10-05, from the three flows they named: reviewing
somebody else's change, answering review on your own, and reviewing your own
work to fix what it finds, which is how vuoro dogfoods its review loop.

| Change | What the person does with an item | Where the fixes go |
|---|---|---|
| Somebody else's merge request | Keeps a finding as a comment | A stacked draft merge request (D39) |
| Their own merge request, threads from reviewers | Picks a thread; its reply cites the fix | The merge request's branch (D41) |
| Their own merge request, reviewed by vuoro | Picks a finding to fix, or dismisses it | The merge request's branch (D41) |
| A local range, before any merge request | Picks a finding to fix, or dismisses it | A new branch in their own repository |

- **A review of the person's own merge request posts nothing.** Keep is
  refused there, and a finding is picked for the editor or dismissed
  locally. The alternatives were picking with an optional post, and posting
  as on others' changes.
- **A local range's fixes become a new branch, `vuoro/fix-<run>`, in the
  person's own repository.** vuoro writes the branch's ref and nothing else:
  not the checkout, the working tree, or HEAD. It never overwrites a branch
  that exists, and the repository's hooks do not run. The person merges or
  cherry-picks it. The alternatives were a patch file, and uncommitted
  changes in the working tree, which collide with the person's own work.
- **Reviewing the fixed head again comes next**, as its own slice, so the
  loop closes with the findings the fixes resolved.

### D43. Forgejo drafts stay in vuoro until Submit, which publishes the review in one call

Decided by the owner on 2026-10-06, from the Forgejo write spike,
TKT-01M47EG9BHW0HBVFBTSBB1915X (Prove which Forgejo review writes round-trip
against a sandbox PR). docs/plan.md section 5.3 records the evidence.

- **Drafts are local until Submit.** Forgejo's issue timeline shows a pending
  review's body to every reader of the repository, and keeps each superseded
  body after submit. A pending review also refuses an empty body. So Submit
  creates and publishes the review in one call, with the verdict, the joined
  general drafts as its body, and every inline comment. The alternatives were
  mirroring GitLab with a forge-side pending review, and a placeholder body
  replaced at submit. Both leak.
- **Replies post into the thread's own review**, at its path and position,
  after the review. Each publishes by itself, so a partial failure is reported
  for each reply. A comment at the same line in a new review was rejected,
  because Forgejo shows it as a separate conversation.
- **Withdrawing an approval is unsupported on Forgejo.** A reviewer cannot
  dismiss their own approval, and dismissal is an admin's action on someone
  else's review. Deleting the published review was rejected because it erases
  the record, and the forge is the record.
- **Resolve stays hidden**, because the API has no route for it.
- **A suggestion posts as a plain fenced replacement under a line that says
  so**, because Forgejo cannot apply one. The alternative, hiding suggestions
  on Forgejo, discards a finding's most useful part.

### D44. Submit carries the person's verdict, and capabilities decide every write control

Decided by the owner on 2026-10-06, choosing the Forgejo parity plan of
TKT-01M47EFQ400T20P79H9X152NAZ (Forgejo parity: posting, verdicts, and the
editor's push). docs/plan.md section 5 records the boundary.

- **`Submit` takes a verdict: commented, approved, or changes requested.**
  Forgejo publishes a review with its verdict in one call (D43), and GitLab
  takes `reviewer_state=requested_changes` with the same batch. "Submit and
  approve" moves into the adapter for the same reason. A separate
  `RequestChanges` call was rejected, because the verdict and the comments it
  explains would then land as two events. So was keeping approve as a second
  call made by the application, because Forgejo cannot approve that way
  without publishing a second review.
- **Capabilities name every write a forge may lack.** `reply`, `unapprove`,
  `request_changes` and `suggestions` join `drafts`, `resolve` and `approve`.
  The UI shows a control only when its flag is set, and the application refuses
  the write otherwise. Deciding by forge kind in the UI was rejected: a forge's
  capabilities change with its credentials and its version, not with its
  name.

### D45. Setup in the window chooses where a token is kept, and never takes the token

Decided by the owner on 2026-10-06, for TKT-01M3YMSVJ2BTQN940P1XR3CJD6 (Set up
forges and check agent tools in the window). D29 stands unchanged.

- **The window helps the person choose a token source.** It offers the stores
  this machine has: the macOS Keychain, the Secret Service, pass, 1Password,
  an environment variable, or a command of their own. It shows the command
  that stores the token in that source, for the person to run in a terminal,
  and writes only the command or the variable that reads it back into
  `config.json`. A Test button resolves the token from the source and asks the
  forge whose it is, and a forge saves only after a test that names the
  person's account.
- **Rejected: pasting the token for vuoro to store in the OS keychain.** It
  needs a store vuoro can write on every platform without cgo, and WSL usually
  has no Secret Service, so it would still need this form as a fallback. It
  would also reopen D29's choice to keep vuoro out of the token's storage.
- **A save does not overwrite a hand edit.** Setup writes the file a person
  could also edit by hand. It refuses to save over a file that changed or no
  longer loads since the window read it, and it keeps every field the form
  does not show. Saves within vuoro are serialized. An edit by another program
  in the instant between the check and the write cannot be detected without
  file locking, which the platforms do not share.
- **Presets stay hand-written for now.** The first slice covers forges and the
  agent-tool check. Building presets in the window is
  TKT-01M48TMGNVS4303NW7NQW26Y6S (Build reviewer presets in the window).

### D46. Vuoro keeps the terva-sh colour scheme, and diffs use its status roles

Decided by the owner on 2026-10-06, after a contrast pass on the diff and the
editor's panel. The owner confirmed D32's violet on the shared Birch and Tar
foundation, and chose to keep every colour inside the terva-sh scheme.

- **Diffs use the shared status roles.** An added line takes `--ui-ok` and a
  removed one `--ui-danger`, as a stronger tint, a gutter bar, and a bold
  sign. Each row carries its `+` or `−` sign and each file its added and
  removed counts, so the colour is never the only signal.
- **A new colour that means something comes from the scheme.** Components
  read the `--ui-*` roles from `terva-sh/design`, and a colour the scheme
  lacks is proposed there rather than added here. The inbox's age badge, whose
  blue, amber, and magenta ramp docs/plan.md section 5 specifies, predates
  this and stays until the scheme offers a ramp.

The alternatives were blue for added and amber for removed, which stay apart
under every common colour blindness but sit outside the terva-sh palette, and
`--ui-ok` with `--ui-warn`, which was rejected because amber already means a
warning elsewhere in the UI.

### D47. The home page gains a Local tab of the person's checkouts, one row per branch

Decided by the owner on 2026-10-06, after a review of the landing page found
it forge-only: a local range could be reviewed only from the command line.
docs/plan.md section 6.8 records the design.

- **Two tabs, "Reviews" and "Local",** each with a count of what needs the
  person, and the last one used remembered. The alternatives were a local
  section under the inbox, which puts local work below the fold, and a side
  column, which squeezes the inbox's list and detail.
- **Projects are folders the person adds, and roots vuoro scans.** Adding uses
  the native directory chooser. A scan is bounded in depth, count, and time,
  and runs only when asked. Listing only the projects already reviewed was
  rejected, because the first review could not then start from the window.
- **A row is a branch with commits beyond its base, and a worktree is a tag
  on the branch it has checked out.** Reviews are of commits on a branch, so
  the row is what gets reviewed. Rows of worktrees were rejected, because a
  branch with no checkout would not show, and so was listing both, as too
  busy.
- **A local review covers commits only,** as the command line's does (D35).
  Reviewing uncommitted work was rejected for now, since the runner would
  have to capture the working tree into its own clone.

### D48. A local branch with no commit in 14 days is quiet, and counts only for what the person left open

Decided by the owner on 2026-10-06, after the first build of the Local tab
counted 118 branches as needing the person on their workspace, nearly all of
them never reviewed and long abandoned. docs/plan.md section 6.8 records the
rule.

- **Time decides, by the branch tip's committer date.** A branch with no
  commit in 14 days is quiet. It stays a row, but being never reviewed or
  having moved since its review stops counting. On the owner's workspace,
  127 unmerged branches across its repositories came to 15 counted at 7
  days, 27 at 14, 31 at 30, and 69 at 60. Fourteen days is a sprint's length
  and leaves room for a branch set down for a week.
- **What the person left open in vuoro counts at any age:** undecided
  findings and an editor's fix branch waiting to be merged. Each clears when
  the person acts, so it cannot pile up as abandoned branches do.
- **The alternatives** were counting only branches checked out in a
  worktree, which misses work on a branch switched away from in the main
  checkout, and counting only branches vuoro has reviewed before, which
  hides the first review the tab exists to start.

### D49. A local branch is tied to its open change through the inbox's cache, and offers both reviews

Decided by the owner on 2026-10-06, after adding a checkout whose branch had
an open merge request on GitLab that the Local tab did not show.
docs/plan.md section 6.8 records the design.

- **The tie comes from the inbox's cache, not a new forge call.** A branch is
  tied to an open change when the change's host and project are one of the
  checkout's remotes and its source branch is the branch's upstream there.
  The cache holds every change the person authors, reviews, is assigned, or
  takes part in. The alternative, asking the forge on Refresh for each
  pushed branch the cache did not tie, finds someone else's change checked
  out to try, at one API call per such branch, and was rejected for now.
- **A tied row offers "Review the change" and "Review local commits" side by
  side.** The first opens the change's page, as the inbox does, where
  findings become drafts the person can post. The second reviews the local
  branch, unpushed commits included, whose findings cannot be posted. The
  alternatives were to make the change's review the default and offer local
  commits only when some are unpushed, and to link the change while Review
  always ran locally.
- **A tied branch counts in Local only for work its change does not hold:**
  commits not pushed, undecided findings in a local run, and a fix branch
  waiting to be merged. Its review is counted in the Reviews tab, and
  counting it in both was rejected.

### D50. A review is fixed in waves, reviewed again with its earlier findings, and records each fix's commit

Decided by the owner on 2026-10-07, after fixing findings on their own merge
request: the editor's push moved the change, after which no second editor
could start on the review and a new review would have lost the earlier
decisions; and two fixes whose commits lacked the `Vuoro-Finding` trailer
stayed undecided and showed as reviewed two commits behind. docs/plan.md
section 6.3 records the design.

- **A new fix wave starts while only vuoro moved the change.** Every commit
  since the review must be one vuoro pushed for it; the new editor starts
  from the last pushed head on the same branch. The alternative, starting
  from the change's current head after anyone's push, was rejected because
  findings could then point at lines that have changed.
- **Reviewing again carries the earlier findings and their decisions.**
  Reviewers see them with where each stands and report what is new or still
  unfixed; a restated finding keeps the person's decision. A review linked
  to the earlier one but starting fresh was rejected, since it raises
  dismissed findings again.
- **A fix is linked by its trailer, then by asking the editor, then by
  hand.** Before a push, vuoro asks the editor about a picked finding no
  commit names. Leaving a missing trailer only to the person was rejected as
  routine busywork after every wave.
- **Fixed findings fold into a "Fixed" group** showing their commit, rather
  than staying in place with a badge, so the open ones are what the page
  shows first.

### D51. vuoro saves a patch of each fix it pushes and of each fix linked by hand, for as long as the run

Decided by the owner on 2026-10-07, after D50's fix links landed: a fix was
recorded as a commit's SHA and subject, and its diff lived only in git, in
the editor's repository that closing the run drops, and on a branch that
goes when it is merged or deleted. docs/plan.md section 6.3, "A fix keeps
its patch", records the design.

- **Patches are saved at each push.** What the person accepted and pushed is
  what is kept. Saving discarded waves too was rejected, since Discard
  promises to delete the work. Saving at the end of every editor turn was
  rejected as history the person rarely needs, most of it revised before
  the push.
- **Fixes linked by hand get a patch too**, when vuoro's cache has the
  commit, so every "fixed in" can show its diff. Saving only the editor's
  commits was rejected.
- **A patch lasts as long as the run record**, with a size cap per patch.
  Dropping a patch once its commit is merged, since the forge then holds it,
  was rejected: merged history would then need the forge, and a local fix
  has no forge.

### D52. vuoro is tested on linux, on the internal runners, and GitHub runs no CI

Decided by the owner on 2026-10-08, when GitHub Actions stopped starting jobs
for a billing problem on the account. `.github/workflows/ci.yml` ran `just
ci`'s steps on ubuntu, macOS and Windows hosted runners, with the race detector
and the window check on macOS. The Forgejo workflow, on the internal runners,
is now the only CI. It tests on linux, and vets and cross-builds darwin and
windows. docs/plan.md section 9 and docs/pr-reviews.md say so.

- **The GitHub workflow is removed** rather than left failing, since a red
  run on every push says nothing about the code. Keeping it until the billing
  is fixed was rejected, for the same reason.
- **macOS and Windows are not tested in CI.** That costs what the hosted legs
  found: Windows-only failures, such as the temp directories in
  TKT-01M47EFQ8M7TDAGN19F7RVZ2G6, the race detector, which works without cgo
  only on darwin (D53 brings it back on linux), and the window check on macOS. Those now run only where
  someone runs `just ci` or `just test-gui` on that platform. Testing them
  again comes back with publishing (TKT-01M3YMSVN79DEXW47ZXB519KVE).
- **go.mod's 1.26 minimum is not tested.** The internal image is Go 1.27, and
  only the GitHub legs took Go from go.mod.

### D53. The tests also run under the race detector, built with cgo, in the gate and in CI

Decided by the owner on 2026-10-08, with D52. The race detector without cgo
works only on darwin, so it ran only on GitHub's macOS leg, which D52 removed.
`just race` runs the tests with `-race`, building them with cgo off darwin,
and `just ci` and the Forgejo workflow run it after the plain test run.

- **It builds tests only.** The rule against cgo is about what ships, and
  the release build, the cross builds and the plain test run all stay
  `CGO_ENABLED=0`. The race run tests a cgo build of the same code, which can
  differ in the standard library's resolvers, so it is an extra run and does
  not replace the plain one.
- **`just ci` needs a C compiler off darwin**, gcc on linux, and says so when
  one is missing. Leaving the race run out of `just ci`, in CI only, was
  rejected: the gate and CI run the same steps.
- **Its first run found a real bug.** `Manager.Shutdown` returned before the
  sessions' goroutines finished, so a run's record could be written after it
  returned, and a session that started during it was left running.
  TKT-01M4CPAZX0WNTCCV7R3HJ5MAJP has the evidence and the fix.

### D54. GitHub is read with a token through GraphQL, and its drafts are a GitHub pending review

Decided by the owner on 2026-10-08, at the start of
TKT-01M3W4HX30MB0D601R3S0ZXSER (GitHub adapter for the forge interface).
docs/plan.md section 5.4 records the design.

- **A GitHub change needs a configured account to open.** GitHub answers
  anonymous GraphQL with 403, checked on 2026-10-08. Reading with REST so that
  public pull requests open anonymously was rejected. REST has no thread
  resolution, so turn rule A1 would count resolved threads, and a change
  needs several REST reads where one GraphQL query does.
- **Drafts are kept in a GitHub pending review**, as GitLab keeps draft
  notes. GitHub shows a pending review only to its author, so D43's reason for
  keeping Forgejo drafts in vuoro does not hold here. The drafts show in
  GitHub's own window and on another machine, and a reply can go inside the
  review. Keeping them in vuoro and sending one REST review at submit was
  rejected. That call cannot carry replies, so they would post separately, and
  a pending review started in GitHub's window would block the submit. GitHub
  allows one pending review per person per pull request, so vuoro adopts the
  one the person started.
- **Live write tests use a private sandbox repository under the owner's
  account.** Approving and requesting changes need a second account, so those
  stay tested from fixtures until there is one.

### D55. vuoro hides to the tray, refreshes every 15 minutes by default, and nudges once per refresh

Decided by the owner on 2026-10-08, at the start of
TKT-01M3YMSVBDTFZ17PS86GXAC4ZK (Tray, background refresh, and quiet turn
notifications). It carries out D24. docs/plan.md section 3.3 records the design.

- **Closing the window hides vuoro to the tray.** The process keeps running
  and refreshing. The tray menu opens the window again and quits. On a desktop
  with no tray host, such as GNOME without the AppIndicator extension, closing
  the window quits as it always has, so the window can never be lost. Asking
  on the first close, and quitting on close with the tray only while the
  window is open, were rejected.
- **The background refresh runs every 15 minutes by default, and the person
  sets it.** It is `inbox.refresh_minutes` in the config, and the window edits
  it. Every 5 minutes and every 2 minutes were rejected as defaults: a turn
  can wait a quarter hour, and the forges see fewer calls.
- **Turn notifications are on by default and batched.** A refresh sends at
  most one notification, at low urgency. It names the change and why it is
  the person's turn, or gives a count when several changes turned at once.
  `inbox.notify` turns them off. Off until enabled, and one notification per
  change, were rejected.
- **Start at login is off, with a toggle in the window.** Nothing is written
  to the login items until the person asks. On by default was rejected.

### D56. vuoro's mark is a white v with an amber dot on a violet tile, and it writes a desktop entry wherever it runs

Decided by the owner on 2026-10-09, under TKT-01M4EHCC0H9KTWM09ZSW1NDD88 (Give
vuoro its own mark on the window, taskbar, and tray). docs/plan.md section 3.3
records the design.

- **The mark is a white lowercase v on a violet tile, with an amber dot at
  the tip of its right arm.** The v names vuoro, and the dot is the turn that
  is the person's. The violet is vuoro's accent from terva-sh/design, and the
  amber is the design's `hot` brand pigment. The window, the taskbar, the
  dock, the tray, and the notifications all show it. Five other concepts were
  rejected: an open ring with an arrow and two arrows passing a turn, which
  read as refresh and sync; a turn travelling between two dots, which blurs
  at tray size; the v inside a speech bubble, which is busier at tray size;
  and a stack of inbox cards, which says nothing about review.
- **The mark is drawn at launch, not shipped as an image.** It is a few
  shapes, so drawing it costs a few milliseconds, and there is no file to fall
  out of step with the code.
- **vuoro has tuohi write a desktop entry wherever it runs.** Wayland has no
  per-window icon, and a GTK3 window there finds its icon only through a
  desktop entry matching its name. So vuoro writes a hidden entry and a
  themed copy of the mark under the person's data directory, by default
  `~/.local/share`. tuohi leaves alone an entry it did not write, such as one
  a package installs. Writing nothing, and keeping tuohi's mark under
  Wayland, was rejected.

### D57. Setup places a token on a gauge from what the forge reports, and never writes to learn it

The owner asked on 2026-10-09, under TKT-01M4EP0RRT9QA7ZQQKQA30E6K4 (Show on a gauge how far a forge
token reaches), for setup to show how far a token reaches on a gauge, with
read only and full in the green and administration in the red. The owner
invited design opinions, and the agent proposed the rest. It stands until
the owner says otherwise.

- **The gauge has four zones: can't read, read only, full, and too broad.**
  Read only is fine: vuoro reviews, and the person posts by hand. Full also
  posts what the person sends. Too broad holds scopes or roles beyond
  reviewing, such as administration, deleting repositories, organization
  writes, or Actions workflows. A token the forge does not describe gets no
  reading, and the gauge says what to check instead.
- **Only what the forge reports, and reads, place the token.** GitHub's
  `X-OAuth-Scopes` header describes a classic token. GitLab's
  `/personal_access_tokens/self` describes a personal access token, and
  `is_admin` marks an administrator's `api` token as broad. Forgejo lists
  a person's tokens, with their scopes, to a token holding `read:user`, and
  vuoro finds its own by the last eight characters. Where that list is
  refused, reads of a repository that cannot exist show which read scopes
  are missing, because Forgejo checks scope before it looks for the
  repository. vuoro never sends a write to see whether a write would
  succeed, so posting rights a forge does not report stay unknown. Probing
  with a write that fails was rejected: it is a write to the person's forge
  that the person did not ask for.
- **Can't read blocks the save. Too broad warns and saves.** A token that
  cannot read reviews is no use to vuoro. A broad one works, and the person
  may have reasons for it, so the gauge names the extra scopes and suggests
  a narrower token. Refusing broad tokens was rejected.
- **GitHub's fine-grained tokens are unknown.** GitHub gives an application
  no way to read their permissions, so the gauge says which to grant.

### D58. On macOS the menu bar shows vuoro's mark without its name, and the Dock icon is the person's choice

The owner reported on 2026-10-09, under TKT-01M4HAZD5QSQZRN4ZBGHE3NZ7A (Drop
the name from the menu bar, and let the person keep the Dock icon), that on
macOS, with vuoro in the menu bar, vuoro had no Dock icon, and that the name
beside the mark took a lot of room in the menu bar. They asked for the name to
go, and for the Dock to be a setting, even one that waits for a restart.

- **The menu bar shows the mark alone.** tuohi's tray prints its `Title` beside
  the icon on macOS, and vuoro no longer sets one. The tooltip still names
  vuoro, and the menu's first line names its version.
- **"Keep vuoro in the Dock as well as the menu bar" is a setting, off by
  default.** tuohi's tray makes an application a menu-bar app, with no Dock
  icon and no place in the application switcher, unless `tray.Config.Dock` is
  set (tuohi TKT-01M4HAEK8VHG2EY13RCNWVTC06, Let a tray application keep its
  macOS Dock icon). The choice is `desktop.dock` in the configuration file, and
  vuoro reads it as it starts, so the settings page says when the saved choice
  waits for a restart. Off keeps what vuoro did before. A window with no Dock
  sends no choice, so a configuration file shared with a Mac keeps the Mac's.
  This supersedes the Dock half of D56, which assumed the Dock always showed
  the mark.
- **A Dock icon only while the window is open was deferred.** It needs tuohi to
  switch the activation policy as the window shows and hides, which macOS
  applies with visible flicker. A Dock icon always present, with no choice, was
  rejected, because a closed vuoro would keep a Dock slot.

### D59. The editor's fixes can go back as suggestions on the person's pending comments, instead of a stacked change

Decided by the owner on 2026-10-08. The author of a change being reviewed
asked for inline suggestions instead of a stacked merge request (D39), so
they could see which changes were made to their own. docs/plan.md section
6.3, "Fixes as suggestions", records the design.

- **One editor, two ways back.** The editor runs as D39 says, in its
  sandbox, one commit per kept finding. At the end, its commits are either
  pushed as a stacked draft change or turned into suggestion blocks on the
  person's pending comments. A second agent that writes suggestion text
  without building or testing was rejected. The editor can run the tests,
  and both paths share one sandbox and one review of the diff.
- **Chosen at the start, switchable at delivery.** The choice changes the
  editor's instructions: for suggestions, keep each fix to existing lines
  of the files the change touches, and add, delete, or rename no file. vuoro
  remembers the choice for each change author, so the next change by that
  author starts with it. The person can still deliver the other way after
  reading the commits. A wave is delivered once, in one way. Deciding only
  at delivery was rejected, because fixes written for a branch often do not
  fit as suggestions.
- **A fix that cannot be a suggestion goes in the comment as a plain
  diff.** Examples are a new, deleted, or renamed file, a file the change
  does not touch, or lines the comment cannot reach. The diff is labelled
  as not applicable, with the reason, and the person can edit or drop it
  before submitting. Over a size cap, the diff is not inlined. On a forge
  that takes uploads (GitLab), the fix's saved patch (D51) is attached to
  the comment as a file. Over the patch cap, or on a forge without uploads
  (GitHub), the comment says only that the fix was too large, never the
  content. Pushing the misfits as a separate stacked change was rejected,
  since it needs vuoro to split the editor's commits.
- **A suggested fix is detected on review again.** A finding whose fix was
  suggested shows as suggested, with its comment. When the change moves,
  vuoro checks whether the new head has the suggested lines in place, with
  the lines around them. If it does, vuoro records the commit that brought
  them as the finding's fix, linked by the suggestion. Marking it fixed by
  hand still works. Relying only on the person to mark it was rejected.
- **GitLab and GitHub.** Both apply suggestion blocks, and GitHub takes a
  multi-line suggestion on a comment that spans the lines it replaces.
  Forgejo cannot apply one, so the choice is not offered there.
- **Nothing is posted.** Delivering updates and adds the person's pending
  comments only, on the person's confirmed action. The person submits the
  review as before.

### D60. vuoro explains a login it cannot see, and never recovers one itself

Reported by the owner on 2026-10-17, under TKT-01M55VRM16QASJ8CER793CR9NF
(Find the agent tool login when vuoro starts from the Dock). The setup screen
called Claude Code signed out. The probe was reading the environment of the
vuoro process, which a Dock launch, a Finder launch, and the login item of
D55 all get from launchd rather than from the login shell. A key exported in
`.zshrc` is invisible to all three, so the tool works in a terminal and
reports nothing to vuoro. `editorEnv` reads that same environment, so a review
started in that state would have failed part-way through with
`ErrUnauthenticated`.

- **vuoro does not recover the credential.** Claude Code already holds a key
  that survives any launch, in an `env` block or an `apiKeyHelper` in its own
  `settings.json`, and vuoro passes both through untouched, since it writes
  its MCP configuration to a scratch directory, names it with `--mcp-config`,
  and never sets `CLAUDE_CONFIG_DIR`. Measured against 2.1.288 with an empty
  environment: both report `loggedIn true`. The person has a real fix, and
  vuoro's failing was never to mention it.
- **The hint is evidence, not a guess.** When a probe comes back signed out,
  and only then, the driver asks the login shell whether any credential
  variable is set, never what it holds, with `$SHELL -lic` and a marker it
  matches in the output. No value crosses into vuoro, so there is none to
  leak into a log, an error, or the setup JSON. A shell that fails, blocks
  past its timeout, refuses the flags, or is not named leaves the answer
  false and the hint as it was, because absence of evidence is not evidence.
- **The state does not change, the wording does.** The tool is still unusable
  in this process, so it stays `signed_out`. The hint stops telling this
  person to run `/login`, which would have replaced a working login with
  another one, and names the launch environment as the cause instead.
- **Reading the shell's environment and using it was rejected.** It makes a
  Dock launch work with no action by the person, and it costs running their
  profile at startup and holding `ANTHROPIC_API_KEY` in vuoro's memory, to
  re-solve a problem the tool solved properly. Naming the variables in
  vuoro's config, as a forge token does with `token_command`, was rejected
  for asking someone who has configured Claude Code to configure it twice.
- **A smaller fix rode along.** `authMethod: oauth_token` is what a custom
  endpoint reached with `ANTHROPIC_AUTH_TOKEN` reports, as well as a Claude
  token, and the setup screen called both "a Claude token". It now says "a
  token", claiming no more than Claude Code knows.

### D61. vuoro ships quick, standard, and thorough, and a person's presets supplement them

Decided by the owner on 2026-10-17, under TKT-01M55VT3H26HX25TB8864NPG6J
(Ship named built-in presets, not one hardcoded default). `presetsOf` returned
one unnamed preset, a single code reviewer, when the config named none. So
anyone who had not hand-written JSON got one reviewer, the picker had nothing
to pick between, and synthesis and verification were built but unreachable.
D14 left the shipped set open, and this fills it in.

- **Three ship, and they differ in what they are for.** `quick` is one code
  reviewer, which is what the old fallback did, now with a name. `standard`
  is code and security. `thorough` is code, security, and documentation, and
  it is the only one that reaches verification (D31) without the person
  writing any config. None of them names a synthesizer or a verifier,
  because a run gets a synthesizer above one reviewer (D11) and a verifier
  always, on the tool and model that drafted or merged the findings. Naming
  them here would pin a tool those defaults otherwise pick up.
- **A person's presets supplement the shipped ones**, as their profiles
  already do at `review/run.go`. One that takes a shipped name shadows it
  rather than appearing twice, so theirs wins and an improvement to a
  shipped preset never quietly changes a run they defined. Replacing, which
  is what presets did before, was rejected: someone who adds one preset
  should not silently lose three, and the asymmetry with profiles read as an
  accident rather than a choice.
- **Theirs sort first, and that is load-bearing.** The window's default is
  the first preset, so putting theirs first keeps it the one they wrote.
- **The command line keeps matching `quick` against their own presets.**
  `vuoro review` with no `-preset` prefers a preset named `quick` when the
  config file names one, and otherwise takes the first preset. That test
  must read their presets alone and never the merged list, because vuoro now
  ships a `quick`: against the merged list, someone whose config names other
  presets would silently start running the shipped one. With no config file
  the first preset is `quick` anyway, so the command line and the window
  agree there, which was the point.
- **A preset named `quick` in a config still diverges between the two.** The
  window runs that person's first preset and the command line runs their
  `quick`. That predates this decision, and it is left alone deliberately,
  because changing it would change what an existing config does. Settling it
  is a decision of its own.
- **Editing a shipped preset copies it first.** The editor
  (TKT-01M48TMGNVS4303NW7NQW26Y6S) offers copy on a shipped preset and edit
  on the person's own, so the shipped set keeps its meaning across an
  upgrade. Editing in place would pin a private fork of `thorough` at the
  version they first touched it.

### D62. The presets page reports a name it cannot resolve, and never refuses it

Decided by the owner on 2026-10-18, under TKT-01M561J4A1AB61CS7FP6J6TJDD
(Warn on the presets page when a profile or tool no longer resolves). A
preset names a profile and may name a tool. Neither was checked until
`Manager.Start`, so a preset that had gone stale failed on the way into a
review rather than on the page that writes it.

- **The page says what does not resolve. It does not refuse it.** A preset
  naming a tool absent from this machine is still correct on the machine
  that has it, and a person may run vuoro on more than one. Refusing the
  save would also make the page unable to express a configuration they wrote
  by hand, which turns the editor into a downgrade from a text editor. So
  `canSave` does not consider this, and a save round-trips the name
  untouched. This is the rule most likely to be "fixed" later by somebody
  adding validation, and a test holds it.
- **A signed-out tool is a softer mark than an absent one, because the
  failures differ.** `probeTools` offers a driver to the run manager only
  when its probe returned no error, and `toolState` calls a probe with no
  account `signed_out` with a nil error. So missing, too old, and broken
  never reach `Manager.Drivers` and the run is refused before it begins,
  while a signed-out tool is offered, the run starts, and the session fails
  at authentication. The page words them differently rather than flattening
  both into one warning.
- **The selects stopped lying.** A `select` whose value matches no option
  does not show that value, so a preset naming a profile that is gone read
  as some other profile. The preset still held the real name, which is the
  bad half: the person confirmed what the form showed them and saved
  something else. Each select now renders an option for the value it holds.
  A tool that cannot run is still offered, marked with why, so pinning one
  for another machine stays possible and stays deliberate.
- **Rejected: validating this in `config.Validate`.** It is the obvious
  home and it is the wrong one. The check runs on load, on every machine,
  and a config that names a tool installed on one of them would stop
  loading on the others. The resolution is a fact about this machine, so it
  belongs where the machine is known.
- **Found and not fixed here.** `Manager.driver` resolves an empty tool name
  to the only registered driver, and only when exactly one is registered.
  vuoro registers claude alone today, so it works. The second driver breaks
  every preset that pins no tool. Filed as
  TKT-01M562ESB0WGCRWJJW0QYTBZE6, because it is a different fix in a
  different package and the rule that picks between two tools is a decision
  of its own.

### D63. A reviewer that names no tool runs on a signed-in one, then the first by name

Decided on 2026-10-18 under TKT-01M562ESB0WGCRWJJW0QYTBZE6 (A preset with no
tool pinned breaks when a second agent tool exists). The owner delegated this
one rather than choosing it.

`Manager.driver` resolved an empty tool name to "the only driver", and only
while exactly one was registered. vuoro registers claude alone today, so it
worked. The second driver would have broken every preset that pins no tool at
once: all three shipped ones, and anything the presets page calls "Any tool
available".

- **An empty tool means the person did not care, so vuoro picks.** It is not
  a gap in their configuration to be refused. It is the common case, and the
  answer has to be an actual tool.
- **The pick is stable, and that matters more than which tool wins.** Ranging
  the map would run the same preset on a different tool each start, with
  nothing in the window to say why. Go randomizes that order, so the bug
  would have looked intermittent.
- **A signed-in tool beats a signed-out one.** `probeTools` offers a driver
  whenever its probe returned no error, and a tool with no account probes
  clean (D62), so a signed-out tool is a real candidate that would start a
  run and then die at authentication. Between tools that are equal on this,
  the first by name wins.
- **No hand-ranked list of tools.** vuoro has no basis for an opinion about
  whether claude reviews better than terva, and a ranking written into the
  code would be a preference nobody chose, sitting where nobody looks for
  it. Sorting by name is arbitrary and says so.
- **`app` decides and `review` obeys.** Only `app` runs the probes, so only
  `app` knows which tools are signed in. `SetDrivers` takes the drivers and
  the pick in one call, because a pick naming a driver that is no longer
  registered is worse than no pick at all.
- **Rejected: a `default_tool` key in config.json.** Someone who cares which
  tool runs already has a place to say so, the preset's own `tool` field,
  which the presets page makes a select (D62). A second place to express the
  same preference needs validation, a view field, and a settings control
  before it is worth having, and it would then disagree with the first one.
- **The reload path needed nothing, for a reason worth keeping.** `Start`
  records the tool it resolved, not the name the preset held, so a session on
  disk always names a concrete tool and the reload never picks again. A test
  pins it, because the day somebody records `rv.Tool` instead, every run with
  no tool pinned stops coming back after a restart.
- **The failure now reads as what it is.** With nothing to pick, the error is
  `no agent tool is available to run a reviewer that names none`, not
  `no agent tool named "" is available`, which blamed the preset for naming
  the empty string.

### D64. A project view triages every open change in one project, by where it sits

Decided by the owner on 2026-10-27, in a design conversation before any ticket
existed. The inbox (D34, D38) answers what is waiting on the person. A tech
lead who handles a project's incoming changes asks a different question: what
is open here, where is each one stuck, and for how long. Nothing in vuoro
answered it, because discovery only finds changes the person authors, reviews,
or is assigned to.

- **The person points vuoro at a project, and sees every open change in it.**
  They pick the project from a configured forge account by searching, or paste
  its URL. Membership is the project's, not the person's, so a change nobody
  has looked at is a row.
- **The view is ordered by where a change sits and for how long, not by whose
  turn it is.** Where it sits is derived from the forge's facts: a draft, no
  reviewers, waiting on reviewers, changes requested, waiting on the author,
  approved and not merged, or blocked. Each row says whether it has reviewers,
  whether it has discussion, whether that discussion is answered, and whether
  it belongs to a stack. Turn (section 4) still shows on a row, and it does not
  drive the order. Reusing the inbox's order was rejected because a change with
  no reviewers is nobody's turn, and that is the change a tech lead most needs
  to see.
- **It serves one person running a project, in their own name.** That is still
  section 2.2's one person on one machine. Who a change should go to is a
  judgement about teammates, and it is the person's judgement, made with their
  own credentials.
- **GitLab first, then GitHub and Forgejo.** GitLab is the owner's day job.
  GitHub and Forgejo get the same view through the same contracts, as tickets
  of their own.
- **It widens v1.0.0.** The owner needs it now, so it joins the v1.0.0 scope
  (section 10.2) rather than waiting until after the release.
- **Rejected: Gitea.** The owner runs none, and named it only alongside
  Forgejo. Forgejo's adapter may work against a Gitea host by accident, and
  vuoro does not claim it.

### D65. vuoro sets reviewers, assignees, labels, and draft state, and links out to merge

Decided by the owner on 2026-10-27, with D64. Section 2.1 kept settings in the
forge, and `forge.Writer` covers review actions only. Unblocking a project's
changes is mostly administrative, and a view that shows a change has no
reviewers and cannot give it one sends the person back to the forge for the one
action they came for.

- **Four administrative writes.** Set and remove reviewers, set and remove
  assignees, add and remove labels, and mark a change draft or ready. They are
  a new optional contract beside `forge.Writer`, with their own capability
  flags, so each forge adds them on its own schedule, and D44 still applies: a
  write whose flag is unset is refused before the adapter is called.
- **Merging links out to the forge, for now.** Pipelines, merge trains and
  merge settings are still the forge's. The owner will revisit it.
- **The agent drafts proposals, and the person accepts them.** A read-only
  triage session suggests reviewers, labels and what would unblock each
  change. Its proposals are drafts in the person's own view, and nothing
  reaches the forge until the person accepts them, singly or several at once
  after one confirmation. This is "an agent drafts, a person posts" applied to
  administrative writes, and it is not a new exception.
- **An accepted proposal is carried out by vuoro's code, not by an agent.**
  Accepting turns a proposal into a fixed list of writes, such as add this
  reviewer and remove that label, and `forge.Administrator` performs them. No
  agent session runs during the apply, and none holds a forge credential.
  Handing accepted proposals back to an agent to carry out was rejected: the
  work is mechanical, and an agent in the loop would be one more thing that
  could write something the person did not accept.
- **A write sends additions and removals, never a replacement.** GitLab's
  GraphQL mutations for reviewers, assignees and labels take an
  `operationMode` of `APPEND` or `REMOVE`, so a write cannot drop a reviewer a
  colleague added a minute earlier. Replacing the whole set from what the
  person last saw was rejected for exactly that. A proposal whose change has
  moved since the agent read it, such as a new push or a reviewer already
  added, is marked stale and is not applied until the person looks again.

### D66. vuoro updates a change from its target, by merge or rebase, and the editor resolves only conflicts

Decided by the owner on 2026-10-27. A change that conflicts with its target, or
has fallen behind it, is blocked until somebody brings the target in. Doing
that by hand means a checkout, a merge or a rebase, and resolving every
conflict before anything can be reviewed again.

- **Both strategies, merge by default.** "Update from the target" offers a
  merge of the target into the change, the default, or a rebase onto it. A
  merge adds one commit, so its push is a fast-forward.
- **No conflicts, no agent.** vuoro computes the merge or rebase itself. When
  nothing conflicts, the result is ready for the person at once, and no agent
  session runs. Where the forge can update the branch itself, a clean update
  goes through the forge, against the head the person saw.
- **Conflicts go to the editor.** The editor, the only agent that writes code
  (section 6.3), gets a fresh repository in its sandbox whose working tree
  holds the conflicted result, with markers, and no credential. It resolves
  each conflict, builds and tests, and commits. vuoro never runs git in the
  editor's repository. It reads the resolved tree back, checks that no marker
  is left, and builds the merge commit, or the rebased commit, itself, with
  the right parents and authorship. A rebase can stop at several commits.
  Each stop gets its own repository and session, and the person accepts each
  stop's resolution before vuoro replays the next commit.
- **The person accepts, and vuoro pushes.** The person sees each resolution
  as a diff of the conflicted files, with the editor's account of each, and
  can talk to the editor before accepting or discarding. Nothing reaches the
  forge until the person confirms the push.
- **Whose change.** The person's own change is pushed directly. On somebody
  else's, a merge goes up as a draft change onto its branch, as a fix does
  (D39), and the author decides. A rebase of somebody else's branch is
  refused, since it rewrites their history. A change whose branch is in a fork
  is refused, as D39 refuses one.
- **Rejected:** having the agent run the merge or rebase itself. vuoro would
  then have to trust git state the agent wrote, and the result's parents and
  authorship would be the agent's to get right.

### D67. vuoro force-pushes with a lease only for a rebase the person accepted

Decided by the owner on 2026-10-27, with D66. Section 6.3 said vuoro never
forces a push to a change's branch. A rebase rewrites the branch, so it
cannot be pushed without force.

- **Only an accepted rebase forces, and always with a lease.** The push is
  `--force-with-lease` against the exact head the person saw in the
  confirmation dialog. A push that landed on the branch since makes the push
  fail rather than be overwritten, and the person updates again.
- **Only the person's own change.** D66 already refuses a rebase of somebody
  else's branch.
- **Everything else still never forces.** The editor's fixes (D39, D41, D42)
  and a merge update stay fast-forward only.

### D68. vuoro answers questions about a change with a read-only agent, and keeps every conversation

Decided by the owner on 2026-10-28. The Summary tab answered one question,
once. The person wants to ask their own and follow up, to keep the answers,
and to find them again later.

- **Ask replaces Summary.** One kind of run, mode `ask`, serves canned
  prompts and free-form questions, and Summary becomes the shipped
  `explain` prompt. Keeping both was rejected: two tabs that start the same
  read-only session differ only in whether a follow-up is allowed.
- **It reads files, git history, and the change, and nothing else.** vuoro
  serves read-only git and change tools over its own MCP server (section
  6.11). A read-only shell was rejected, because a shell is read-only only by
  policy, and a reviewer's lockdown is proven by construction. Files alone
  were rejected, because "why is this here" is answered by history and by
  the change's discussion.
- **Canned prompts are files in vuoro's config directory**, shipped ones and
  the person's own beside them, as presets are (D61). A repository's prompts
  wait for its review checklist, read at the base commit, so that the change
  under review cannot write the question asked about it.
- **Conversations are kept until the person deletes one**, outside the
  retention of review runs. They are the person's notes about a change, and
  they cost money to make.
- **Nothing an agent writes here reaches the forge.** An answer is posted
  only by the person copying it into their own comment.

### D69. vuoro tests a change in the editor's sandbox, and a test has nothing to push

Decided by the owner on 2026-10-29, with D68. Dogfooding a dependency bump
showed that the question before merging was "what breaks", which a
read-only agent cannot answer. D68 keeps Ask read-only, so this is a second
kind of conversation beside it (section 6.12).

- **It is editor-class, in the editor's sandbox (D40).** Running code is the
  editor's job (section 10.4), so a test runs where the editor runs, with no
  forge credential and the network limited to package registries. Giving Ask
  a shell was rejected in D68 and is not reopened here.
- **It has nothing to push.** A test may change files and commit inside its
  own repository while it investigates, but vuoro offers no way out of the
  sandbox for that work. The editor already fixes; a test reports.
- **It shares Ask's tab, prompts, and kept conversations**, marked as one
  that ran code, so a person finds every conversation about a change in one
  place.
- **The sandbox is adapted in use, not settled up front.** A probe starts on
  the editor's sandbox as it is. Whether the registry allowlist is enough for
  a test suite, whether some projects need a container, and how long a
  probe's repository is kept are answered by running probes on real changes.
  The design keeps network policy, runtime, and workspace retention as
  separate seams recorded on each run, so each can change on its own, by a
  new decision, when use shows it must.

