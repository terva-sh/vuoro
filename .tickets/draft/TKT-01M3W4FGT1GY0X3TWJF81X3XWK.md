---
schema: 3
id: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
title: "vuoro v0.1.0 (MVP): orchestrated GitLab reviews, then the inbox"
type: epic
status: draft
status_reason: null
priority: high
due_on: null
labels:
  - inbox
  - review
assignees: []
milestone: v0.1.0
parent: null
origin: null
dependencies: []
blocks_on: none
references:
  - ref: plan:design-of-record
    path: docs/plan.md
  - ref: handoff:first-session
    path: handoffs/2026-10-02-vuoro-handoff.md
  - ref: doc:decisions
    path: docs/decisions.md
  - ref: doc:research
    path: docs/research.md
claim: null
archive: null
created_at: 2026-10-01T16:24:07Z
updated_at: 2026-10-29T18:57:28Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude-code/release-0.1.0
  name: ""
extensions: {}
---

## Description

The first usable vuoro, for its owner on macOS. It is built orchestrator first:
open a GitLab merge request by URL, run several reviewer sessions in parallel
on Claude Code, let a synthesizer session merge their
drafts, talk to any of them, edit the drafts, and submit the review. The
whose-turn inbox lands second and feeds the orchestrator.

The decisions behind this scope were made with the owner on 2026-10-02 and are
recorded in docs/plan.md section 10. v1.0.0 is a separate epic.

terva is not an MVP reviewer: decision D28, after the driver spike found that
terva 0.139.7 cannot be locked down. It moves to the v1.0.0 epic.

## Acceptance criteria

- [x] A GitLab review I am requested on appears in the inbox with the reason it is my turn
- [x] A review I wrote shows as my turn when a reviewer replies, with the age
- [x] I can comment inline, reply, resolve, approve, and submit a batch review without opening GitLab
- [x] just ci passes with CGO_ENABLED=0 and cross-builds linux, darwin, and windows
- [x] A synthesizer session merges the reviewers' drafts and keeps each finding's sources
- [x] Every session shows its tool, model, tokens, and cost
- [x] I can open a GitLab merge request by URL and run several Claude Code reviewers on it in parallel
- [x] GitLab and Forgejo authored/requested changes appear in one inbox with correct turn, reason and earliest-unanswered age.
- [x] I can talk to any reviewer between turns, or after stopping it, and nothing posts without my action (D73)

## Notes

**agent:codex/windows-roadmap-20261002** at 2026-10-03T02:01:28Z

Owner-selected priority: one GitLab-and-Forgejo inbox ordered by whose turn it is. D34 records that outcome and moves Forgejo discovery/inbox into the MVP while writes remain later. The executable backlog is shared discovery/refresh (TKT-01M3ZQXC5CPRDM3CD937C6PC3Q), the existing pure turn ticket, both adapters (new Forgejo slice TKT-01M3ZQXC6SWB7M9D46AN0QJAP8), then the mixed inbox. Structured dependencies connect the slices. The new read-only Forgejo task does not inherit blocked GitLab live-write validation. The broader Forgejo ticket now tracks only writes.

All implementation tickets remain draft. docs/roadmap.md maps the existing implementation, completion gates and subsequent MVP/v1.0 work; docs/plan.md remains authoritative. Considered alternatives were Forgejo posting first and more agent workflows first; the owner chose the inbox because it supplies the missing whose-turn workflow across both existing readers. Older relocation/publication/usage tickets are flagged for evidence-based reconciliation, not silently closed, and the earlier adoption claim is left untouched.

**agent:claude/forgejo-parity-20261006** at 2026-10-08T14:11:39Z

2026-10-08 audit against main at fa91bac, with `CGO_ENABLED=0 go test ./...` green. Nothing has been released: there are no tags, and no decision records a release.

