---
schema: 3
id: TKT-01M3W4HX4CJE62G9R124EV5PT4
title: Forgejo posting through the shared forge boundary
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - forge
  - forgejo
assignees: []
milestone: v1.0.0
parent: TKT-01M47EFQ400T20P79H9X152NAZ
origin: null
dependencies:
  - TKT-01M3W4HWTXC6EM9SS84A4WMEPG
  - TKT-01M3ZKGRZBBS869259MBA818NQ
  - TKT-01M3W4HWR5K8TXA6S6HBVXDXX5
  - TKT-01M47EG9BHW0HBVFBTSBB1915X
  - TKT-01M47EG9CSRDZ2HKANYN30XBAY
  - TKT-01M47EG9E07Y4N1J34S5VDDC5N
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-01T16:25:25Z
updated_at: 2026-10-06T04:35:59Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
extensions: {}
---

## Description

Complete human-directed Forgejo posting through the shared forge boundary: comment, reply, approve, request changes and submit drafts, with live round-trip validation and capability-gated controls. Agents still only draft; every remote write requires the person’s protected UI action.

Open-by-URL reading/checkout is complete under TKT-01M3ZKGRZBBS869259MBA818NQ (Open Forgejo PRs through the shared forge boundary), merged as PR #3. D34 moves discovery and the mixed inbox into v0.1.0 under TKT-01M3ZQXC6SWB7M9D46AN0QJAP8 (Discover open Forgejo changes for the shared inbox). This v1.0 ticket now tracks writes only, avoiding an unrelated live-write prerequisite on read-only discovery.

## Acceptance criteria

- [x] Comment, reply, approve, request changes, and submit work against a real pull request
- [x] Human-directed writes refresh forge facts and the shared inbox turn; agents have no posting route.
- [x] Pending comments are created, edited or replaced, deleted, and submitted as one review, as the spike's matrix says

## Implementation plan

Implements D43 through the boundary that D44 settled.

### Where the drafts live

- **A new store.** `forge.DraftStore` (`Drafts(ref)` and `SaveDrafts(ref, drafts)`) keeps a forge's pending comments in vuoro's state. A client that needs it implements `forge.LocalDrafter` (`KeepDraftsIn(DraftStore)`). The app gives it a store in `app.client` after dialing, backed by `StateDir/drafts/<host>/<project>/<n>.json` through the `state` package. That file is private, 0600.
- **The adapter does no file I/O**, and the application knows no adapter.
- **Capabilities.** Without a store or a token, Forgejo reports no write capabilities. With both, it reports drafts, reply, approve and request_changes. Resolve, unapprove and suggestions stay false (D43).

### Reading

- **What Open returns.** It returns the local drafts as well as any pending review the person started in Forgejo's web UI, so the window shows everything Submit will publish.
- **IDs.**
  - Local drafts get ids `local-<random>`.
  - The pending review's body becomes id `review-<id>`. Review and comment ids are separate sequences that can collide.
  - Forge-side pending comments keep their numeric comment id.

### Writing

- **CreateDraft** saves locally: an inline draft keeps its anchor, a reply keeps its thread id, and a general draft keeps its body.
- **UpdateDraft and DeleteDraft.**
  - A `local-` draft is changed in the store.
  - A forge pending comment is edited with `PATCH /issues/comments/{id}`, or deleted with `DELETE /pulls/{n}/reviews/{pending}/comments/{id}`.
  - The web-started review body is refused with ErrUnsupported, because changing it would leak a new body into the timeline. Submit keeps it.
- **Submit(verdict)** makes one `POST /pulls/{n}/reviews` call:
  - **Event:** COMMENT, APPROVED, or REQUEST_CHANGES.
  - **commit_id:** the current head.
  - **body:** the pending review's body, if there is one, then the general drafts, in order.
  - **comments[]:** the inline drafts, with `new_position` for a new-side line and `old_position` for a removed one.
- **Refused before any write:**
  - An inline draft written against an older head.
  - Request changes with no body.
  - A comment with nothing at all to publish.
- **After the review,** each reply posts into its thread's own review (`POST /reviews/{rid}/comments` at the thread's path and positions), or as an issue comment for a general thread. Each published draft is removed from the store as it lands, so a retry never publishes twice. A failed reply stays pending and is named in the error.
- **Approve(true)** is an APPROVED review with no body. It is refused while a web-started pending review exists, because Forgejo would merge and publish it. Approve(false) and Resolve are ErrUnsupported.
- **Requests.** `get` becomes `do(method, …)`, with the same scrubbing, size limit, refused redirects, and APIError.

### Checks

- **The inbox.** It already refreshes after every write through `app.reply`. ReadFacts never returns drafts.
- **Offline tests:** each request's shape, ID routing, stale-head refusal, the partial reply failure, and the store round trip.
- **The live test.** `TestLiveWrite` follows TestLive's pattern: every request goes through `tea api --login`, so tea owns authentication and the test never sees a token. It reviews as a `reviewer` login on the sandbox PR, and an `author` login reads back what others can see.

## Notes

**agent:codex/forgejo-20261002** at 2026-10-03T00:44:56Z

