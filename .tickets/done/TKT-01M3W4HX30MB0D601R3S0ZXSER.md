---
schema: 3
id: TKT-01M3W4HX30MB0D601R3S0ZXSER
title: GitHub adapter for the forge interface
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - forge
  - github
assignees: []
milestone: v1.0.0
parent: TKT-01M3YMSTYMFRDFXD4QSVWHMM8P
origin: null
dependencies:
  - TKT-01M3W4HWTXC6EM9SS84A4WMEPG
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-01T16:25:25Z
updated_at: 2026-10-08T06:12:24Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
extensions: {}
---

## Description

Implement the `forge` interface for GitHub: review requests, pull request
reviews and their states, review threads, and pending reviews, which are the
GitHub equivalent of GitLab draft notes. GraphQL may be the better fit for
threads and resolution, so decide in the plan.

A part of GitHub that does not fit the interface is a finding against the
model in docs/plan.md section 4. Do not leak GitHub detail through the
interface.

## Acceptance criteria

- [x] GitHub pull requests appear in the inbox with correct turns, tested from fixtures
- [ ] Comment, reply, resolve, approve, request changes, and submit work against a real pull request

## Implementation plan

Owner decisions, 2026-10-08 (D54):
- **Drafts are a GitHub pending review.** GitHub keeps a pending review private to its author, so D43's reason for keeping drafts local doesn't apply.
- **Reads need a token and use GraphQL.** GitHub refuses anonymous GraphQL with a 403, and REST has no thread resolution.
- **Live tests use a private sandbox repo under warricksothr.** Approving and requesting changes need a second account, so those stay fixture-tested until there is one.

### Slice 1, this PR: read-only GitHub in the inbox and the window
- **Package `forge/github`.**
  - The API base comes from the configured web URL: github.com uses api.github.com, and any other host uses https://HOST/api/v3 and /api/graphql (GitHub Enterprise Server). The Ref keeps the web host.
  - Every request sends `Authorization: Bearer`. Errors are scrubbed and mapped to `forge.APIError`, the same as Forgejo.
  - `ParseURL` matches `/pull/N` with two-part projects.
