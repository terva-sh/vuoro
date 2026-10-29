// Package update moves an installed vuoro release to the newest one (D74). It
// follows git-ticket's self-update: it reads the latest release of
// terva-sh/vuoro on GitHub, downloads the archive for this platform, checks it
// against the release's checksums.txt before anything on disk moves, and
// swaps the binary in place. The window and `vuoro self-update` both use it.
package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// API is the GitHub API the releases are read from. The internal forge is
// never asked: a binary in the wild has never heard of it.
const API = "https://api.github.com"

const latestPath = "/repos/terva-sh/vuoro/releases/latest"

// maxDownload bounds an archive or checksums download, so a wrong answer
// cannot fill memory. A release archive is about 10 MB.
const maxDownload = 200 << 20

// ErrNotARelease refuses to update a build that is not a release: a dev
// build, or one past the last tag. Replacing it would destroy that build,
// and there is no release to compare it with.
var ErrNotARelease = errors.New("this vuoro is not a release build, so it is not updated; build it from source again, or install a release")

// Release is the part of a GitHub release this package reads.
type Release struct {
	Tag    string  `json:"tag_name"`
	Assets []Asset `json:"assets"`
}

// Asset is one file a release carries.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

// Client reads releases. Its zero value asks GitHub with a default client.
type Client struct {
	HTTP *http.Client
	API  string // the API's base URL; empty means API
}

// Latest asks for the newest release that is not a prerelease, which is
// what GitHub's latest is.
func (c Client) Latest(ctx context.Context, current string) (*Release, error) {
	api := c.API
	if api == "" {
		api = API
	}
	body, err := c.get(ctx, api+latestPath, current)
	if err != nil {
		return nil, fmt.Errorf("asking for the latest release: %w", err)
	}
	var rel Release
	if err := json.Unmarshal(body, &rel); err != nil {
		return nil, fmt.Errorf("reading the latest release: %w", err)
	}
	if rel.Tag == "" {
		return nil, errors.New("the latest release has no tag")
	}
	return &rel, nil
}

// get is every request the package makes. GitHub's API requires a
// User-Agent, and naming the version makes a request log legible.
func (c Client) get(ctx context.Context, url, current string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "vuoro/"+current)
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxDownload+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxDownload {
		return nil, fmt.Errorf("GET %s: the answer is larger than %d MB", url, maxDownload>>20)
	}
	return b, nil
}

// semver reads v0.2.0. Anything else, a prerelease or a pseudo-version
// included, is not a release this package reasons from.
func semver(v string) (n [3]int, ok bool) {
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	if !strings.HasPrefix(v, "v") || len(parts) != 3 {
		return n, false
	}
	for i, p := range parts {
		x, err := strconv.Atoi(p)
		if err != nil || x < 0 || strconv.Itoa(x) != p {
			return n, false
		}
		n[i] = x
	}
	return n, true
}

// Newer says whether latest is a newer release than current. A current that
// is not a release is ErrNotARelease. A latest at or behind current is not
// newer: offering a downgrade is never an update.
func Newer(current, latest string) (bool, error) {
	c, ok := semver(current)
	if !ok {
		return false, ErrNotARelease
	}
	l, ok := semver(latest)
	if !ok {
		return false, fmt.Errorf("the latest release's tag %q is not a version", latest)
	}
	for i := range c {
		if l[i] != c[i] {
			return l[i] > c[i], nil
		}
	}
	return false, nil
}

// AssetName is the archive a release carries for a platform, as
// .goreleaser.yaml names it.
func AssetName(tag, goos, goarch string) string {
	ext := "tar.gz"
	if goos == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("vuoro_%s_%s_%s.%s", strings.TrimPrefix(tag, "v"), goos, goarch, ext)
}

// Executable is the running binary's own path, with links resolved, which
// is the file an update replaces.
func Executable() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("finding this binary's own path: %w", err)
	}
	if r, err := filepath.EvalSymlinks(p); err == nil {
		p = r
	}
	return p, nil
}

