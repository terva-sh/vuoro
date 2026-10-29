---
schema: 3
id: TKT-01M61JQ2PCA8DRN7GWKX94V16R
title: Load canned ask prompts from the config directory beside shipped ones
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - review
  - state
assignees: []
milestone: v1.0.0
parent: TKT-01M61JPA9TYM392M8XBAZ6RQV4
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-28T15:40:31Z
updated_at: 2026-10-29T14:46:17Z
created_by:
  id: agent:claude-code/ask-plan
  name: ""
updated_by:
  id: agent:claude-code/ask-20261029
  name: ""
extensions: {}
---

## Description

Canned prompts as files. vuoro ships `explain` (the summary prompt), `risks`, `test`, and `start-here`. The person adds Markdown files under `prompts/` in vuoro's config directory, each an optional front matter of `title`, `scope` (`change` or `since_review`), `tool`, and `model`, then the prompt. As presets do (D61), theirs supplement the shipped ones, one with a shipped name shadows it, and theirs sort first. A file that cannot be read or parsed is listed with its reason and never stops the others. Served at `GET /api/prompts` behind the window guard.

Design: docs/plan.md section 6.11, D68.

## Acceptance criteria

- [x] vuoro ships explain, risks, test and start-here
- [x] A prompt file's front matter sets title, scope, tool and model
- [x] The person's prompts sort first, and one with a shipped name shadows it
- [x] A bad file is listed with its reason and the rest still load
- [x] GET /api/prompts is behind the window guard

## Implementation plan

- review/prompts.go: Prompt{Name,Title,Scope,Tool,Model,Text,Builtin}; shipped explain, risks, test, start-here. LoadPrompts(dir) reads dir/*.md: the name from the file name ([a-z0-9-]+), optional front matter between --- lines of title, scope (change|since_review), tool, model. Unknown keys, a bad scope, an empty body, or an unreadable file become a PromptError{File,Reason}, and the rest load. Theirs sort first by name, a shipped name is shadowed, then the shipped ones in order. A missing directory is no prompts of theirs and no error.
- app: GET /api/prompts answers {prompts, errors}, reading config.Dir()/prompts on each call (App.PromptsDir overrides it for tests). Behind the window guard.
- tests: parse, shadow, order, bad files listed, missing directory; the route and the guard.

## Notes

**agent:claude-code/ask-20261029** at 2026-10-29T14:28:36Z

vuoro review round 1 (run 20261029-142410-2a1e0e) found two things here. The shipped test prompt is renamed manual-test, because section 6.12 uses test for conversations that run code; criterion 1's 'test' now means manual-test. And a prompt's scope was parsed but never applied: startAsk now applies it through fromPrompt (a change prompt drops the baseline, a since_review prompt refuses without one, no scope follows the tab), tested by app TestAPromptsScopeDecidesTheRange. Criterion 2 was ticked before scope did anything; it is true as of the fix commit.

## Summary

Landed in review/prompts.go and app/prompts.go. vuoro ships explain, risks, manual-test (renamed from test after review round 1), and start-here. LoadPrompts reads prompts/*.md beside the configuration file in use (App.PromptsDir overrides it), on every GET /api/prompts so an edited file shows without a restart. A file's optional front matter takes title, scope (change or since_review), tool, and model; anything else, a bad scope, an unclosed front matter, an empty prompt, a file name outside [a-z0-9-], or an unreadable file is listed in errors with its reason while the rest load. The person's prompts sort first by name and shadow a shipped one of the same name (D61). Tests: review TestPromptsAreShippedAndThePersonsOwnSortFirst and TestNoPromptsDirectoryIsOnlyTheShippedOnes; app TestThePromptListNamesTheFilesItCouldNotUse and TestThePromptListStaysBehindWindowGuard. Note for TKT-01M63YYXYAS9VXS4GWR19BJ77T: runs: true is an unknown field today and is reported as such until that ticket adds it.
