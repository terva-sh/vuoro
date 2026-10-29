---
schema: 3
id: TKT-01M61JPA9TYM392M8XBAZ6RQV4
title: "Ask about a change: canned prompts and questions, kept and searchable"
type: epic
status: ready
status_reason: null
priority: normal
due_on: null
labels:
  - review
  - agent
assignees: []
milestone: v1.0.0
parent: null
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-28T15:40:06Z
updated_at: 2026-10-29T13:59:18Z
created_by:
  id: agent:claude-code/ask-plan
  name: ""
updated_by:
  id: agent:claude-code/ask-plan
  name: ""
extensions: {}
---

## Description

Every change gets an Ask tab: canned prompts such as "Explain this change to me" and a free-form question, each answered by a read-only agent the person can keep talking to. The agent reads the head's files, vuoro's clone's git history, and the change's description, threads, and pipeline through vuoro-served read-only tools, and has no shell and no writing tool. Canned prompts are shipped and the person's own, as Markdown files in vuoro's config directory. Every conversation is kept until the person deletes it, and an Asked tab on the home page lists and searches them across all changes. Ask replaces the Summary tab.

A test conversation sits beside them: the same tab and kept conversations, answered in the editor's sandbox by an agent that may run the code and has nothing to push, for questions such as what a dependency bump breaks.

Design: docs/plan.md sections 6.11 and 6.12. Decisions: D68 and D69.