// Apply downloads rel's archive for this platform, checks it against the
// release's checksums.txt, and replaces target with the binary inside. Nothing
// on disk moves until the checksum has matched.
func (c Client) Apply(ctx context.Context, rel *Release, current, target string) error {
	return c.apply(ctx, rel, current, target, runtime.GOOS, runtime.GOARCH)
}

func (c Client) apply(ctx context.Context, rel *Release, current, target, goos, goarch string) error {
	name := AssetName(rel.Tag, goos, goarch)
	var assetURL, sumsURL string
	for _, a := range rel.Assets {
		switch a.Name {
		case name:
			assetURL = a.URL
		case "checksums.txt":
			sumsURL = a.URL
		}
	}
	if assetURL == "" {
		return fmt.Errorf("release %s has no archive for %s/%s (%s)", rel.Tag, goos, goarch, name)
	}
	if sumsURL == "" {
		return fmt.Errorf("release %s has no checksums.txt, so its archive cannot be checked", rel.Tag)
	}
	archive, err := c.get(ctx, assetURL, current)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", name, err)
	}
	sums, err := c.get(ctx, sumsURL, current)
	if err != nil {
		return fmt.Errorf("downloading checksums.txt: %w", err)
	}
	if err := checksum(sums, name, archive); err != nil {
		return err
	}
	bin, err := extract(name, archive)
	if err != nil {
		return err
	}
	return replace(target, bin, goos)
}

// checksum holds the archive to its line in checksums.txt.
func checksum(sums []byte, name string, archive []byte) error {
	want := ""
	for _, line := range strings.Split(string(sums), "\n") {
		if f := strings.Fields(line); len(f) == 2 && f[1] == name {
			want = f[0]
			break
		}
	}
	if want == "" {
		return fmt.Errorf("checksums.txt has no line for %s", name)
	}
	sum := sha256.Sum256(archive)
	if got := hex.EncodeToString(sum[:]); got != want {
		return fmt.Errorf("%s does not match checksums.txt (got %s, want %s), so it was not installed", name, got, want)
	}
	return nil
}

// extract takes the binary from the archive's root, where .goreleaser.yaml
// puts it.
func extract(name string, archive []byte) ([]byte, error) {
	if strings.HasSuffix(name, ".zip") {
		zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
		if err != nil {
			return nil, fmt.Errorf("opening %s: %w", name, err)
		}
		for _, f := range zr.File {
			if f.Name == "vuoro.exe" {
				rc, err := f.Open()
				if err != nil {
					return nil, err
				}
				defer rc.Close()
				return io.ReadAll(io.LimitReader(rc, maxDownload))
			}
		}
		return nil, fmt.Errorf("%s has no vuoro.exe at its root", name)
	}
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", name, err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("%s has no vuoro at its root", name)
		}
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", name, err)
		}
		if h.Typeflag == tar.TypeReg && h.Name == "vuoro" {
			return io.ReadAll(io.LimitReader(tr, maxDownload))
		}
	}
}

// replace swaps the binary: it writes the new one beside target, then renames
// it over target, so a reader sees the old binary or the new one and never
// half of either. Windows cannot replace a running executable, so there the
// old one is renamed aside first, to target.old, which the next update
// removes.
func replace(target string, bin []byte, goos string) error {
	tmp, err := os.CreateTemp(filepath.Dir(target), ".vuoro-new-*")
	if err != nil {
		return fmt.Errorf("writing beside %s: %w", target, err)
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(bin); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, 0o755); err != nil {
		return err
	}
	if goos == "windows" {
		old := target + ".old"
		os.Remove(old)
		if err := os.Rename(target, old); err != nil {
			return fmt.Errorf("moving the running binary aside: %w", err)
		}
	}
	if err := os.Rename(name, target); err != nil {
		return fmt.Errorf("replacing %s: %w", target, err)
	}
	return nil
}
