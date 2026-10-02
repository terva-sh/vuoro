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

# Build vuoro for this platform.
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

# Validate the ticket store and detect pending repairs without writing.
tickets-check:
    git ticket check --fix --dry-run --strict

# The full gate. CI runs the same steps.
ci: vet fmt-check test cross tickets-check
