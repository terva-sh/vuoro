---
schema: 3
id: TKT-01M47EG9F7HZ7C3EPYF2TJMEYV
title: "Push the editor's fixes to Forgejo: own branch, or a stacked draft PR"
type: task
status: done
status_reason: null
priority: normal
due_on: null
labels:
  - forgejo
  - agent
  - review
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
updated_at: 2026-10-06T05:29:58Z
created_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
updated_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
extensions: {}
---

## Description

On GitLab the editor's fixes go back through `forge.Stacker`. D39 puts a stacked draft merge request onto someone else's change. D41 and D42 push to the branch of the person's own change. Both paths call `StackTarget`, and `app/editor.go` refuses a Forgejo change outright ("GitLab only, so far").

Implement `forge.Stacker` for Forgejo:

- **`StackTarget`** reads the PR's head branch and SHA, and refuses with `forge.ErrFork` when the head repository is not the base repository.
- **`OpenStacked`** opens a draft PR from the pushed branch onto the reviewed PR's head branch, and returns its web URL.

Remove the GitLab-only refusal. Make the editor's wording forge-neutral where it says "merge request" (`review.StackTitle`, `StackDescription`, `threadsMessage`); `changeName` already uses `#` on Forgejo.

The push itself stays vuoro's own git push with the person's transport, and happens only at their confirmed action. The editor never holds a Forgejo token.

## Acceptance criteria

- [x] Forgejo implements forge.Stacker, and StackTarget refuses a fork with ErrFork
- [x] On my own Forgejo PR the editor's accepted fixes push to its branch
- [x] On someone else's Forgejo PR the accepted fixes open a draft PR onto its head branch
- [x] Both routes are validated live against a sandbox PR

## Implementation plan

### Forgejo
- **The Stacker.** A new `forge/forgejo/stack.go` implements `forge.Stacker` (`var _ forge.Stacker = (*Client)(nil)`).
- **StackTarget.** It GETs `/pulls/{n}` and returns `head.ref` and `head.sha`. It refuses with `forge.ErrFork` when `head.repo` is missing (a deleted fork) or its `full_name` differs from `base.repo`'s. A head branch that fails `forge.ValidBranch` is refused too.
- **OpenStacked.** It POSTs `/repos/{o}/{r}/pulls` with `head` set to the pushed branch, `base` set to the reviewed PR's head branch, the title, and the body. It returns `html_url`.
  - Forgejo marks a draft by a `WIP:` title prefix (the spike). The prefix is added unless the title already has `WIP:` or `[WIP]`.
  - If the reply says `draft: false` (an instance configured with other prefixes), the PR still opened. The error says that, so the push isn't retried into a duplicate.

### The app
- **The refusal.** `app/editor.go` loses the GitLab-only refusal. A client without `forge.Stacker` still gets ErrUnsupported, in forge-neutral words.
- **ciFile by forge.** It takes the forge kind. GitLab keeps `.gitlab-ci.yml` and `.gitlab/`. Forgejo flags `.forgejo/`, `.gitea/workflows/` and `.github/workflows/`, since Forgejo Actions falls back to the latter two.
- **Wording.** "The merge request needs a title" names the forge's word.

### Wording
- **review.** A `changeWord(v)` helper gives "pull request" or "merge request". threadsMessage, findingsMessage and PushEditor's "did not open" error use it.
- **Web.** A `changeWord(kind)` in `types.ts` is used throughout PushBar and PushDialog. Rebuild `web/dist`.

### Checks
- **Offline tests.**
  - The Forgejo stack test covers the target, the draft opened with its fields, an existing WIP prefix, a fork, a missing head repo, and a reply with `draft: false`.
  - An app test covers ciFile per forge.
  - The review wording tests cover a Forgejo run.
- **The live test.** `TestLiveStack` uses the sandbox through tea for the API, and git over SSH for the pushes.
  - StackTarget on the same-repo PR, and ErrFork on reanzu's fork PR.
  - The own-branch route pushes a commit onto the sandbox PR's head branch the way `Repos.PushEditor` does (`git push --quiet --no-verify <remote> sha:refs/heads/<branch>`), and checks that StackTarget's head moved.
  - The stacked route pushes a new branch and calls OpenStacked. It checks that the new PR is a draft with the reviewed PR's head branch as its base, then closes the PR and deletes the branch.
