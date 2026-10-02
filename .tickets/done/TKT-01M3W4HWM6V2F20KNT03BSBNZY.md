---
schema: 3
id: TKT-01M3W4HWM6V2F20KNT03BSBNZY
title: Decide credential handling and the local state format
type: spike
status: done
status_reason: null
priority: high
due_on: null
labels:
  - credentials
  - state
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
updated_at: 2026-10-02T18:49:08Z
created_by:
  id: agent:claude/vuoro-bootstrap
  name: ""
updated_by:
  id: agent:claude/vuoro-mvp-1
  name: ""
extensions: {}
---

## Description

docs/plan.md sections 5.1 and 8 leave two decisions open.

Credentials. The starting position is a configured `token_command`, run
without a shell and read from stdout, with an environment variable as fallback.
A token never goes in a config file, argv, or a log. Decide whether a keychain
is worth it for v0.1 without cgo, which means `security` on macOS and Secret
Service over D-Bus on Linux.

State. Decide the cache format. JSON files may be enough for one person's
inbox. `modernc.org/sqlite` is the cgo-free option if queries need it. Keep
derived state (the cache) separate from authored state (snoozes, notes), so
deleting the cache loses nothing the person wrote.

Decide the config file's location and shape: forges, hosts, usernames, token
sources.

## Acceptance criteria

- [x] docs/plan.md sections 5.1 and 8 record the decisions, with the open-question markers removed
- [x] The config file format is documented with an example that contains no secret

## Implementation plan

The owner picked the token command (D29). config/ parses a JSON config that refuses unknown fields, so a pasted token is refused. It runs token_command with no shell, falls back to token_env, and holds the token as a Secret that prints [redacted]. docs/plan.md sections 5.1 and 8 record the file location, the example, and the JSON state layout with derived and authored state apart.

## Notes

**agent:claude/vuoro-mvp-1** at 2026-10-02T18:49:08Z

config_test.go proves the following:
- The example in docs/plan.md 5.1 parses, and the test checks that the plan carries it verbatim.
- A token field is refused without being quoted.
- Arguments like $(echo hi) reach the token command unexpanded.
- The token never prints under fmt verbs, log, or JSON.
- A failed, empty, or multi-line command is reported without its output.
- The environment fallback works.

For the owner: store the token with 'pbpaste | ritual-secret set gitlab-pat --expires-days N', and write the config file at ~/Library/Application Support/vuoro/config.json. The GitLab ticket only needs read_api for discovery, but submitting a review needs the api scope.

## Summary

Tokens come from a token_command run without a shell, or from token_env, and are held in memory as a Secret that never prints (D29). The config is JSON in the platform config directory, documented with a secret-free example in plan 5.1. State is JSON files, laid out in plan 8 with the derived cache apart from authored annotations.