### Ticked
1. **Requested GitLab reviews appear with the reason.** Rule R1 is in `turn/turn.go`, and the inbox shows the reasons. Tested from recorded GitLab facts: `TestRecordedGitLabTurns`, `TestEveryRuleFromRecordedModel`. Live, the GitLab inbox was checked only while empty.
2. **My review shows as my turn when a reviewer replies, with the age.** Rules A1 and A2 cover it, and the inbox shows an age badge. Tests: `TestWaitingAges`, `TestNoiseAndEarliestUnanswered`. The same live caveat applies.
3. **Comment, reply, resolve, approve and submit without opening GitLab.** `forge/gitlab/write.go`, and `TestLiveWrite` passed on drewshort/vuoro-sandbox!1 on 2026-10-05.
4. **`just ci` with cgo off, cross-building three platforms.** Forgejo CI runs the same steps inline: vet for three platforms, tests, `-race` (D53), cross-builds for linux, darwin and windows on amd64 and arm64, the web checks, and the ticket check. It passed on PRs #22 to #26. `just ci` passed locally on 2026-10-08.
5. **The synthesizer merges drafts and keeps their sources.** `TestASynthesizerMergesDraftsAndNamesItsSources`, `TestReviewersRunInParallelAndTheSynthesizerMergesAfterThem`.
8. **Open a GitLab MR by URL and run several Claude Code reviewers in parallel.** `TestParseGitLabURL`, `TestAMergeRequestOpensWithItsDiffAndThreads`, and the parallel-run test. It was used on the sandbox MR.
9. **GitLab and Forgejo changes in one inbox with turn, reason and age.** `TestMixedAccountsRefreshAndFailure`, `TestMixedForgesRoute…`. A live Forgejo comparison passed for 24 PRs. GitHub has joined since (#23).

### Not ticked, and why
6. **"I can talk to any reviewer while it works."** `review/run.go:558` refuses a message while a session is working ("the session is still working; wait, or stop it"). You can talk between turns, or stop a reviewer and resume it. The "nothing posts without my action" half is met: `TestAnAgentDraftsAndOnlyThePersonPosts`. The owner should decide between two options. One is to queue the message, or interrupt the turn, so it can be sent while a session works. The other is to reword the criterion to "between turns, or after stopping it".
7. **"Every session shows its tool, model, tokens, and cost."** Sessions and runs show all four. But `usd()` in `web/src/ui.tsx` renders a cost the tool did not report as `$0.00` rather than marking it unknown. TKT-01M3YMSST3CNA34862NNERKMN2 (Show tokens and cost per session and per review run) is still a draft and covers this.

### Children still open
- **Drafts:**
  - TKT-01M3W4HX0 (Decide how a change finds its git-ticket store and ticket)
  - TKT-01M3W4HX1 (Show a change's ticket, and give it to reviewers as context)
  - TKT-01M3YMSST (Show tokens and cost per session and per review run)
  - TKT-01M3YMSSX (Keep review runs until the change closes, then 30 days)
- **Done tickets with an unticked box:**
  - TKT-01M3W4HWSGA04D4VQ8WHBEJC2D: since-my-last-review mode.
  - TKT-01M3YK0CM0SWMYXMM7A7M7J2QX: a real change with either driver, and settings read at the base commit.
  - TKT-01M3YK0CPYG132897XVVZ9G4EF: three reviewers on two drivers, which waits on terva.
  - TKT-01M3W4HWB551REE766G3TE2Y7D: the GitHub workflow, which D52 made obsolete.

The epic stays open until 6 and 7 are settled, the drafts are done or moved to v1.0.0, and the owner cuts v0.1.0.

**agent:claude-code/release-0.1.0** at 2026-10-29T18:57:28Z

2026-10-29: the two criteria the 2026-10-08 audit left open are settled, and this supersedes that note's "Not ticked" and "Children still open" lists. The criteria were renumbered when the talk criterion was reworded, so that note's "6." and "7." no longer name the same lines.

- Talking to a reviewer: the owner reworded the criterion to "between turns, or after stopping it" (D73), which is met today. Queuing a message sent mid-turn is TKT-01M64FWYSQ40PJJ1Y3S1XG3VXY (Queue a message sent to a reviewer mid-turn for the turn's end), under v1.0.0. The window's help text now says between turns.
- Tool, model, tokens and cost: TKT-01M3YMSST3CNA34862NNERKMN2 (Show tokens and cost per session and per review run) is done. A cost the tool did not report, or a turn stopped before it reported usage, shows as unknown or as a floor, never $0.00.
- TKT-01M3W4HX0 (git-ticket linking spike), TKT-01M3W4HX1 (the change's ticket as context) and TKT-01M3YMSSX (run retention) moved to TKT-01M3YMSTYMFRDFXD4QSVWHMM8P (vuoro v1.0.0) by the owner's decision (D73).

Every criterion is ticked. The epic closes when the owner tags v0.1.0.
