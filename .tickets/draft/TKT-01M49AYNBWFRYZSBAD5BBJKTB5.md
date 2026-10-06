---
schema: 3
id: TKT-01M49AYNBWFRYZSBAD5BBJKTB5
title: Split the frontend into reusable components and modules
type: epic
status: draft
status_reason: null
priority: normal
due_on: null
labels:
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
created_at: 2026-10-06T19:27:23Z
updated_at: 2026-10-06T19:27:23Z
created_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
updated_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
extensions: {}
---

## Description

A review of web/src on 2026-10-06 found it split by screen, about 4,000 lines in 17 modules, with shared diff, badge, markdown, and writer pieces. Reuse is missing elsewhere. Five text editors are hand-built, run.tsx holds 15 components across four concerns in 781 lines, the file browser beside a diff is wired twice, async error handling repeats 21 times, polling loops are written four times, and dialogs are built twice. Each child extracts one piece and changes no behaviour, and the existing tests hold that.
