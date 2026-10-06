---
schema: 3
id: TKT-01M47EG9BHW0HBVFBTSBB1915X
title: Prove which Forgejo review writes round-trip against a sandbox PR
type: spike
status: done
status_reason: null
priority: high
due_on: null
labels:
  - forgejo
  - forge
assignees: []
milestone: v1.0.0
parent: TKT-01M47EFQ400T20P79H9X152NAZ
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-06T01:50:57Z
updated_at: 2026-10-06T02:00:31Z
created_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
updated_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
extensions: {}
---

## Description

Before building `forge.Writer` and `forge.Stacker` for Forgejo, find out, against a real pull request on the internal instance (Forgejo 15.0.3, gitea-1.22 API), which of GitLab's write operations Forgejo can do through its API and what each one costs. The swagger leaves these questions open:

### Questions

- **Editing a pending comment.** The swagger offers no edit for review comments. Does `PATCH /repos/{o}/{r}/issues/comments/{id}` edit one? If not, do we delete and recreate it, which changes its ID and so the local draft's identity?
- **General comments.** A review carries one body. How do several general drafts map: joined into the body, or posted as issue comments at submit time? Does either keep "the review lands as one event" (the `Submit` contract in `forge/forge.go`)?
- **Replies.** Review comments have no reply-to field. Does a new pending comment at the same path, line and commit join the existing conversation in the UI? Is there any other route to a reply?
- **Resolve.** Is there an API route at all? If not, resolving stays hidden on Forgejo.
- **Withdrawing an approval.** Dismissal needs repository admin and leaves a record. Is it an honest equivalent of GitLab's unapprove, or should withdrawal stay unsupported?
- **Verdicts.** Do `COMMENT`, `APPROVED` and `REQUEST_CHANGES` on submit change `reviewer state` the way `forge/forgejo` and the turn engine read it?
- **One pending review per person.** What does the API do when a pending review already exists, including one made in the web UI?
- **Draft PRs.** Does a `WIP:` title prefix (or the instance's configured prefix) make a PR a draft? Does the PR's `head.repo` against `base.repo` tell a fork apart reliably?
- **Suggestions.** How does a GitLab-style `suggestion` block render in a Forgejo comment? What should vuoro post in its place?

### Output

A capability matrix written into docs/plan.md, with the request and response shape of each operation the writer will use. Scrubbed fixtures go under `forge/forgejo/testdata`. Notes on this ticket record the alternatives that lost.

## Acceptance criteria

- [x] Every question in the description has an answer backed by a live request against the instance
- [x] docs/plan.md carries the Forgejo write capability matrix
- [x] Scrubbed fixtures for each operation the writer will use are under forge/forgejo/testdata

## Notes

**agent:claude/forgejo-parity-20261006** at 2026-10-06T02:00:30Z

Ran on 2026-10-06 against the private sandbox `warricksothr/vuoro-sandbox` PR #1 on the internal Forgejo (15.0.3, gitea-1.22 API). The PR author was `warricksothr` and the reviewer was `reanzu`, a collaborator with write access, both through `tea api`. Scrubbed cases are in `forge/forgejo/testdata/writes/spike.json`, and the matrix is in docs/plan.md section 5.3.

### Answers

- **Editing a pending comment:** yes. `PATCH /issues/comments/{id}` edits it and keeps its id.
- **General comments:** a review has one body. A pathless comment is accepted but publishes as a code comment with an empty path. Join general drafts into the body.
- **Replies:** posting a comment into the thread's own review (`POST /reviews/{thread review}/comments`, same path and position) joins that conversation in the web UI, which I checked on the PR files page. It publishes at once, and anyone with access can post one. A comment at the same line in a new review is a separate conversation, which matches the adapter's grouping by review, path and position.
- **Resolve:** no API route. Hide it.
- **Withdrawing an approval:** the reviewer gets 403 dismissing their own. The owner can dismiss and undismiss. The reviewer can delete their own published review, which erases the record. Leave withdrawal unsupported.
- **Verdicts:**
  - All three publish.
  - `REQUEST_CHANGES` needs a body. `COMMENT` needs a body or a comment. `APPROVED` may be empty.
  - The author can do neither approve nor request changes on their own PR.
  - A later approval marks the same reviewer's earlier request-changes review dismissed.
- **One pending review per person:** a second `PENDING` upserts the same review and replaces its body. A one-call review while a pending review exists merges into it, publishing its comments and replacing its body.
- **Draft PRs:** `WIP:` and `[WIP]` both give `draft: true`. A fork PR has `head.repo` different from `base.repo`, and `fork: true`.
- **Suggestions:** a `suggestion` block renders as a plain code block.

### The finding that shapes the writer

A pending review's body is readable by other accounts through `GET /issues/{n}/timeline`, once for each time the body is set. The reviews list and the comment endpoints hide it. Superseded bodies remain after submit. A pending review also refuses an empty body. vuoro's reader ignores `review` timeline entries, so vuoro itself never shows someone else's pending text, but any API client sees it. Forgejo drafts therefore stay local until Submit, which publishes in one call. This is worth reporting upstream to Forgejo, which has not been done.

### Alternatives that lost

- **Mirror GitLab and keep a forge-side pending review:** it leaks general text and needs a placeholder body that would also leak.
- **Placeholder body, real body at submit:** the placeholder stays in the timeline for good.
- **Reply at the same line in the person's own review:** it starts a separate conversation.
- **Withdraw by deleting the review:** it erases the forge record (plan: the forge is the record).

### Left in the sandbox

The private repository `warricksothr/vuoro-sandbox`, with open PR #1 and closed PRs #2 and #3 (the draft probes) and #4 (from the fork). Also `reanzu`'s fork `reanzu/vuoro-sandbox`. Both are reusable for the writer's and stacker's live tests. Neither repository has been deleted.

## Summary

Every write was exercised live; docs/plan.md 5.3 and forge/forgejo/testdata/writes/spike.json record the result. Forgejo drafts stay local and Submit publishes in one call, because the timeline exposes pending review bodies. Replies post into the thread's review. Resolve and approval withdrawal are unsupported. Stacked draft PRs use a WIP: prefix, and forks show as head.repo differing from base.repo.
