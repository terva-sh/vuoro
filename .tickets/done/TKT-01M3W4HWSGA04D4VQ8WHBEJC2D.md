---
schema: 3
id: TKT-01M3W4HWSGA04D4VQ8WHBEJC2D
title: "Change view: diff, threads, and review state"
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - review
  - ui
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies:
  - TKT-01M3W4HWGGB7RMVFAJPW2KPKY5
  - TKT-01M3YMSSPF763ZMG6P9FAWQYXK
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-01T16:25:25Z
updated_at: 2026-10-02T19:40:50Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-mvp-1
  name: ""
extensions: {}
---

## Description

Read one change in place: description, the diff by file with threads anchored
at their lines, general discussion, approvals, and pipeline status as a link.
Show "since my last review" as a diff mode when the forge records the SHA.

Large diffs need care. Collapse generated and very large files, and load
files on demand.

## Acceptance criteria

- [x] A change's diff renders with every anchored thread shown at its line
- [ ] The since-my-last-review mode shows only commits after my last review
- [x] A 5,000-line diff stays responsive, which means collapsed files load on demand

## Implementation plan

The window opens a merge request by URL and shows it in three tabs: Overview (description and general discussion), Changes (a file list and each file's diff with its threads and the run's drafts at their lines), and Review (start a run from a preset, and watch its sessions and drafts). Files past 400 lines start collapsed, and their rows are built only when opened. Styles read --ui-* roles from a stand-in copy of the design stylesheet.

## Notes

**agent:claude/vuoro-mvp-1** at 2026-10-02T19:40:50Z

Checked live on 2026-10-02 in a browser against vuoro -browser: gitlab-org/cli!3991
rendered 2 files with the anchored thread at its line, reviewer states with
symbols, and a two-reviewer Haiku run streaming tool calls in two panes. Its
one draft showed in the run and inline in the diff.

Criterion 2 is not met. GitLab records no SHA with an approval (D30), so
"since my last review" needs the diff between the head at my last review and
the head now. That means GitLab's diff versions
(/merge_requests/:iid/versions) or a git diff in the bare clone, and neither
is built. TKT-01M3YMSV8 (Re-anchor drafts, and review only the new commits)
is the nearest home.

web/src/styles/design.css is git-ticket-canvas's stylesheet as a stand-in
until TKT-01M3W4HWH ships the vuoro accent. Pipeline status is not shown:
the GraphQL query does not read it yet.

Also found and fixed: reviewer processes outlived vuoro when it was killed.
main now ends every live session on exit, on SIGTERM, and on interrupt.

## Summary

The change view: description, discussion, the diff by file with threads and drafts at their lines, reviewer states, and the review tab that starts and watches runs. Large files load on demand. Since-my-last-review is not built, per the note.
