---
schema: 3
id: TKT-01M3YK0CM0SWMYXMM7A7M7J2QX
title: "Run a review: worktree, review profile, and validated findings"
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - review
  - agent
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies:
  - TKT-01M3YK0CH5RHPJJS7E0DX9SH8X
  - TKT-01M3YMSSPF763ZMG6P9FAWQYXK
  - TKT-01M3YMSSRC81M0BYHK15TW9VDY
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-02T15:16:29Z
updated_at: 2026-10-02T19:30:11Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-mvp-1
  name: ""
extensions: {}
---

## Description

vuoro runs reviews itself. For a change, check out the head in a worktree under
the state directory, compute the diff against the merge base, and start a
session with the chosen driver, a review profile, and the diff. The agent
reports findings as drafts through the drafts MCP server. Each finding has
severity, title, body, evidence, and an optional anchor (path, side, SHA, line
range).

The reviewer is read-only, always: the worktree, and nothing else (docs/plan.md
section 6.3). Profiles start as code, security, and documentation, built in,
with the person's presets in vuoro's own config. Repository-supplied
instructions come at v1.0.0.

terva-sh/terva-action-code-review is prior art: its `templates/`, its finding
and anchor shapes, and its rule that reviewer settings come from the base
commit. Borrow ideas, not code or schema.

## Acceptance criteria

- [ ] A review of a real GitLab change produces validated findings with either driver
- [x] An invalid finding goes back to the agent with the rule it broke, and a test proves it
- [ ] A repository's review settings are read at the base commit, never the head
- [x] The worktree is removed when the review is closed

## Implementation plan

review.Manager starts a run per change: a bare partial clone under repos/
(the MR head ref and the target branch, over the person's ssh or https), a
detached worktree at the head under worktrees/<run>, and one locked session
per reviewer in the preset, each with its own drafts token. The drafts server
now serves every run from one handler, and each token names its run's store.
A run's store checks anchors against the forge's diff. app serves the API
under /api/ (open a change, start, read, talk to, stop, and close a run), and
main wires it with the Claude Code driver. Presets, the person's profiles,
and the clone transport are optional config fields.

## Notes

**agent:claude/vuoro-mvp-1** at 2026-10-02T19:30:10Z

Live, 2026-10-02: one Haiku code reviewer on gitlab-org/cli!3991, opened
anonymously and fetched over https. It read the worktree, proposed three
drafts, and two were accepted. Its API connection dropped mid-turn; stop and
a follow-up message resumed the same session. Close removed the worktree and
kept both drafts in runs/<id>/run.json. A second run through the gated
review TestLive finished cleanly with no drafts, at $0.24.

Criterion 1 says "with either driver". It holds for Claude Code. terva cannot
be offered until its lockdown exists (D28, TKT-01M3YRE0E00TRVKXP8YVASHDGD),
so the box stays empty for the owner to narrow or keep.

Criterion 3 has nothing to read in the MVP. D14 moves repository-supplied
review settings to v1.0, and the lockdown already keeps the head's CLAUDE.md,
AGENTS.md, and hooks out of the session. When repository settings arrive,
read them with `git show <base>:<path>` from the bare clone, never from the
worktree. The box stays empty.

Not done here: runs do not survive a restart of vuoro (the record is
written, and nothing reads it back yet), and Claude Code's own transcript
under ~/.claude/projects is not deleted on close. Both belong to the history
work in plan section 8.

## Summary

Reviews run end to end on Claude Code: worktree from a partial clone, built-in and configured profiles, presets, anchors checked against the diff, and the API that drives runs. Proven by fake-driver tests and a live run on a public gitlab.com MR. Criteria 1 and 3 are left for the owner, per the note.
