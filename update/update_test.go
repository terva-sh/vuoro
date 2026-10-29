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
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOnlyANewerReleaseIsAnUpdate(t *testing.T) {
	for _, c := range []struct {
		current, latest string
		newer           bool
	}{
		{"v0.1.1", "v0.1.2", true},
		{"v0.1.1", "v0.2.0", true},
		{"v0.9.9", "v1.0.0", true},
		{"v0.1.1", "v0.1.1", false},
		{"v0.2.0", "v0.1.9", false}, // never a downgrade
		{"v0.10.0", "v0.9.0", false},
	} {
		if got, err := Newer(c.current, c.latest); err != nil || got != c.newer {
			t.Errorf("%s → %s: %v, %v; want %v", c.current, c.latest, got, err, c.newer)
		}
	}
	for _, current := range []string{"dev", "v0.1.2-0.20261029191500-efcff53abcde", "v0.2.0-rc.1", "0.1.1", ""} {
		if _, err := Newer(current, "v0.2.0"); !errors.Is(err, ErrNotARelease) {
			t.Errorf("%q is updated: %v", current, err)
		}
	}
	if _, err := Newer("v0.1.1", "nightly"); err == nil {
		t.Error("a latest tag that is not a version was compared")
	}
}

func TestAssetNamesMatchTheArchives(t *testing.T) {
	if got := AssetName("v0.2.0", "darwin", "arm64"); got != "vuoro_0.2.0_darwin_arm64.tar.gz" {
		t.Error(got)
	}
	if got := AssetName("v0.2.0", "windows", "amd64"); got != "vuoro_0.2.0_windows_amd64.zip" {
		t.Error(got)
	}
}

func tgz(t *testing.T, name string, body []byte) []byte {
	t.Helper()
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	tw := tar.NewWriter(gz)
	for _, f := range []struct {
		name string
		body []byte
	}{{"LICENSE", []byte("MIT")}, {name, body}} {
		tw.WriteHeader(&tar.Header{Name: f.name, Mode: 0o755, Size: int64(len(f.body)), Typeflag: tar.TypeReg})
		tw.Write(f.body)
	}
	tw.Close()
	gz.Close()
	return b.Bytes()
}

func zipOf(t *testing.T, name string, body []byte) []byte {
	t.Helper()
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	w, _ := zw.Create(name)
	w.Write(body)
	zw.Close()
	return b.Bytes()
}

// github serves a latest release of tag carrying archives, and checksums.txt
// with sums, which default to the archives' own.
func github(t *testing.T, tag string, archives map[string][]byte, sums map[string]string) (Client, *Release) {
	t.Helper()
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	rel := &Release{Tag: tag}
	var lines []string
	for name, body := range archives {
		mux.HandleFunc("/dl/"+name, func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("User-Agent") != "vuoro/v0.1.1" {
				t.Errorf("User-Agent %q", r.Header.Get("User-Agent"))
			}
			w.Write(body)
		})
		rel.Assets = append(rel.Assets, Asset{Name: name, URL: srv.URL + "/dl/" + name})
		sum := sha256.Sum256(body)
		s := hex.EncodeToString(sum[:])
		if o, ok := sums[name]; ok {
			s = o
		}
		lines = append(lines, s+"  "+name)
	}
	mux.HandleFunc("/dl/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, strings.Join(lines, "\n"))
	})
	rel.Assets = append(rel.Assets, Asset{Name: "checksums.txt", URL: srv.URL + "/dl/checksums.txt"})
	mux.HandleFunc(latestPath, func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(rel) })
	return Client{API: srv.URL}, rel
}

func installed(t *testing.T) string {
	t.Helper()
	target := filepath.Join(t.TempDir(), "vuoro")
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	return target
}

func TestAnUpdateReplacesTheBinaryWithTheCheckedOne(t *testing.T) {
	ctx := context.Background()
	c, _ := github(t, "v0.2.0", map[string][]byte{
		"vuoro_0.2.0_darwin_arm64.tar.gz": tgz(t, "vuoro", []byte("new")),
		"vuoro_0.2.0_windows_amd64.zip":   zipOf(t, "vuoro.exe", []byte("new.exe")),
	}, nil)
	rel, err := c.Latest(ctx, "v0.1.1")
	if err != nil || rel.Tag != "v0.2.0" {
		t.Fatalf("latest %+v, %v", rel, err)
	}
	target := installed(t)
	if err := c.apply(ctx, rel, "v0.1.1", target, "darwin", "arm64"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(target); string(b) != "new" {
		t.Fatalf("the binary is %q", b)
	}
	if fi, _ := os.Stat(target); fi.Mode().Perm()&0o100 == 0 {
		t.Errorf("the new binary is not executable: %v", fi.Mode())
	}
	// On Windows the running binary is moved aside, not overwritten.
	target = installed(t)
	if err := c.apply(ctx, rel, "v0.1.1", target, "windows", "amd64"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(target); string(b) != "new.exe" {
		t.Fatalf("the binary is %q", b)
	}
	if b, _ := os.ReadFile(target + ".old"); string(b) != "old" {
		t.Fatalf("the old binary was not kept aside: %q", b)
	}
	if left, _ := filepath.Glob(filepath.Join(filepath.Dir(target), ".vuoro-new-*")); len(left) != 0 {
		t.Errorf("temporary files left: %v", left)
	}
}

func TestNothingIsReplacedUnlessTheArchiveChecksOut(t *testing.T) {
	ctx := context.Background()
	name := "vuoro_0.2.0_linux_amd64.tar.gz"
	for _, c := range []struct {
		why      string
		archives map[string][]byte
		sums     map[string]string
		want     string
	}{
		{"a checksum that does not match", map[string][]byte{name: tgz(t, "vuoro", []byte("evil"))},
			map[string]string{name: strings.Repeat("0", 64)}, "does not match checksums.txt"},
		{"no checksum line", map[string][]byte{name: tgz(t, "vuoro", []byte("new"))},
			map[string]string{name: ""}, "no line for"},
		{"no archive for this platform", map[string][]byte{"vuoro_0.2.0_plan9_amd64.tar.gz": tgz(t, "vuoro", nil)},
			nil, "has no archive for linux/amd64"},
		{"no binary at the archive's root", map[string][]byte{name: tgz(t, "bin/vuoro", []byte("new"))},
			nil, "has no vuoro at its root"},
	} {
		// An empty sum writes a line with no checksum on it.
		client, rel := github(t, "v0.2.0", c.archives, c.sums)
		target := installed(t)
		err := client.apply(ctx, rel, "v0.1.1", target, "linux", "amd64")
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want an error saying %q", c.why, err, c.want)
		}
		if b, _ := os.ReadFile(target); string(b) != "old" {
			t.Errorf("%s: the binary was replaced with %q", c.why, b)
		}
	}
}

func TestAFailedAnswerIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusForbidden)
	}))
	defer srv.Close()
	if _, err := (Client{API: srv.URL}).Latest(context.Background(), "v0.1.1"); err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("%v", err)
	}
}