- **A gap.** The full UI flow with a live editor agent is not exercised. It needs a vuoro config and token on a test machine.

## Notes

**agent:claude/forgejo-parity-20261006** at 2026-10-06T02:00:31Z

From the spike: POST /repos/{o}/{r}/pulls with head=<branch> and base=<reviewed PR's head.ref> and a 'WIP: ' title prefix returns draft: true ([WIP] works too). A fork PR has head.repo.full_name different from base.repo.full_name, and head.repo.fork true, so StackTarget refuses it with ErrFork. Sandbox for the live test: warricksothr/vuoro-sandbox, plus reanzu's fork reanzu/vuoro-sandbox for the fork case.

**agent:claude/forgejo-parity-20261006** at 2026-10-06T05:17:36Z

Implemented and live-validated against the sandbox on 2026-10-06.

### What changed from the plan
- OpenStacked does not treat `draft: false` as an error. An error there makes PushEditor report "pushed, but did not open", and a retry would then open a second pull request. An instance with other WIP prefixes gets an ordinary pull request, which is still the person's to mark.

### Live: TestLiveStack
- **Setup.** It ran on warricksothr/vuoro-sandbox#1 as tea login `local`, with git over the PR's SSH clone URL.
- **The fork.** StackTarget refused the fork PR #4 (reanzu/vuoro-sandbox:feature) with ErrFork.
- **The stacked route.** It pushed vuoro/live-stack-20261006T051622Z, and OpenStacked opened #6. #6 was a draft from that branch onto `feature` at the pushed sha. The test then closed #6 and deleted the branch, and I confirmed both afterwards.
- **The own-branch route.** It fast-forwarded `feature` with the same commit, the same push shape as Repos.PushEditor, and StackTarget then reported the new head.
- **No regression.** TestLiveWrite still passes on the moved head.

### What was not exercised
The full UI flow with a live editor agent was not run, because this machine has no vuoro config or token. The code between the route and the forge (app.stack, Manager.PushEditor, Repos.PushEditor) is shared with GitLab and covered by review's editor tests. This change only removed the GitLab-only refusal there and made the wording and the CI-file list follow the forge.

### Leftover
The spike's branch vuoro/fix-1 is still on the sandbox. I left it alone.

**agent:claude/forgejo-parity-20261006** at 2026-10-06T05:24:45Z

terva-review run 74b890c3 (medium): an instance with other draft prefixes opens an ordinary PR, which vuoro still called a draft. Forgejo's API neither names the instance's prefixes nor sets draft another way, so vuoro can't guarantee one. It now reports it truthfully. OpenStacked returns the URL with forge.ErrNotDraft. PushEditor keeps the URL, so pushing again updates it rather than opening a second, and sets Pushed.NotDraft. The push bar links 'the pull request' and warns that it can be merged as it is, and should be marked as a draft on the forge. This supersedes the earlier note that OpenStacked ignores draft:false. Rejected: failing the push, because the URL would be lost and a retry would open a duplicate. Tests cover the adapter, openStacked, and the UI. TestLiveStack passes again (#7 opened and closed).

**agent:claude/forgejo-parity-20261006** at 2026-10-06T05:29:58Z

terva-review run f81dbe88 (low): TestLiveStack failed on ErrNotDraft before registering the cleanup that closes the new PR. It now registers the close whenever a URL came back, then fails. It passed live again (#8 opened and closed, only #1 left open).

## Summary

Forgejo implements forge.Stacker: StackTarget reads the PR's head branch and refuses a fork (or a deleted one) with ErrFork, and OpenStacked opens a WIP:-marked draft PR onto it. The editor's fixes now push on Forgejo by both routes, the GitLab-only refusal is gone, and the push dialog, the editor's task, and the CI-file warning follow the forge. TestLiveStack validates both routes against a sandbox PR.
