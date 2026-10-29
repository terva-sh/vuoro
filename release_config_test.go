package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The release tooling's rules (D74), held as text: vuoro has no YAML parser
// to read the files with, and the rules are about what the files say.

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// GitHub runs release CI/CD and nothing else: per-branch and pull-request CI
// stays on the internal runners (D52, D74).
func TestGitHubRunsTheReleaseOnATagAlone(t *testing.T) {
	files, _ := filepath.Glob(".github/workflows/*")
	if len(files) != 1 || filepath.Base(files[0]) != "release.yml" {
		t.Fatalf("GitHub workflows %v, want release.yml alone", files)
	}
	wf := readFile(t, ".github/workflows/release.yml")
	on := regexp.MustCompile(`(?m)^on:\n((?:[ \t]+.*\n)+)`).FindStringSubmatch(wf)
	if on == nil || on[1] != "  push:\n    tags: ['v*']\n" {
		t.Fatalf("the release workflow's triggers are not a v* tag alone:\n%v", on)
	}
	// Forgejo Actions also runs .github/workflows, so every job checks the
	// server it runs on.
	jobs := regexp.MustCompile(`(?m)^  [a-z][a-z0-9-]*:\n`).FindAllString(wf[strings.Index(wf, "\njobs:\n"):], -1)
	guards := strings.Count(wf, "if: github.server_url == 'https://github.com'")
	if len(jobs) == 0 || guards != len(jobs) {
		t.Fatalf("%d jobs and %d server guards", len(jobs), guards)
	}
}

// goreleaser packages and never publishes; the workflow publishes only what
// scripts/verify-release.py passed.
func TestTheReleasePublishesOnlyWhatItVerified(t *testing.T) {
	cfg := readFile(t, ".goreleaser.yaml")
	if !regexp.MustCompile(`(?m)^release:\n  disable: true\n`).MatchString(cfg) {
		t.Error("goreleaser may publish: release.disable is not true")
	}
	wf := readFile(t, ".github/workflows/release.yml")
	build := strings.Index(wf, "args: release --clean --skip=publish")
	verify := strings.Index(wf, "python3 scripts/verify-release.py --tag")
	publish := strings.Index(wf, "gh release create")
	if build < 0 || verify < build || publish < verify {
		t.Errorf("the workflow does not build, then verify, then publish (at %d, %d, %d)", build, verify, publish)
	}
	if !strings.Contains(wf, "fetch-depth: 0") {
		t.Error("the checkout is shallow, so the binaries would say dev")
	}
}

// What ships: no cgo, the five targets, no link-time version stamp, and the
// documents every archive carries.
func TestTheReleaseShipsFiveArchivesWithTheirNotices(t *testing.T) {
	cfg := readFile(t, ".goreleaser.yaml")
	for _, want := range []string{
		"env: [CGO_ENABLED=0]",
		"goos: [linux, darwin, windows]",
		"goarch: [amd64, arm64]",
		"- goos: windows\n        goarch: arm64",
		`name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"`,
		"name_template: checksums.txt",
	} {
		if !strings.Contains(cfg, want) {
			t.Errorf(".goreleaser.yaml does not say %q", want)
		}
	}
	if strings.Contains(cfg, "-X ") {
		t.Error("the version is stamped at link time; it comes from the build information (D74)")
	}
	files := regexp.MustCompile(`(?m)^    files:\n((?:      - .*\n)+)`).FindStringSubmatch(cfg)
	if files == nil {
		t.Fatal("the archive lists no files")
	}
	var got []string
	for _, l := range strings.Split(strings.TrimSpace(files[1]), "\n") {
		f := strings.TrimPrefix(strings.TrimSpace(l), "- ")
		got = append(got, f)
		if _, err := os.Stat(f); err != nil {
			t.Errorf("the archive carries %s, which the tree does not have", f)
		}
	}
	if strings.Join(got, " ") != "LICENSE THIRD_PARTY_LICENSES README-release.md" {
		t.Errorf("the archive carries %v", got)
	}
}

// The release README tells a macOS user how to allow the unsigned binary,
// since the release is not signed yet (D74).
func TestTheReleaseReadmeSaysHowToAllowTheUnsignedMacBinary(t *testing.T) {
	readme := readFile(t, "README-release.md")
	for _, want := range []string{"Open Anyway", "xattr -d com.apple.quarantine", "checksums.txt"} {
		if !strings.Contains(readme, want) {
			t.Errorf("README-release.md does not say %q", want)
		}
	}
}
