---
schema: 3
id: TKT-01M3W4HWB551REE766G3TE2Y7D
title: Scaffold the Go module, justfile gate, and CI without cgo
type: chore
status: draft
status_reason: null
priority: high
due_on: null
labels:
  - ci
assignees: []
milestone: v0.1.0
parent: TKT-01M3W4FGT1GY0X3TWJF81X3XWK
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-01T16:25:25Z
updated_at: 2026-10-01T16:25:25Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
extensions: {}
---

## Description

Create the Go module `github.com/terva-sh/vuoro` and grow `just ci` into the
gate that docs/plan.md section 9 describes, minus the frontend steps that the
frontend ticket adds.

Decide the `go` line here. terva declares `go 1.27.0`, so the module must too
once `assist` imports terva. This workstation had Go 1.26.1 at bootstrap. Either
install 1.27 or rely on `GOTOOLCHAIN=auto`, and say which in AGENTS.md.

Copy the CI shape from tuohi and git-ticket-canvas: `.forgejo/workflows/ci.yml`
as the gate, and `.github/workflows/ci.yml` on the mirror for macOS and Windows.
Install git-ticket at a pinned version for the store check, as the canvas CI
does.

## Acceptance criteria

- [ ] go.mod declares github.com/terva-sh/vuoro and the chosen go line, recorded in AGENTS.md
- [ ] just ci runs gofmt, go vet, go test -race, and git ticket check, all with CGO_ENABLED=0
- [ ] just cross builds linux, darwin, and windows on amd64 and arm64
- [ ] Forgejo and GitHub workflows run the same steps as just ci
