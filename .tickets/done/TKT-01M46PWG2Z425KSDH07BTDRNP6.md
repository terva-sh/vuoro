---
schema: 3
id: TKT-01M46PWG2Z425KSDH07BTDRNP6
title: Draft suggestions with findings, and ask for one in a small session
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
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-05T18:58:11Z
updated_at: 2026-10-05T19:06:49Z
created_by:
  id: agent:claude-code/suggestions-20261005
  name: ""
updated_by:
  id: agent:claude-code/suggestions-20261005
  name: ""
extensions: {}
---

## Description

Asking for a suggestion today resumes the session that drafted the finding. For a merged finding that is the synthesizer, whose context holds the whole diff and every reviewer's drafts, roughly 100k tokens. Every model call of the suggestion turn re-reads that context. On the owner's runs (2026-10-02 Sonnet, 2026-10-05 Opus), each suggestion took 2–3 tool calls and 8–18s once the session was live. A session that had ended had to be resumed first, and after five minutes the prompt cache has expired too.

Instead:

1. Reviewers attach a suggestion in propose_draft for critical, high, and medium findings when the fix fits inside the anchored head lines (about 20 lines at most). Low and info findings, and fixes that need lines outside the anchor or a design change, get none.
2. The synthesizer carries a source's suggestion into the merged finding when the merged anchor still covers it.
3. The verifier reads the suggestion too, and its reason says when the suggestion would not fix the finding.
4. "Ask for a suggestion" stays for the remaining findings, but starts a small fresh session given only that finding and its file, with permission to set that one draft's suggestion. It no longer resumes the drafting session.

This supersedes D31's "when the person asks" (decision D37).

## Acceptance criteria

- [x] A reviewer's critical, high, or medium finding with a local fix arrives with a suggestion, and low or info findings do not
- [x] A merged finding keeps a source's suggestion when its anchor covers it
- [x] The verifier's reason says when a suggestion would not fix the finding
- [x] Asking for a suggestion starts a fresh session that can set only that draft's suggestion, and does not resume the drafting session
- [x] D37 records the change, and docs/plan.md says how suggestions are drafted

## Implementation plan

1. Prompts (review/profile.go): reviewers attach a suggestion on critical, high, and medium findings when a replacement of the anchored head lines (about 20 at most) fixes it, and none on low or info. The synthesizer carries a source's suggestion when its merged anchor covers the same lines, and otherwise rewrites or drops it. The verifier's reason says when a suggestion would not fix the finding. The suggestion field description in drafts/mcp.go matches.
2. drafts: a new role, Suggester, with drafts.Session.Target naming the one draft it may touch. Its only tool is set_suggestion {id, suggestion, anchor?}, which goes through Store.Suggest. Suggest changes only the suggestion and, optionally, a widened head-side anchor on the same path, of a pending, undecided draft. It bumps the revision.
3. review/run.go: AskSuggestion starts a fresh session (profile "suggestion", the drafting session's tool and model, Role Suggester, Target the draft) whose first message is only the finding and where it sits. It reads the file itself; no diff, no transcript. It refuses while a suggestion session for that draft is still working. The session shows in the run like any other. view.go names the role and restores its spec and target. ReviewComplete ignores it.
4. The frontend tooltip text. Tests in drafts and review. D37, and docs/plan.md.

## Notes

**agent:claude-code/suggestions-20261005** at 2026-10-05T19:06:49Z

Live check on 2026-10-05 with `vuoro review -profile code -model sonnet` against a throwaway repo with two planted bugs.

The first run, before the boundary rule: both high findings carried a suggestion ($0.09 total). Both ranges stopped one line short of the function's closing brace and started on its doc comment, so applying either would have left `}` twice and removed the comment. The verifier called one of them correct.

Added: `drafts.Boundary` (`suggestion.boundary`), which refuses a suggestion whose first or last line repeats the non-blank line just outside its range at the head. It's wired through `Store.CheckSuggestion` to the worktree via `os.Root`. The reviewer, suggester and verifier prompts now spell out that the range is inclusive.

The second run: both suggestions covered exactly their functions (10–12 and 15–23) and would apply cleanly. The verifier confirmed both, and checked the suggestions. Cost $0.07.

Not exercised live: the desktop "Ask for a suggestion" path with a fresh Suggester session. Tests cover it with the fake driver: a fresh spec, no resume, only read_draft and set_suggestion, and nothing sent to the drafting session.

## Summary

Reviewers now draft suggestions on critical/high/medium findings with a local fix, the synthesizer carries them, and the verifier checks them. Ask for a suggestion starts a fresh Suggester session limited to read_draft and set_suggestion on one draft, instead of resuming the ~100k-token drafting session. A boundary rule refuses suggestions that would duplicate the line outside their range. D37 records it, and docs/plan.md is updated.
