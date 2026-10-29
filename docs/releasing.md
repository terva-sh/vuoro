# Releasing vuoro

How a vuoro release is made (D74, docs/plan.md section 9.1). It follows
git-ticket-canvas's `docs/releasing.md`.

## Who decides what

An agent may prepare a release and run every local check. A person chooses
the version, writes or approves the tag's message, and pushes the tag. No
recipe here tags this checkout or pushes anything.

GitHub publishes every release, from a `v*` tag pushed to it, and runs
nothing else: per-branch and pull-request CI stays on the internal runners
(D52). The tag's annotated message becomes the release notes.

## Local checks

These need goreleaser v2 and Python 3 on `PATH`, besides what `just ci`
needs.

```sh
just ci                 # the gate, including the notices check
just release-check      # goreleaser check, and the notices
just release-rehearse   # tag a disposable clone of HEAD, build, verify
```

`just release-rehearse` builds the committed HEAD in a fresh clone, tagged
`v0.0.0-rehearsal`, and runs `scripts/verify-release.py` on its five
archives: the checksums, the documents in each archive, the module, commit,
target and tag every binary records, and the native binary's `-version`. It
removes the clone and publishes nothing. `just release-snapshot` builds the
working tree instead, and needs a full clone rather than a linked git
worktree, whose binaries record no commit.

When a dependency changes, run `just third-party-licenses` and commit the
new `THIRD_PARTY_LICENSES`; the gate fails until you do.

## The release

1. `main` is green on the internal CI, and `just release-rehearse` passes on
   it. The tickets in the release's scope are done or moved, with reasons.
2. The person chooses the version and creates an annotated tag on that
   commit. Its message is the release notes: what changed since the last
   release, and any known gap. Build a binary from a fresh clone of the tag
   with `go build` and check that `vuoro -version` names it.
3. Push `main` and that one tag to GitHub: `git push origin main vX.Y.Z`.
4. Watch the release workflow on GitHub. It publishes only archives that
   passed verification, through a draft made public once every asset is up.
5. Download the archive for your platform from the release, check it
   against `checksums.txt`, and run `vuoro -version`. Record the tag, the
   commit, and what was checked on the release's ticket.

If the workflow fails, the tag has no release, or a draft to inspect and
delete. Never move a published tag to repair a release; fix it in the next
one.
