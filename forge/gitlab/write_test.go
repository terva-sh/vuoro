package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
)

type call struct {
	Method, Path string
	Body         map[string]any
}

// recorder is a GitLab that accepts every write and records it.
func recorder(t *testing.T) (*Client, forge.Ref, *[]call) {
	t.Helper()
	var mu sync.Mutex
	var calls []call
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+secret {
			http.Error(w, `{"message":"401 Unauthorized"}`, 401)
			return
		}
		b, _ := io.ReadAll(r.Body)
		var body map[string]any
		json.Unmarshal(b, &body)
		mu.Lock()
		calls = append(calls, call{r.Method, r.URL.EscapedPath(), body})
		mu.Unlock()
		if strings.HasSuffix(r.URL.Path, "/draft_notes") {
			w.Write([]byte(`{"id": 41, "note": "kept", "discussion_id": null}`))
			return
		}
		w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	c, ref := client(t, srv, secret)
	return c, ref, &calls
}

func TestWritesSendWhatGitLabExpects(t *testing.T) {
	c, ref, calls := recorder(t)
	ctx := context.Background()
	refs := forge.DiffRefs{Base: "b", Start: "s", Head: "h"}
	d, err := c.CreateDraft(ctx, ref, forge.NewDraft{Body: "kept", Anchor: &forge.Anchor{Path: "a.go", NewLine: 12, OldLine: 10, Diff: refs}})
	if err != nil || d.ID != "41" {
		t.Fatalf("%+v %v", d, err)
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err = c.CreateDraft(ctx, ref, forge.NewDraft{Body: "added line", Anchor: &forge.Anchor{Path: "a.go", NewLine: 3, Diff: refs}})
	must(err)
	_, err = c.CreateDraft(ctx, ref, forge.NewDraft{Body: "me too", ThreadID: "abc123"})
	must(err)
	must(c.UpdateDraft(ctx, ref, "41", "edited"))
	must(c.DeleteDraft(ctx, ref, "41"))
	must(c.Resolve(ctx, ref, "abc123", true))
	must(c.Approve(ctx, ref, true))
	must(c.Approve(ctx, ref, false))
	must(c.Submit(ctx, ref))

	mr := "/api/v4/projects/gitlab-org%2Fcli/merge_requests/3991"
	want := []struct{ method, path string }{
		{"POST", mr + "/draft_notes"}, {"POST", mr + "/draft_notes"}, {"POST", mr + "/draft_notes"},
		{"PUT", mr + "/draft_notes/41"}, {"DELETE", mr + "/draft_notes/41"},
		{"PUT", mr + "/discussions/abc123"}, {"POST", mr + "/approve"}, {"POST", mr + "/unapprove"},
		{"POST", mr + "/draft_notes/bulk_publish"},
	}
	if len(*calls) != len(want) {
		t.Fatalf("%d calls: %+v", len(*calls), *calls)
	}
	for i, w := range want {
		if got := (*calls)[i]; got.Method != w.method || got.Path != w.path {
			t.Errorf("call %d: %s %s, want %s %s", i, got.Method, got.Path, w.method, w.path)
		}
	}
	pos := (*calls)[0].Body["position"].(map[string]any)
	if pos["new_line"] != 12.0 || pos["old_line"] != 10.0 || pos["head_sha"] != "h" || pos["old_path"] != "a.go" || pos["position_type"] != "text" {
		t.Fatalf("position %v", pos)
	}
	if pos := (*calls)[1].Body["position"].(map[string]any); pos["old_line"] != nil {
		t.Fatalf("an added line carried an old line: %v", pos)
	}
	if (*calls)[2].Body["in_reply_to_discussion_id"] != "abc123" || (*calls)[2].Body["position"] != nil {
		t.Fatalf("reply %v", (*calls)[2].Body)
	}
	if (*calls)[5].Body["resolved"] != true {
		t.Fatalf("resolve %v", (*calls)[5].Body)
	}
}

func TestAnAnonymousClientWritesNothing(t *testing.T) {
	c, ref, calls := recorder(t)
	c.token = config.Secret{}
	if err := c.Submit(context.Background(), ref); !errors.Is(err, errNoToken) {
		t.Fatalf("err %v", err)
	}
	if len(*calls) != 0 {
		t.Fatal("an anonymous write reached the server")
	}
	if err := c.UpdateDraft(context.Background(), ref, "../../approve", "x"); err == nil {
		t.Fatal("a draft id that is a path was accepted")
	}
}
