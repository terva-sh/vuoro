---
schema: 3
id: TKT-01M4ES1E4BJAZ78GB96H20W1RD
title: Send the editor's fixes back as suggestions on the review's comments
type: epic
status: done
status_reason: null
priority: high
due_on: null
labels:
  - review
  - agent
  - ui
assignees: []
milestone: null
parent: null
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-08T22:09:46Z
updated_at: 2026-10-08T23:09:49Z
created_by:
  id: agent:claude/editor-suggestions-2026-10-08
  name: ""
updated_by:
  id: agent:claude/editor-suggestions-2026-10-08
  name: ""
extensions: {}
---

## Description

D59, and docs/plan.md section 6.3, "Fixes as suggestions". A change author asked for inline suggestions instead of a stacked merge request, so they could see which changes were made to their own. The editor's work can now go back either way. The first live test is a GitLab merge request in flywheel-io/infrastructure/terraform.

## Summary

All three tickets landed on branch tickets/editor-suggestions. Not yet seen live: the first live test is the terraform MR.
