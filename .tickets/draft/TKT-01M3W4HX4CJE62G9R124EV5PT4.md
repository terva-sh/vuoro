---
schema: 3
id: TKT-01M3W4HX4CJE62G9R124EV5PT4
title: Forgejo posting through the shared forge boundary
type: task
status: draft
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
updated_at: 2026-10-06T02:00:31Z
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

- [ ] Comment, reply, approve, request changes, and submit work against a real pull request
- [ ] Human-directed writes refresh forge facts and the shared inbox turn; agents have no posting route.
- [ ] Pending comments are created, edited or replaced, deleted, and submitted as one review, as the spike's matrix says

## Notes

**agent:codex/forgejo-20261002** at 2026-10-03T00:44:56Z

The owner brought the open-by-URL read/review slice into v0.1 on 2026-10-02, tracked as TKT-01M3ZKGRZBBS869259MBA818NQ (Open Forgejo PRs through the shared forge boundary). This ticket retains discovery/inbox and human-directed writes for v1.0, including the existing live GitLab-write dependency. The new slice proceeds independently; it does not claim this ticket's inbox or posting criteria.

**agent:claude/forgejo-parity-20261006** at 2026-10-06T01:51:05Z

Re-parented under TKT-01M47EFQ400T20P79H9X152NAZ (Forgejo parity: posting, verdicts, and the editor's push) on 2026-10-06. Its earlier prerequisites (GitLab writes, Forgejo open-by-URL, inbox view) are all done. It now waits on TKT-01M47EG9BHW0HBVFBTSBB1915X (Prove which Forgejo review writes round-trip against a sandbox PR), TKT-01M47EG9CSRDZ2HKANYN30XBAY (Carry the person's verdict on submit), and TKT-01M47EG9E07Y4N1J34S5VDDC5N (Show only the write actions a forge supports), because request changes is not in forge.Writer and the Forgejo API has no review-comment edit, thread reply, or resolve route in its swagger. The editor's push is a sibling ticket, TKT-01M47EG9F7HZ7C3EPYF2TJMEYV, so posting does not wait on it.

**agent:claude/forgejo-parity-20261006** at 2026-10-06T02:00:31Z

From the spike (TKT-01M47EG9BHW0HBVFBTSBB1915X, docs/plan.md 5.3): Forgejo drafts stay local, because the issue timeline exposes a pending review's body to other readers. Submit is one POST /pulls/{n}/reviews with the verdict, the joined general drafts as the body, and comments[]. If a pending review already exists, perhaps started in the web UI, that call merges into it and replaces its body, so show its comments as part of what Submit publishes. Replies post into the thread's own review at its path and position after the review. They publish one by one, so handle a partial failure explicitly. Resolve and approval withdrawal are unsupported.
