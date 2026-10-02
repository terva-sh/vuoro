---
schema: 3
id: TKT-01M3Z7WCX5A15ZC1C6715QWT3Q
title: Hand kept findings to a fixing agent as a Markdown report
type: task
status: done
status_reason: null
priority: high
due_on: null
labels:
  - state
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
updated_at: 2026-10-02T21:42:48Z
created_by:
  id: agent:claude/vuoro-mvp-1
  name: ""
updated_by:
  id: agent:claude/vuoro-mvp-1
  name: ""
extensions: {}
---

## Description

D31. The kept findings of a run as a Markdown brief for an agent that fixes
them. For each finding it gives the change, the head, file and line, severity,
confidence, title, description, evidence, and the suggestion if there is one.
The UI copies it with one button, and vuoro writes it to runs/<id>/report.md
and shows the path.

## Acceptance criteria

- [x] The report lists every kept finding and nothing else, which a test proves
- [x] One button copies it, and the file in the run holds the same text

## Implementation plan

review.Report(RunView) builds the Markdown from kept drafts only, ordered by severity, with the person's mark winning over the verifier's and evidence in a fence longer than any backtick run inside it. Manager.WriteReport writes it to runs/<id>/report.md; POST /api/runs/{id}/report answers with the text and the path. The run view's button copies it, falling back to a read-only textarea when the clipboard refuses.

## Notes

**agent:claude/vuoro-mvp-1** at 2026-10-02T21:42:48Z

Checked in the isolated copy of the !144 run with two findings marked kept by hand in the copy's record (never through Keep, which posts): the report was written to the run's report.md and shown. The preview browser refused the clipboard, so the fallback textarea was what showed; the copy itself has not been seen to work in tuohi's window. The suggestion block (TKT-01M3Z7WCQF2NTTV9312ZZQYTRS) is not in the report yet, because suggestions do not exist yet; that ticket adds it.

## Summary

Kept findings make a Markdown hand-off report for a fixing agent, written to runs/<id>/report.md and copied with one button, with a by-hand fallback when the clipboard refuses.
