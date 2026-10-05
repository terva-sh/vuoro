# Vuoro roadmap

Execution proposal following the owner’s choice on 2026-10-02: one inbox for GitLab and Forgejo, ordered by whose turn it is (D34). Updated through the shared inbox implementation after merged discovery and turn PRs #6–#9.
The design of record remains [plan.md](plan.md), with owner decisions in
[decisions.md](decisions.md). This proposes execution order and completion gates;
implementation tickets stay in draft until selected. D34 brings Forgejo discovery/inbox into the MVP; Forgejo writes and GitHub remain later work.

## Where we stand

| Area | Evidence and remaining work |
|---|---|
| Desktop foundation | Loopback protection, tuohi, embedded Preact UI, shared violet styling, cgo-free cross-builds, and the internal PR review workflow are implemented. Merged PR #4 passes the complete Windows/Linux/macOS gate, including the macOS window check. |
| Reviews | Parallel Claude Code reviewers, synthesis, verification, conversations, editable local drafts, suggestions, and a kept-findings report are implemented. Local summaries stream for the whole change or an exact range since a completed Vuoro review, with history and cancellation. |
| Forges | GitLab and Forgejo open by URL through shared contracts. Both provide review checkout; Forgejo remains read-only. GitHub is not supported yet. |
| GitLab posting | Implemented and validated live on 2026-10-05 against a private sandbox merge request (`drewshort/vuoro-sandbox!1`) with the opt-in `TestLiveWrite`: inline and general drafts publish as one batch, a reply lands in its thread, resolve and unresolve round-trip, and approve then revoke move the derived turn from R1 to waiting and back. The test drives `forge.Writer` directly; the window's controls over it have offline coverage. |
| Inbox | Shared discovery/cache/refresh, the pure turn engine and both discovery adapters are merged in PRs #6–#9; the home inbox landed in PR #10 with reasons, ages, account status, snooze and workspace opening. Forgejo live comparison passed for 24 open PRs. GitLab personal-account comparison passed on 2026-10-03 for an empty inbox (zero requested reviews and zero open memberships); authenticated nonempty GitLab turn facts and pagination remain unexercised live. |
| Author help | Thread classification and local replies are implemented, with explicit editing, acceptance into pending comments and Submit. [Interaction sketch](author-triage.md) records the flow and stale-thread states; fixture coverage proves posting boundaries. Live GitLab posting is validated above. |
| Review lifecycle | Token/cost displays and head-SHA flags already exist. Run and draft badges now show verified commit distance, with explicit rewritten/unavailable states and old-head drafts separated from today’s diff. Unknown-cost labeling and retention remain. |
| External blockers | Terva reviewers wait for a release whose per-run MCP configuration and lockdown are proven. The editor waits for a sandbox spike. Neither blocks the inbox. |

The shared boundary is deliberately specific: forges use `forge.Provider` and
optional `forge.Writer`; tools use `agent.Driver`; ticket stores use `tickets`.
Discovery should extend the forge boundary with a narrow contract, without
requiring agent tools or ticket stores to implement forge operations.

## Recommended sequence

Keep the existing decisions: orchestrator before inbox (D12), GitLab first
(D4), Forgejo before GitHub (D21), Claude Code for the MVP (D28), and the editor
at v1.0 after its sandbox proof (D18/D20). The next product milestone should be
an inbox that tells the person what they owe, and opens the existing review
workspace. The owner selected both GitLab and Forgejo for that inbox (D34). Build both
adapters against the same discovery/refresh service and prove mixed-account
results, while keeping Forgejo read-only until its separate write milestone.

| Stage | Outcome | Completion gate |
|---|---|---|
| 0. Reliable baseline | Required CI is green on Linux, macOS, and Windows; remaining backlog gaps are identified. | PR #4 passes native platform checks, including the macOS window check, with all four Windows failures resolved. Audit older tickets against evidence. |
| 1. First useful inbox | My GitLab and Forgejo reviews appear automatically, grouped by whose turn it is, oldest unanswered event first. | Fixture tests cover every turn rule, bots/system notes and re-requests; live discovery matches both forges; opening a row enters the existing review workspace; refresh, account errors and snooze behave correctly. |
| 2. Complete the MVP | Both reviewer and author workflows work end to end, with accountable usage and history. | Real GitLab posting and refreshed turns are validated; author triage/reply drafts and summaries work; ticket context is optional; stale-head and retention criteria pass; unavailable cost is labeled honestly. |
| 3. Complete Forgejo posting | Internal PRs can be reviewed and posted from Vuoro, using the existing inbox. | Human-directed draft/reply/verdict writes round-trip; unsupported actions remain hidden; refreshed forge state drives the inbox. |
| 4. Complete review coverage | GitHub joins the same workflows; reviews can start locally and survive new commits. | GitHub thread resolution, requests, verdicts and writes are validated; local ranges use the orchestrator; base-commit repository instructions and deliberate incremental review have coverage. |
| 5. Ship v1.0 | A new person can install, configure and use Vuoro on every supported platform, with the sandboxed author-side editor. | Onboarding, archives/self-update, signing decision, tray/notifications and budget behavior are validated; the editor proves isolation, shows local commits for acceptance, and never holds a forge token or pushes itself. |

These are outcome gates, not calendar estimates. Stages 3–5 can ship as smaller
pre-1.0 releases while retaining the existing v1.0 commitments. The terva driver
can proceed independently only after its lockdown spike passes. Editor use on
other people's changes stays after v1.0 (D18).

## Inbox implementation slices

