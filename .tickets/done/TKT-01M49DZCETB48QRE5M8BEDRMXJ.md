---
schema: 3
id: TKT-01M49DZCETB48QRE5M8BEDRMXJ
title: Bring setup.tsx onto the shared frontend components
type: chore
status: done
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
created_at: 2026-10-06T20:20:12Z
updated_at: 2026-10-06T20:22:15Z
created_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
updated_by:
  id: agent:claude/frontend-modules-2026-10-06
  name: ""
extensions: {}
---

## Description

setup.tsx (setup in the window, D45) landed beside the shared components rather than on them. It repeats the error idiom five times beside three hand-rolled busy and error pairs, builds ForgeEditor's Save, Cancel, and error by hand, leaves Remove clickable while a removal runs, and puts its module comment after the imports. Its stylesheet already reads only --ui-* roles (D46).

## Acceptance criteria

- [x] ForgeEditor's test and save, Tools' check, and Forges' remove use useAction or errorText, and setup.tsx has no error idiom of its own
- [x] ForgeEditor is built on EditForm, which gains an optional Cancel and a hint on Save, with tests
- [x] Remove is disabled while a removal runs
- [x] The module comment opens the file
- [x] just ci passes

## Summary

setup.tsx now uses the shared components. ForgeEditor runs its test and its save as two useActions, so Test reads 'Testing…' only while testing and Save reads 'Saving…' only while saving; a failed test still clears the earlier pass. It is built on EditForm, which gained an optional Cancel (the first run has none) and a hint on Save, with a test. Tools' check and Forges' remove use useAction, and Remove and Keep are disabled, Remove reading 'Removing…', while a removal runs, with a test. The command parse error uses errorText, so setup.tsx has no error idiom of its own. The module comment opens the file. The stylesheet already read only --ui-* roles and is unchanged. The setup tests gained a click helper that flushes a few microtasks, because useAction sets state a few hops after the answer; no assertion changed.
