---
schema: 3
id: TKT-01M4ES1EATMXPJR56FDF54630C
title: Apply suggestions on GitHub, with comments spanning several lines
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - github
  - forge
  - review
assignees: []
milestone: null
parent: TKT-01M4ES1E4BJAZ78GB96H20W1RD
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-08T22:09:46Z
updated_at: 2026-10-08T22:14:21Z
created_by:
  id: agent:claude/editor-suggestions-2026-10-08
  name: ""
updated_by:
  id: agent:claude/editor-suggestions-2026-10-08
  name: ""
extensions: {}
---

## Description

D59 offers suggestions on GitHub as well as GitLab. GitHub applies a ```` ```suggestion ```` block over the lines its comment spans, so a suggestion over more than one line needs a comment with a start line.

- `forge.Anchor` gains `StartLine`, the first new-side line of a comment spanning several lines. GitHub's CreateDraft passes `startLine` and `startSide` when it is set. GitLab ignores it, since its blocks carry their own offsets.
- GitHub's capabilities set `Suggestions`.
- A suggestion block is written in the forge's own syntax: `suggestion:-N+M` on GitLab, and a plain `suggestion` fence on GitHub. Keep anchors a reviewer's suggestion on GitHub as a comment spanning its range. The UI's keepText agrees with keepBody for both.

## Acceptance criteria

- [x] GitHub's capabilities set Suggestions, and a multi-line suggestion is kept as a comment spanning its lines, which a test proves
- [x] A suggestion block is written in each forge's own syntax, and keepText agrees with keepBody for both

## Summary

GitHub now sets Suggestions and SpanSuggestions. forge.Anchor gains StartLine, and GitHub's CreateDraft sends startLine/startSide for a RIGHT-side comment spanning lines. drafts.Suggestion writes a GitLab block (suggestion:-N+M) or, with span, GitHub's bare block. keep spans a suggestion's comment over its range on a span forge, refusing a range not inside one hunk (patch.Span). The UI's keepText takes span and agrees with keepBody on a shared vector. Tested: forge/github multi-line test, patch.Span, keepBody and spanLines, vitest keepText. Not yet seen live on github.com.
