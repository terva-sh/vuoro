---
schema: 3
id: TKT-01M3Z7WC760Q9864DGDRYAAETJ
title: "Make a large review readable: compact findings, Markdown, folding"
type: task
status: done
status_reason: null
priority: high
due_on: null
labels:
  - ui
  - review
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-02T21:21:18Z
updated_at: 2026-10-02T21:29:26Z
created_by:
  id: agent:claude/vuoro-mvp-1
  name: ""
updated_by:
  id: agent:claude/vuoro-mvp-1
  name: ""
extensions: {}
---

## Description

A real review (next-gen-compute!144: 47 files, 4 sessions, 15 drafts) is hard
to read. Every finding shows twice, once merged and once from its reviewer.
Every card is fully open, the four session panes push the findings below the
fold, and the description shows raw Markdown.

Make the findings a compact list: one row per finding, with its severity,
confidence, title, place, and decision, opened in place. Hide a merged
finding's sources under it, and keep what was not carried forward in its own
folded list. Show a triage count. Fold the session panes to one line each once
they are idle. Render descriptions, notes, and drafts as Markdown, without
ever parsing them as HTML, in a measure that reads well. Stop the file list
cutting paths off.

## Acceptance criteria

- [x] Each finding shows once, with its sources folded under it
- [x] Descriptions, notes, and drafts render as Markdown, and no input reaches the page as HTML, which a test proves
- [x] Idle session panes fold to one line, and open on a click

## Implementation plan

Render forge and agent prose with a small Markdown renderer of our own (web/src/markdown.tsx) that builds Preact elements and never sets HTML, so an MR author's text cannot become markup. Links are restricted to http, https, and mailto, plus project-relative ones, and GitLab refs !N and #N link to the project. Prose uses it everywhere.

The run view becomes a triage list: one collapsed row per shown finding (severity, title, file:line, decision), opening to the draft and its actions, with each reviewer's original folded under "What each reviewer wrote". Session panes fold to chips in a strip and open by default only while working. The file list shows the name with its directory below it.

## Notes

**agent:claude/vuoro-mvp-1** at 2026-10-02T21:29:26Z

Checked against the real next-gen-compute!144 run (copied into an isolated HOME, so the person's running window was untouched): the overview renders 202 Markdown elements and no raw `##` or `**`; the Review tab shows 7 one-line rows (37px each) and four ended sessions folded into chips; the file list keeps the comment badge on the name line.

web/src/markdown.test.tsx proves hostile input (script tags, javascript: links, onerror attributes, raw HTML) reaches the page only as text. Pane folding was checked in the browser, not by a test.

Also in this commit: a 404 on a change read anonymously now says that no forge is configured for that host, with a Go test.

## Summary

Findings are a one-line-per-row triage list with sources folded; descriptions, notes, and drafts render as safe Markdown; idle sessions fold to chips; the file list reads by name. Checked on the real !144 run.
