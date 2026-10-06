---
schema: 3
id: TKT-01M47EG9E07Y4N1J34S5VDDC5N
title: Show only the write actions a forge supports, from its capabilities
type: task
status: draft
status_reason: null
priority: normal
due_on: null
labels:
  - forge
  - ui
assignees: []
milestone: v1.0.0
parent: TKT-01M47EFQ400T20P79H9X152NAZ
origin: null
dependencies:
  - TKT-01M47EG9BHW0HBVFBTSBB1915X
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-06T01:50:57Z
updated_at: 2026-10-06T02:00:31Z
created_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
updated_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
extensions: {}
---

## Description

`forge.Capabilities` is `{Drafts, Resolve, Approve}`. Forgejo will support part of what GitLab does: the Forgejo posting spike (TKT-) decides which part. Possible gaps are replies, editing pending comments, resolving, withdrawing an approval, verdicts, and suggestions. The UI must show only what this forge, with these credentials, can do, and must stop special-casing a forge by name. `web/src/change.tsx` hardcodes "Forgejo posting is not available yet" today.

Extend the capability set to the granularity the spike shows is needed. Drive every write control from it, and make the read-only explanation come from capabilities and configuration rather than from `kind`. A hidden action is also refused server-side, with `forge.ErrUnsupported`.

## Acceptance criteria

- [ ] Capabilities cover every write the spike found Forgejo cannot do
- [ ] No web/src control or message branches on forge kind to decide what can be written
- [ ] A write the forge cannot do is refused server-side with ErrUnsupported, with a test

## Notes

**agent:claude/forgejo-parity-20261006** at 2026-10-06T02:00:31Z

From the spike, Forgejo's capabilities: drafts yes, but local only; replies yes, published separately at Submit; resolve no; approve yes; withdraw approval no; request changes yes; applicable suggestions no, so a suggestion posts as a plain fenced replacement; own-PR verdicts no. GitLab's withdraw-approval and resolve stay yes. The flags need to separate approve from withdraw, and to say whether a suggestion can be applied.
