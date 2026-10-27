---
schema: 3
id: TKT-01M5ZDR759JGEN663FWTQ707ZG
title: Add a GitLab project by search or URL, and read all its open changes
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - gitlab
  - forge
  - state
assignees: []
milestone: v1.0.0
parent: TKT-01M5ZDQ3ZB0K6KMW2CB8NM183S
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-27T19:35:16Z
updated_at: 2026-10-27T20:06:18Z
created_by:
  id: agent:claude-code/project-triage
  name: ""
updated_by:
  id: agent:claude-code/project-triage
  name: ""
extensions: {}
---

## Description

The read side of project triage on GitLab (plan section 6.9, D64). A person adds a project from a configured GitLab account, by searching (`GET /projects?search=…&membership=true`, with `search_namespaces=true` to match the group) or by pasting a project URL (`GET /projects/:id` on the URL-encoded path), and vuoro keeps the list in the state directory. `forge.ProjectLister`, optional beside `Provider`, returns the project's complete open membership with update stamps.

Read the changes with GraphQL `project.mergeRequests(state: opened)`, paged by cursor like the inbox's fact reads. It gives `draft`, `conflicts`, `detailedMergeStatus`, `headPipeline`, labels, reviewers, assignees and discussions in one query. REST's list omits `head_pipeline`, so it would cost one more read per change. Dependencies (`blockingMergeRequests`, experimental since 16.5) exist on Premium and Ultimate only. On Free the field is absent, which means unknown, not "none", and the snapshot records which.

A new `projects` package holds project-scoped refresh and cached snapshots, following the inbox's rules: refresh only while the view is open, a failed refresh keeps the last snapshot, no poller, and a project too large to read in full is refused with a count rather than shown in part. It imports neither `agent` nor `tickets`, and its tests do no network I/O. Fixtures recorded from a real project are scrubbed before commit. The endpoints were checked against the GitLab documentation on 2026-10-27 (plan section 6.9); a live read against a real project confirms them.

## Acceptance criteria

- [x] A project is found by searching a configured GitLab account, or by pasting its URL
- [x] Added projects survive restart, in the state directory and never in the config file
- [x] Every open merge request in the project is read, with pipeline, conflict, labels and blocking merge requests
- [x] A failed refresh keeps the last snapshot visible with a credential-free message
- [x] projects imports neither agent nor tickets, and a boundary test holds it

## Implementation plan

1. `forge/project.go`: `Project` (host, path, name, web URL) and the optional `forge.ProjectLister` beside `Provider`: `SearchProjects`, `Project` (one by path, which confirms the account can see it), and `ListOpen`, complete open membership as `[]Discovered` with update stamps. `forge.ProjectURL` parses a project URL; `adapters.ParseProjectURL` picks the kind like `ParseURL`.
2. `forge.Change` gains `Labels`, `Conflicts`, `Pipeline` (the head pipeline's status, lowercased), `BlockedBy` and `DependenciesKnown`, all `omitempty`. The inbox's snooze fingerprint zeroes them, so existing snoozes neither wake on upgrade nor on a pipeline finishing.
3. GitLab: `query.graphql` reads `conflicts`, `labels(first: 100)` and `headPipeline { status }`. `forge/gitlab/project.go` implements `ProjectLister` with REST (`GET /projects?search=&membership=true&simple=true`, `GET /projects/:path`, `GET /projects/:path/merge_requests?state=opened`, paged as `Discover` is) and a `DependencyReader` over GraphQL `blockingMergeRequests`; a GitLab without the field, or a Free project, answers unknown rather than none.
4. `projects` package: the person's projects in `projects.json`, a snapshot per project in `cache/projects/<key>.json`, `Add` (checked through `Project`), `Remove`, `Refresh` (one project, six reads at a time, at most 300 open changes or refused with the count, a failure keeps the last snapshot with a fixed message), `RefreshChange`, and `Search`. Imports neither agent nor tickets; the boundary test covers it.
5. `app`: `GET /api/projects`, `POST /api/projects/search`, `POST /api/projects/add`, `POST /api/projects/remove`, `GET /api/projects/{key}`, `POST /api/projects/{key}/refresh`, behind the guard like every route.
6. Tests from fixtures and a fake GitLab server; an opt-in live read against a real project.

## Notes

**agent:claude-code/project-triage** at 2026-10-27T20:06:18Z

Live read on 2026-10-27 (TestLiveProject, read-only over glab): drewshort/vuoro-sandbox (2 open) and flywheel-io/infrastructure/terraform (51 open) both list, read and answer dependencies. gitlab.com answered blockingMergeRequests on the personal Free namespace too, as an empty list, so Free on gitlab.com reads as known-none rather than unknown; that is still correct, since such a project cannot record a dependency. A self-managed GitLab before 16.5 refuses the field and reads as unknown. The live test transport sent GraphQL bodies with --input, which glab's graphql endpoint ignores, so TestLiveDiscovery had never exercised ReadFacts; it now passes the query and string variables as fields. The window has no UI for adding a project yet: that is TKT-01M5ZDR7JF3GD741SREJJGQ8WP.

## Summary

forge.ProjectLister and forge.DependencyReader, the GitLab adapter for both (REST project search, project read and open listing; GraphQL blockingMergeRequests), labels, conflicts and head pipeline read into forge.Change and left out of the inbox's snooze fingerprint, and the projects package with its routes under /api/projects. Fixture tests cover paging, refusal of partial or foreign lists, unknown dependencies, a failed refresh, and a project over 300 open changes; a live read confirmed the endpoints.