1. **Shared discovery and refresh:** TKT-01M3ZQXC5CPRDM3CD937C6PC3Q (Share forge discovery,
   caching, and refresh for the inbox). Define an optional forge discovery contract,
   account-scoped results, pagination/cancellation boundaries, cache refresh and
   last-success/error metadata. A failing account must not erase another
   account’s results or look like an empty inbox. Preserve authored snoozes,
   classify configured bot accounts before turn derivation, and require no
   installed agent or ticket store. Adapter fixtures prove the shared contract.
2. **Pure turn derivation:** TKT-01M3W4HWPT9NYP4ADW339FJDSC (Derive the turn
   for each change, with its reasons). Its model dependency is already done.
   Implement R1/R2/R2b/R3 and A1/A2/A3 from plan section 4, with earliest
   unanswered `since`, explicit reasons, and no network I/O. Use both forge
   readers' recorded facts to expose normalization gaps. Do not invent event
   times or treat unavailable merge permission as permission.
3. **GitLab discovery:** TKT-01M3W4HWNGYDHGXMSE91HKGTNT (GitLab adapter:
   discover the changes I review and wrote). Its original foundation dependencies
   are done; the new shared discovery service must land first.
   Discover authored/requested/assigned changes, deduplicate account roles,
   paginate, and integrate the shared derived cache and bounded refresh. Implement
   the common discovery contract; retain per-adapter API details, host/token
   isolation and visible stale/error status. No GitLab write is needed to build
   discovery, although private live validation needs configured credentials.
4. **Forgejo discovery:** TKT-01M3ZQXC6SWB7M9D46AN0QJAP8 (Discover open Forgejo changes for
   the shared inbox), a separate read-only task under the MVP. Discover
   authored/requested/assigned open PRs, deduplicate roles, respect host-specific
   pagination and credential boundaries, and compare a live account with the
   forge. Depend on the shared service and completed Forgejo reader, not blocked
   GitLab write validation.
5. **Inbox integration:** TKT-01M3W4HWR5K8TXA6S6HBVXDXX5 (Inbox view: every
   change, sorted by whose turn and how long). Depends on turn derivation and both discovery adapters.
   Show reason, age, forge, role and refresh status; keep snoozes as authored
   annotations; return a snoozed item on its date or a new forge event. Refresh
   only while the window is open, per D24. Exercise transitions after human
   posting without adding an agent or ticket-store requirement to the inbox.

The critical path is `shared discovery → GitLab + Forgejo adapters`, alongside
`turn`, then `mixed-forge inbox`. The owner selected all five slices for
autonomous implementation. The shared
service, turn engine and both adapters are merged; the home inbox completes
the implementation path. GitLab’s personal-account discovery comparison passed
for `WarrickSothr` on 2026-10-03: both discovery and the independent paginated
requested-review query returned zero reviews, and all role queries found zero
open memberships. This clears the discovery authentication blocker and completes
the five selected implementation tickets. It proves empty-inbox parity;
offline fixtures cover nonempty facts and pagination, with an additional
anonymous live facts check. Authenticated nonempty GitLab coverage is still
to obtain. The separate
GitLab write validation was not attempted with this read-only credential;
it passed separately on 2026-10-05 with the owner's configured token.

## Remaining MVP work

| Work | Ticket | What remains |
|---|---|---|
| Usage display | TKT-01M3YMSST3CNA34862NNERKMN2 — Show tokens and cost per session and per review run | Audit existing displays and totals, especially tools with no reported cost. |
| Ticket context | TKT-01M3W4HX0BNXTDC2DXBH5HQ0FM — Decide how a change finds its git-ticket store and ticket; then TKT-01M3W4HX1PHPQ98KX0PE5HW60E — Show a change's ticket, and give it to reviewers as context | Settle linking and store discovery before implementation; no ticket UI or dependency when absent. |
| Retention | TKT-01M3YMSSX3WH3Q6TW6BCGK7XV7 — Keep review runs until the change closes, then 30 days | Configurable deletion of expired transcripts and preservation of one-line run records. |

## Backlog reconciliation

- TKT-01M3YMST0M467K1FB4J5B2C7B2 (Move vuoro to terva-sh/vuoro on Forgejo once
  the skeleton lands) still reads as a draft although the repository and CI
  already exist. Close against explicit evidence when selected.
- TKT-01M3ZDQ9W9TCQCM93S8YEWTEYS (Adopt terva-review and the two-forge process
  on Forgejo) has all criteria checked, but still holds an earlier session's
  claim. Confirm that session's ownership and close the record against merged
  PR #1 and the now-exercised review/carry/sync process.
- TKT-01M3YMSVN79DEXW47ZXB519KVE (Publish vuoro: GitHub mirror, and docs for
  someone new) is partially satisfied: the mirror exists, while install and
  onboarding remain. Retain that unfinished scope.
- Discovery is now separate from TKT-01M3W4HX4CJE62G9R124EV5PT4 (Forgejo
  posting through the shared forge boundary). The new MVP discovery ticket has
  no blocked GitLab live-write dependency; the original ticket retains later
  writes. Keep PR #3's completed open-by-URL slice closed.
- Give the editor sandbox spike a separate schedule so its risk is understood
  before the rest of v1.0 is ready. Do not make safe reviewer/inbox work wait
  for it. A failed spike leaves editor implementation unstartable.

## How to use this plan

The owner selected the five inbox slices, and their implementation now follows
this sequence. Keep the remaining MVP/v1.0 tickets in draft, and judge each
stage against its observable completion gate before expanding scope. The
existing “agent drafts, person posts” and read-only reviewer boundaries hold
across every stage.
