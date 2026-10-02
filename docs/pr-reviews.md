# Pull requests and reviews

vuoro has two forges. Internal work goes through the internal Forgejo
(`origin`, `git.local.sothr.com/terva-sh/vuoro`), where `terva-review` runs.
GitHub (`github`, `github.com/terva-sh/vuoro`) is where agents working outside
the internal network push, and where the macOS and Windows legs of CI run. The
two meet at `main`, which `just sync-github` keeps equal on both. There is no
push mirror, because a push mirror force-pushes and would overwrite anything
merged on GitHub.

## Internal work, on Forgejo

1. Branch from `main`, commit, and push the branch to `origin`.
2. Open the pull request with `tea pulls create`, or through the API.
3. `.forgejo/workflows/ci.yml` is the gate. It runs the steps of `just ci`.
4. Request a model review when the pull request is ready. See
   [terva-review](#terva-review).
5. Post a disposition for each finding, and record it in the ticket, while
   the pull request is still open.
6. Merge on Forgejo, then run `just sync-github --yes` to fast-forward
   GitHub `main`. The GitHub workflow then runs on Linux, macOS, and Windows,
   including the window check on macOS. A failure there is a regression to fix
   on Forgejo.

## Pull requests on GitHub

Work that cannot reach the internal forge opens its pull request on GitHub,
where `.github/workflows/ci.yml` runs. `terva-review` does not run there.
After a merge on GitHub, run `just sync-github --yes` to fast-forward Forgejo
`main`. To give that work a model review, push its branch to `origin` and open
a Forgejo pull request for it before merging anywhere.

## Keeping main equal

`just sync-github` fetches both `main` branches and fast-forwards the one
that is behind. Without `--yes` it prints the commits it would push and stops.
It never force-pushes. If both forges merged something since the last sync,
the recipe stops with both heads. Then:

1. Branch from `origin/main` and merge `github/main` into the branch.
2. Land the branch through a Forgejo pull request.
3. Run `just sync-github --yes`.

## terva-review

`.forgejo/workflows/terva-review.yml` runs the reviewer from
`terva-sh/terva-action-code-review` on internal pull requests and publishes
its result as the `terva-review/code` status. It is advisory and does not
replace CI. Request a review when a pull request is ready, and again after
substantive fixes.

```sh
tea actions workflows dispatch terva-review.yml --ref main \
  --input pr=PR_NUMBER --input request-id=ready-review
```

Reuse a request ID to recover a delivery that failed. Change it to get a fresh
review of the same head. The dispatch may answer "unexpected end of JSON
input" even when the run was created, so check the Actions page before
retrying.

A maintainer can also comment `/terva review HEAD_SHA BASE_SHA` on the pull
request, with full lowercase SHAs. Only the logins listed in the workflow, in
both places, can trigger a review by comment.

The review does not read `web/dist/**` or `package-lock.json`. Both are
generated, a model cannot judge them, and they are what usually pushes a pull
request past the review's context. `just ci` checks that `web/dist` matches
its source.

### Carry

After commits that touch only `.tickets/`, dispatch with `--input carry=true`.
That copies the last review's result to the new head without a model run, or
refuses and names the path that needs a real review.

### Moving the pin

The workflow pins the reviewer image by digest. A new pin is its own pull
request, reviewed by the new pin from its own branch, as
terva-action-code-review's `docs/installation.md#adopting-a-new-pin`
describes.

A passing review is evidence, not permission to merge.
