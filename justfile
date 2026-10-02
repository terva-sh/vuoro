# vuoro dev tasks. `just` lists them.
#
# `just ci` runs the same steps as .forgejo/workflows/ci.yml, the gate for
# internal pull requests. That workflow keeps its commands inline because its
# container does not install just, so change both together.

set shell := ["bash", "-euo", "pipefail", "-c"]

# What ships is built without cgo, so everything here is built and tested that
# way (AGENTS.md).
export CGO_ENABLED := "0"

# Every GOOS/GOARCH pair that must build.
targets := "linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64"

# List available recipes.
default:
    @just --list

# Build vuoro for this platform from the committed web/dist.
build:
    go build -o vuoro .

# Install vuoro into GOBIN, or GOPATH/bin.
install:
    go install .

# Rewrite sources with gofmt.
fmt:
    gofmt -w .

# Fail if gofmt would change a file.
fmt-check:
    @diff=$(gofmt -l .); if [ -n "$diff" ]; then echo "gofmt issues in:"; echo "$diff"; exit 1; fi

# go vet for this platform, darwin and windows, since desktop code is per-OS.
vet:
    go vet ./...
    GOOS=darwin go vet ./...
    GOOS=windows go vet ./...

# Run the tests; accepts go test flags. With cgo off the race detector works
# on darwin and refuses on linux, so -race is added on darwin only.
[positional-arguments]
test *args:
    race=""; if [ "$(go env GOOS)" = darwin ]; then race="-race"; fi; \
    go test $race "$@" ./...

# Open the window and check that the page reached the server and Go. It
# needs a display, so `just test` skips it.
test-gui:
    race=""; if [ "$(go env GOOS)" = darwin ]; then race="-race"; fi; \
    TUOHI_REQUIRE_GUI=1 go test $race -count=1 -run TestCheck -v .

# Build and open the window; forwards its arguments.
[positional-arguments]
run *args: build
    ./vuoro "$@"

# Build every package for every target.
cross:
    for t in {{targets}}; do \
        echo "== $t"; \
        GOOS=${t%/*} GOARCH=${t#*/} go build ./...; \
    done

# Install the locked frontend dependencies.
web-setup:
    npm ci

# Typecheck and build web/src into web/dist; commit the result.
web-build:
    npm run build

# Check the frontend's TypeScript without building it.
web-typecheck:
    npm run typecheck

# Run the frontend's unit tests; accepts vitest arguments.
[positional-arguments]
web-test *args:
    npm run test:unit -- "$@"

# Fail if web/dist differs from a fresh build of web/src. Never writes web/dist.
dist-verify:
    npm run verify:dist

# Validate the ticket store and detect pending repairs without writing.
tickets-check:
    git ticket check --fix --dry-run --strict

# The full gate. CI runs the same steps.
ci: vet fmt-check test cross web-typecheck web-test dist-verify tickets-check

# Internal work merges on Forgejo (origin) and outside pull requests merge on
# GitHub (github), so either main can move ahead of the other. This
# fast-forwards whichever is behind. When both have moved it stops and names
# the heads: merge them on a branch, land it through a Forgejo pull request,
# and run this again. It never force-pushes, and without --yes it prints the
# plan and pushes nothing. docs/pr-reviews.md.
# Fast-forward the stale main between Forgejo and GitHub. Needs --yes.
[positional-arguments]
sync-github *flags:
    #!/usr/bin/env bash
    set -euo pipefail
    yes=false
    for f in "$@"; do
        case "$f" in
            --yes) yes=true ;;
            *) echo "sync-github: unknown flag $f" >&2; exit 2 ;;
        esac
    done
    git fetch --quiet origin main
    # ls-remote exits 2 only when the remote answered and has no main, so an
    # unreachable remote or a refused key is not reported as a missing branch.
    found=0
    git ls-remote --exit-code --heads github main >/dev/null || found=$?
    if [ "$found" = 2 ]; then
        echo "github/main does not exist yet; push it with: git push github origin/main:refs/heads/main" >&2
        exit 1
    elif [ "$found" != 0 ]; then
        echo "sync-github: cannot read the github remote (git ls-remote exited $found)" >&2
        exit 1
    fi
    git fetch --quiet github main
    forgejo=$(git rev-parse origin/main)
    github=$(git rev-parse github/main)
    if [ "$forgejo" = "$github" ]; then
        echo "in sync at ${forgejo:0:12}"
        exit 0
    fi
    if git merge-base --is-ancestor "$github" "$forgejo"; then
        from=origin; to=github; head=$forgejo
    elif git merge-base --is-ancestor "$forgejo" "$github"; then
        from=github; to=origin; head=$github
    else
        echo "diverged: origin/main ${forgejo:0:12}, github/main ${github:0:12}" >&2
        echo "merge them on a branch and land it through a Forgejo PR" >&2
        exit 1
    fi
    echo "fast-forward $to/main to $from/main ${head:0:12}:"
    git log --oneline "$to/main..$head"
    if [ "$yes" != true ]; then
        echo "dry run; pass --yes to push"
        exit 0
    fi
    git push "$to" "$head:refs/heads/main"
