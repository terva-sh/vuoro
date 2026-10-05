---
schema: 3
id: TKT-01M3YMSVD4Y1ME5QST9BY818XD
title: Decide the editor agent's sandbox
type: spike
status: done
status_reason: null
priority: high
due_on: null
labels:
  - agent
  - sandbox
assignees: []
milestone: v1.0.0
parent: TKT-01M3YMSTYMFRDFXD4QSVWHMM8P
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-02T15:47:52Z
updated_at: 2026-10-05T20:42:02Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude-code/editor-20261005
  name: ""
extensions: {}
---

## Description

The editor agent runs the change's code, so its boundary matters, and it must
hold without cgo. The default position is a container (podman or docker) with
only the editor's worktree mounted, no forge credentials, and network limited
to the model endpoint and package registries. Compare it with each tool's own
sandbox (terva's lock, Claude Code's sandbox) and with what macOS offers
natively, and decide.

Reviewers are unaffected. They stay read-only, always.

## Acceptance criteria

- [x] docs/plan.md records the sandbox chosen and the alternatives rejected, with reasons
- [x] A recorded run shows the editor unable to reach a forge or read a credential

## Notes

**agent:claude-code/editor-20261005** at 2026-10-05T20:13:33Z

Findings are in docs/editor-sandbox.md. Probes ran as shell commands, with fake canaries only. The sandbox runtime (srt 0.0.78), with a vuoro-built environment, held every probe and still ran go test, the module proxy, and git commit. Podman with only the worktree mounted held the files, but it offers no network allowlist (none blocks the registries; the default reaches gitlab.com) and needs a per-project toolchain image. Inside Seatbelt, Claude cannot reach the Keychain ("Not logged in"), so the editor needs CLAUDE_CODE_OAUTH_TOKEN from `claude setup-token`, kept in the age store.

Recommended: srt with a per-run config written by vuoro, keeping the container as a later strong mode. That supersedes D20's container default, so it waits for the owner. AC 2, a recorded run of Claude itself inside the boundary, waits for the owner's token.

## Summary

Decided as D40. The editor runs locally under Claude Code with --restricted, sandboxed Bash (no home reads or writes, no unsandboxed retry, fail if unavailable, registries-only network), and an allowlisted environment. With an API key configured for the editor, srt wraps the whole process instead. Recorded runs of Claude itself against fake canaries were refused on Read, Grep, Glob, symlinks, home writes and gitlab.com, while go test and commit worked (docs/editor-sandbox.md). Not yet run: Claude under srt, which waits for a key.