- **Reading** (`Open` and `ReadFacts`) is one GraphQL query.
  - It reads the pull request, author, branches and OIDs, `isDraft`, state, `mergeStateStatus`, and the viewer's permission for CanMerge.
  - It also reads `reviewRequests`, reviews (verdict, `submittedAt` and `commit.oid`, with a dismissed review's original verdict taken from `ReviewDismissedEvent.previousReviewState`), `reviewThreads` with `isResolved` and their comments, issue comments as unanchored threads, assignees and participants.
  - From the timeline it reads `ReviewRequestedEvent` (R2b) and `HeadRefForcePushedEvent`. GitHub records no push time otherwise, so R2 uses `commit_id != head`.
  - Any connection past 100 nodes is paged by cursor, up to a cap, and an error is returned rather than a partial read. A re-read of `updatedAt` refuses a snapshot that changed partway through.
  - Bots come from `__typename == Bot` and the `[bot]` suffix.
  - `Open` adds the patches from REST `/pulls/N/files`, paged. The viewer's PENDING review stays out of Threads; slice 2 surfaces it as Drafts.
  - The checkout is `refs/pull/N/head`, over HTTPS and SSH from the repository's URLs.
- **`Discover`** runs GraphQL search for `is:pr is:open` with `author:`, `review-requested:`, `reviewed-by:` and `assignee:` for the user. It merges roles and pages by cursor, returning an error at GitHub's search cap rather than a partial membership.
- **`Me`** reads the viewer's login.
- **Capabilities are all false** in slice 1.
- **Registration:**
  - `forge.GitHub`, the adapters registry, and URL routing for github.com, which is refused today.
  - The kind joins config's accepted kinds.
- **Kind sites:**
  - `Ref.Display` and `changeLabel` use `#`, and ChangeWord says "pull request".
  - `ciFile` treats `.github/workflows/` as CI.
  - In setup: the token page, the option, and the address text.
  - In the web frontend: `changeNumber`, `changeWord`, `forgeName`, and the GitHub URL shapes in markdown.tsx's `projectURL`, `commitURL` and reference links.
  - Each kind check that tests "not forgejo" for GitLab is rewritten so GitHub doesn't fall into it.
- **Tests:**
  - Scrubbed GraphQL and REST fixtures from public pull requests, served by httptest.
  - Turn tests through `turn.Derive`.
  - Discovery paging and the search cap.
  - The tests that assert github is refused are updated.
  - An opt-in live read test, `VUORO_LIVE_GITHUB` + `_PR`.
- **Docs:** D54, and docs/plan.md §5 gains a GitHub section.

### Slice 2: the writer
- GraphQL pending-review writes:
  - `addPullRequestReview` (the pending review)
  - `addPullRequestReviewThread` (inline comments)
  - `addPullRequestReviewThreadReply` (replies)
  - `updatePullRequestReviewComment` and `deletePullRequestReviewComment` (edits and deletes)
  - `submitPullRequestReview` with the event, for Submit
  - `resolveReviewThread` and `unresolveReviewThread`
- Approve is a submit with `APPROVE`. Unapprove is unsupported, because only dismissal withdraws an approval.
- Suggestions stay off until a multi-line anchor exists.
- One pending review per person per pull request: vuoro adopts the one the person started in the web UI.
- Live test on the warricksothr sandbox for comment, reply, resolve and submit (comment).

### Slice 3: the stacker
- A native draft pull request. A 422 because the plan has no drafts maps to ErrNotDraft with a non-draft PR, or a refusal, decided then.

## Notes

**agent:claude/forgejo-parity-20261006** at 2026-10-08T05:09:12Z

Slice 1 is built: the reader, discovery, Me, registration, and every place that keys on a forge's kind.

### Verified
- **Fixtures.** Two public cli/cli pull requests were recorded on 2026-10-08 and scrubbed: #13899 and #14578. testdata/README.md says how. `turn.Derive` over them gives the expected turns:
  - #13899: reviewer1 was asked again after requesting changes, so the turn is mine with R1, R2 and R2b, and the author waits on them.
  - #14578: reviewer1's approval was of an earlier head, so R2 makes it mine, and the author waits.
- **Synthetic cases:** a pending review stays out of the facts; a dismissed review takes its verdict from the timeline; a connection is paged by cursor; a pull request that moved between pages is refused; reading without a token sends nothing; GraphQL NOT_FOUND is ErrNotFound; errors never carry the token; and discovery's role merge, other-account refusal, search-cap refusal and other-host refusal.
- **Live, read only:**
  - `TestLive` opened and checked out cli/cli #13899, #14578 and #13982.
  - #13084 failed at `git fetch`. It is merged, and its target branch is gone, so the failure is the test PR's, not the adapter's.
  - `TestLiveDiscovery` found the owner's 3 open pull requests and read each one's facts.

### Findings, kept out of the interface
- GitHub records a time only for a force push, so R2 rests on the commit comparison.
- A team's review request, or one for an account the token cannot see, comes back as a null reviewer and is skipped.
- `line` is null on an outdated thread. It is anchored at `originalLine` on the original commit.
- The merge base needs a REST compare, so only `Open` sets `Diff.Base`. Nothing outside the adapters reads it from facts, which was checked.

### Kind sites
- An empty kind is a GitLab-era record, so GitLab stays the fallback everywhere. GitHub is named next to Forgejo at each site.

**agent:claude/forgejo-parity-20261006** at 2026-10-08T05:42:40Z

Slice 2, the writer, is built and validated live. Slice 1 landed as PR #23.

### The spike, on 2026-10-08
It ran on warricksothr/vuoro-sandbox#1, a private repository made for it. docs/plan.md §5.4 "GitHub writes" has the full table.
- **The finding that shapes the design:** a reply that names no pending review is published immediately (state SUBMITTED), and one sent while a pending review exists joins it. So every comment and reply names the pending review. A write that comes back other than PENDING is an error, and a write naming a submitted review answers null and publishes nothing.
- **A pending review's body cannot be edited** ("Could not edit a review with a missing body"). Comments on the whole change, and replies to them, therefore stay local until Submit, as Forgejo's do.
- **Smaller findings:**
  - A thread on a line outside the diff answers a null thread with no error.
  - A second pending review is refused.
  - An empty review is refused.
  - The author cannot approve or request changes.
  - Resolve and unresolve work.

### Validation
- **Fixture GitHub.** `write_test.go` runs against a fake that keeps a pending review and records every mutation. It covers:
  - Every comment names the pending review.
  - A published or refused answer is an error.
  - An inline comment on an earlier head is refused before any write.
  - General comments stay local.
  - Only pending comments are edited or deleted, and a web-UI body is read-only.
  - Submit's body order and replies, a verdict alone, and a verdict already standing.
  - A retry posts only the replies.
  - Approve refuses while comments are pending, and Unapprove is unsupported.
  - Resolve works on review threads only.
  - Open shows pending-review drafts.
- **Live.** `TestLiveWrite` round-tripped all of it on the sandbox:
  - It kept an inline comment, a reply, a summary, an edit and a delete pending, and confirmed none reached ReadFacts.
  - It published them as one review, then resolved and reopened the thread.
  - GitHub refused approve and request-changes from the author, with the expected messages.

### Criterion 2 stays unticked
Approve and request changes were not exercised live as a reviewer, which needs a second GitHub account. They are fixture-tested, and the live test confirms GitHub's own refusal for the author.

**agent:claude/forgejo-parity-20261006** at 2026-10-08T06:12:13Z

Criterion 2 is left unticked on purpose. Comment, reply, resolve and unresolve, and submit (as a comment) passed live on warricksothr/vuoro-sandbox#1 in `TestLiveWrite`, 2026-10-08. Approve and request changes passed live only as GitHub's refusal of the author's own verdict, which the test checks. A real approval needs a second account (D54), so  (Verify GitHub approve and request changes live from a second account) tracks it.

**agent:claude/forgejo-parity-20261006** at 2026-10-08T06:12:24Z

Supersedes the previous note's blank ticket ID: the follow-up for a live approval from a second account is TKT-01M4D283WTGMG60THDTWD3X59Y (Verify GitHub approve and request changes live from a second account).

## Summary

GitHub is a full forge in vuoro, in three PRs on Forgejo:

- **#23, reading (D54):** pull requests in the inbox, with turns derived from GitHub's reviews, requests, dismissals, and force pushes. Discovery runs GitHub search across the author, review-requested, reviewed-by and assignee roles. The window reads pull requests with files paged up to GitHub's 3000-file listing. A token is required, because GitHub refuses anonymous GraphQL.
- **#24, writing:** drafts are the person's GitHub pending review, and every comment and reply names it, because GitHub publishes a comment that names none at once. A comment on the whole change stays in vuoro until Submit. Edit and delete touch only pending comments, and say so when a submission on GitHub races them. Submit publishes the review with the chosen verdict and then posts replies, keeping any reply GitHub did not confirm. Resolve and unresolve work on review threads. Approve is supported and unapprove is not.
- **#25, stacking:** the editor's fixes go up as a draft pull request onto the head branch, or an ordinary one where the plan has no drafts, or onto the person's own head branch. Forks are refused.

Live tests passed on warricksothr/vuoro-sandbox for reading, writing, and stacking. docs/plan.md §5.4 records what the spikes showed. A live approval from a second account remains, in TKT-01M4D283WTGMG60THDTWD3X59Y (Verify GitHub approve and request changes live from a second account).
