package gitlab

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
)

// A second open of a change at the same diff version reuses its patches,
// and a push reads them again.
func TestPatchesAreReadOncePerDiffVersion(t *testing.T) {
	var head atomic.Value
	head.Store("h1")
	var diffReads atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/diffs") {
			diffReads.Add(1)
			w.Write([]byte(`[{"old_path":"a.go","new_path":"a.go","diff":"@@ -1 +1 @@\n-x\n+y\n"}]`))
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"project": map[string]any{"mergeRequest": map[string]any{
			"iid": "5", "title": "t", "state": "opened", "updatedAt": "2026-02-01T00:00:00Z", "createdAt": "2026-01-01T00:00:00Z",
			"author":            map[string]any{"username": "me"},
			"diffRefs":          map[string]any{"baseSha": "b", "headSha": head.Load(), "startSha": "s"},
			"mergeRequestDiffs": map[string]any{"pageInfo": map[string]any{"hasNextPage": false}, "nodes": []any{}},
			"discussions":       map[string]any{"pageInfo": map[string]any{"hasNextPage": false}, "nodes": []any{}},
		}}}})
	}))
	defer srv.Close()
	c, err := New(srv.URL, config.Secret{})
	if err != nil {
		t.Fatal(err)
	}
	ref := forge.Ref{Host: strings.TrimPrefix(srv.URL, "http://"), Project: "g/p", Number: 5}
	open := func() *forge.Change {
		ch, err := c.Open(context.Background(), ref)
		if err != nil {
			t.Fatal(err)
		}
		return ch
	}
	first := open()
	first.Files[0].Patch = "changed by a caller"
	if again := open(); diffReads.Load() != 1 || again.Files[0].Patch != "@@ -1 +1 @@\n-x\n+y\n" {
		t.Fatalf("%d diff reads, patch %q", diffReads.Load(), again.Files[0].Patch)
	}
	head.Store("h2")
	if open(); diffReads.Load() != 2 {
		t.Fatalf("a push did not read the patches again: %d reads", diffReads.Load())
	}
}