The owner brought the open-by-URL read/review slice into v0.1 on 2026-10-02, tracked as TKT-01M3ZKGRZBBS869259MBA818NQ (Open Forgejo PRs through the shared forge boundary). This ticket retains discovery/inbox and human-directed writes for v1.0, including the existing live GitLab-write dependency. The new slice proceeds independently; it does not claim this ticket's inbox or posting criteria.

**agent:claude/forgejo-parity-20261006** at 2026-10-06T01:51:05Z

Re-parented under TKT-01M47EFQ400T20P79H9X152NAZ (Forgejo parity: posting, verdicts, and the editor's push) on 2026-10-06. Its earlier prerequisites (GitLab writes, Forgejo open-by-URL, inbox view) are all done. It now waits on TKT-01M47EG9BHW0HBVFBTSBB1915X (Prove which Forgejo review writes round-trip against a sandbox PR), TKT-01M47EG9CSRDZ2HKANYN30XBAY (Carry the person's verdict on submit), and TKT-01M47EG9E07Y4N1J34S5VDDC5N (Show only the write actions a forge supports), because request changes is not in forge.Writer and the Forgejo API has no review-comment edit, thread reply, or resolve route in its swagger. The editor's push is a sibling ticket, TKT-01M47EG9F7HZ7C3EPYF2TJMEYV, so posting does not wait on it.

**agent:claude/forgejo-parity-20261006** at 2026-10-06T02:00:31Z

From the spike (TKT-01M47EG9BHW0HBVFBTSBB1915X, docs/plan.md 5.3): Forgejo drafts stay local, because the issue timeline exposes a pending review's body to other readers. Submit is one POST /pulls/{n}/reviews with the verdict, the joined general drafts as the body, and comments[]. If a pending review already exists, perhaps started in the web UI, that call merges into it and replaces its body, so show its comments as part of what Submit publishes. Replies post into the thread's own review at its path and position after the review. They publish one by one, so handle a partial failure explicitly. Resolve and approval withdrawal are unsupported.

**agent:claude/forgejo-parity-20261006** at 2026-10-06T04:35:59Z

Implemented as the plan says, D43 throughout.

### Where it landed

- **The boundary.** `forge.DraftStore` and `forge.LocalDrafter` are in forge/forge.go. `app.client` gives a LocalDrafter a store, `app/localdrafts.go`: one 0600 file per change under `StateDir/drafts`, named by a hash of the change, and removed when it is empty.
- **The writer** is `forge/forgejo/write.go`.
  - Local draft ids are `local-<hex>`, and a web-started pending summary is `review-<id>`.
  - Submit publishes the review in one call, joining the web summary and the general drafts into its body. It then posts each reply into its thread's review, or as an issue comment for a general thread.
  - A stale inline draft, a request for changes with no body, and an empty submit are each refused before any write.
  - A failed reply stays pending and is named in the error. A retry never republishes the review.
- **`Client.get`** became `do(method, …)`, with the same scrubbing, size limit and refused redirects.

### A reader bug found live, and fixed here

Forgejo lists in `requested_reviewers` anyone whose latest review approved or requested changes, with no request on record. I proved this on a fresh sandbox PR, #5:
- After a comment, the list was empty.
- After REQUEST_CHANGES or APPROVED, it listed the reviewer.
- A real request added a timestamped REQUEST_REVIEW record.

`reconcileRequests` treated a listed reviewer with no timestamp as a pending request and reset them to unreviewed. So after a reviewer's verdict, the inbox would still have said it was their turn. A listed reviewer with no request on record, whose undismissed verdict stands on today's head, now counts as having answered. The test case "request without timeline timestamp", which expected unreviewed, encoded the disproved assumption. It now expects approved, beside a new stale case. This matches the plan's R1 row for Forgejo ("…and have no review"). docs/plan.md section 5 records it.

### Tests

- **Offline.** `forge/forgejo/write_test.go` covers:
  - local-only until Submit, with the exact review and reply requests;
  - each refusal before writing;
  - a partial reply failure;
  - a review started in the web UI: edit and delete on Forgejo, the summary refused but kept by Submit, and a bare approve refused;
  - approve, withdraw, resolve and capabilities.

  `app/localdrafts_test.go` covers the file and the wiring.
- **Live.** `TestLiveWrite` passed on 2026-10-06 against `warricksothr/vuoro-sandbox#1`, as reanzu with warricksothr as author, both through `tea api --login`:
  - Nothing showed to the author before Submit, including in the timeline.
  - One review published the inline comment, the edited general comment and the request for changes, and the reply joined its thread.
  - The author's turn went from A1×12 to A1×13 A2×1.
  - Approve cleared A2.
- `just ci` passes on Linux.

### Not done here

The editor's push to Forgejo is TKT-01M47EG9F7HZ7C3EPYF2TJMEYV. Probe PR #5 is closed. The sandbox repositories stay for the stacker's live test.

## Summary

Forgejo posts human-directed reviews through forge.Writer, with drafts kept in vuoro until one review publishes them with the person's verdict, and replies posted into their threads. Resolve and withdrawing an approval stay unsupported. Live-validated on the sandbox PR, including the author's inbox turn. It also fixes the reader treating a reviewer listed for their own verdict as still requested.
