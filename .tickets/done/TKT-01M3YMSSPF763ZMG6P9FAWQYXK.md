---
schema: 3
id: TKT-01M3YMSSPF763ZMG6P9FAWQYXK
title: "Open a GitLab change by URL: metadata, diff refs, and threads"
type: task
status: done
status_reason: null
priority: high
due_on: null
labels:
  - forge
  - gitlab
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies:
  - TKT-01M3W4HWJZ1013S2MPZ69B72AW
  - TKT-01M3W4HWM6V2F20KNT03BSBNZY
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-02T15:47:50Z
updated_at: 2026-10-02T19:05:26Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-mvp-1
  name: ""
extensions: {}
---

## Description

The first slice of the GitLab adapter, and the orchestrator's entry point:
given a merge request URL on any GitLab host, read its metadata, diff refs
(`base_sha`, `start_sha`, `head_sha`), diff, discussions, approvals, and the
person's draft notes. Discovery of every change the person is involved in is
TKT-01M3W4HWNGYDHGXMSE91HKGTNT (GitLab adapter: discover the changes I review
and wrote), which builds on this.

`~/Workspace/code-reviews/tools/fetch_mr.py` handles GitLab's truncated-diff
fallback, and it is worth reading first.

## Acceptance criteria

- [x] A merge request URL on gitlab.com opens with its diff and threads
- [x] A truncated diff falls back to reading the files, and a test proves it from fixtures
- [x] No token appears in any log line or error message, and a test proves it

## Implementation plan

forge holds the neutral model from docs/plan.md section 4 and ParseGitLabURL.
forge/gitlab reads one merge request with the embedded GraphQL query (metadata,
diff refs, reviewers with their review state, diff versions, and paginated
discussions), then reads the diff and the person's draft notes over REST. A
diff that GitLab collapsed or truncated is rebuilt from the raw files at base
and head with forge/patch, a small Myers diff. A binary or oversized file is
listed as Omitted. The token rides only in the Authorization header,
redirects are refused, and server messages are scrubbed of it. An empty token
reads anonymously, which public projects allow.

## Notes

**agent:claude/vuoro-mvp-1** at 2026-10-02T19:05:26Z

Anonymous REST works on public projects for /diffs and the raw files, but not
for /draft_notes, which answers 401. So an anonymous client skips drafts.
TestLive (VUORO_LIVE_GITLAB=1) opens gitlab-org/cli!3991 on gitlab.com
anonymously, and it passed on 2026-10-02 with 2 files, 7 threads, 3
reviewers, and 2 pushes. Approvals come from the reviewers' review state and
from the system notes, per D30. No live run has used a token yet, because
that needs the owner's PAT in the age store.

## Summary

forge (model and URL parsing), forge/patch (unified diffs), and forge/gitlab (GraphQL read, REST diffs with the truncated-file rebuild, drafts). Fixture tests cover the conversion, the rebuild, and the token's absence from errors. A gated live test opens a public gitlab.com MR.
