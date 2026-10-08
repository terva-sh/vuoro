---
schema: 3
id: TKT-01M3W4HWB551REE766G3TE2Y7D
title: Scaffold the Go module, justfile gate, and CI without cgo
type: chore
status: done
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
updated_at: 2026-10-08T14:10:37Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/forgejo-parity-20261006
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

- [x] go.mod declares github.com/terva-sh/vuoro and the chosen go line, recorded in AGENTS.md
- [x] just ci runs gofmt, go vet, go test -race, and git ticket check, all with CGO_ENABLED=0
- [x] just cross builds linux, darwin, and windows on amd64 and arm64
- [ ] Forgejo and GitHub workflows run the same steps as just ci

## Implementation plan

Copy the shape of tuohi's justfile and CI, which is the closest reference for a cgo-free Go module on the 1.26 line.

- go.mod: `module github.com/terva-sh/vuoro`, `go 1.26.0`, tuohi's line. AGENTS.md records the line and why it is not terva's 1.27.
- A minimal `main.go`, so that vet, test, and cross have a package. It prints the version with `-version` and says that the window comes later. The walking skeleton replaces it.
- justfile: export `CGO_ENABLED=0`, add `fmt`, `fmt-check`, `vet` (this platform, darwin, and windows), `test`, `cross` (linux, darwin, and windows on amd64 and arm64), `build`, and `install`. `ci` runs vet, fmt-check, test, cross, and tickets-check.
- The race detector: with cgo off it works on darwin and refuses on linux ("-race requires cgo", measured on 2026-10-02). `just test` adds `-race` on darwin only. Forgejo's Linux container runs the tests without it, and the GitHub macOS runner runs them with it. The cgo-free rule outranks the race detector, so the Linux run does not turn cgo on to get it.
- `.forgejo/workflows/ci.yml`: golang:1.27-alpine from the internal registry, as tuohi does, because the registry is curated and has no proven 1.26 image. The steps are inline, because the container has no just. It installs git-ticket v0.24.0, the plan's pin.
- `.github/workflows/ci.yml`: a matrix over ubuntu, macos, and windows, with setup-go from go.mod so the 1.26 minimum is what gets tested, and the same steps. Race runs on macOS.

## Notes

**agent:claude/vuoro-bootstrap** at 2026-10-02T14:39:34Z

The Go line depends on TKT-01M3W4HWW7FHHC6VD5M0S9MV90 (Decide how vuoro embeds terva for agent actions). If agent actions call terva rpc as a subprocess, vuoro never imports terva and can stay on Go 1.26, tuohi's line. Start at 1.26 and raise it only if that spike picks the in-process SDK or engine.

**agent:claude/vuoro-bootstrap** at 2026-10-02T15:16:41Z

Supersedes the earlier note on the Go line. On 2026-10-02 the owner decided that vuoro drives terva and Claude Code as subprocesses and imports neither. Nothing pulls the line up to terva's 1.27, so start at Go 1.26, tuohi's line, which this workstation already has.

**agent:claude/vuoro-mvp-1** at 2026-10-02T16:22:57Z

The race detector without cgo works on darwin and refuses on linux (`go: -race requires cgo`), measured on 2026-10-02 with Go 1.26.1. `just test` adds `-race` on darwin only, and the GitHub workflow's macOS leg is the CI run that has it. The Forgejo gate runs the tests without race. docs/plan.md section 9 now says so.

Criterion 2 is ticked because `just ci` on this macOS workstation runs every step it names, race included. On linux the same recipe runs the tests without race.

Criterion 4 is not ticked. Both workflows are written, and their YAML parses. The Forgejo steps passed, word for word, in a docker.io golang:1.27-alpine container, standing in for the internal registry's image. But neither workflow has run on its forge, because the repository has no remote. Tick it after the first green run on Forgejo and on the GitHub mirror, after TKT-01M3YMST0M467K1FB4J5B2C7B2 (Move vuoro to terva-sh/vuoro on Forgejo once the skeleton lands).

`.gitattributes` now pins `*.go` to LF, so a Windows checkout does not fail the gofmt check on CRLF.

**agent:claude/forgejo-parity-20261006** at 2026-10-08T14:10:37Z

2026-10-08 audit: the unticked box "Forgejo and GitHub workflows run the same steps as just ci" stays unticked on purpose. Forgejo's workflow runs those steps inline, but D52 (2026-10-07) removed GitHub CI, so the GitHub half no longer describes anything. Nothing remains to do here.

## Summary

go.mod is github.com/terva-sh/vuoro on go 1.26.0, recorded in AGENTS.md. just ci runs vet (three GOOS), gofmt, go test (with -race on darwin), cross builds for six targets, and the ticket check, all with CGO_ENABLED=0. A minimal main with -version gives the gate a package until the walking skeleton replaces it. The Forgejo and GitHub workflows mirror the gate. They have not run on a forge yet, so criterion 4 stays open until the first run after the Forgejo move.
