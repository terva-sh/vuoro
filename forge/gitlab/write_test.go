package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
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

// recorder is a GitLab that accepts every write and records it. It answers
// the reads a write makes, as the signed-in person me, with the recorded
// merge request, in which phikai has requested changes and rkumar555 has not
// reviewed.
func recorder(t *testing.T, me ...string) (*Client, forge.Ref, *[]call) {
	t.Helper()
	who := "phikai"
	if len(me) > 0 {
		who = me[0]
	}
	gql, err := os.ReadFile("testdata/mr3991.json")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var calls []call
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+secret {
			http.Error(w, `{"message":"401 Unauthorized"}`, 401)
			return
		}
		switch r.URL.Path {
		case "/api/v4/user":
			fmt.Fprintf(w, `{"username": %q}`, who)
			return
		case "/api/graphql":
			w.Write(gql)
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
	must(c.Submit(ctx, ref, forge.VerdictCommented))
	must(c.Submit(ctx, ref, forge.VerdictChangesRequested))
	must(c.Submit(ctx, ref, forge.VerdictApproved))
	if err := c.Submit(ctx, ref, "lgtm"); err == nil {
		t.Fatal("an unknown verdict was submitted")
	}

	mr := "/api/v4/projects/gitlab-org%2Fcli/merge_requests/3991"
	want := []struct{ method, path string }{
		{"POST", mr + "/draft_notes"}, {"POST", mr + "/draft_notes"}, {"POST", mr + "/draft_notes"},
		{"PUT", mr + "/draft_notes/41"}, {"DELETE", mr + "/draft_notes/41"},
		{"PUT", mr + "/discussions/abc123"}, {"POST", mr + "/approve"}, {"POST", mr + "/unapprove"},
		{"POST", mr + "/draft_notes/bulk_publish"},
		{"POST", mr + "/draft_notes/bulk_publish"},
		{"POST", mr + "/draft_notes/bulk_publish"}, {"POST", mr + "/approve"},
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
	if (*calls)[8].Body != nil || (*calls)[10].Body != nil {
		t.Fatalf("a commented or approving batch carried a reviewer state: %v %v", (*calls)[8].Body, (*calls)[10].Body)
	}
	if (*calls)[9].Body["reviewer_state"] != "requested_changes" {
		t.Fatalf("request changes %v", (*calls)[9].Body)
	}
}

// GitLab answers a bulk publish with success whether or not it recorded
// reviewer_state, so asking for changes reads the merge request back, and a
// request it did not record is an error rather than a lost verdict.
func TestRequestingChangesThatGitLabDidNotRecordIsAnError(t *testing.T) {
	c, ref, calls := recorder(t, "rkumar555")
	err := c.Submit(context.Background(), ref, forge.VerdictChangesRequested)
	if !errors.Is(err, errChangesNotRecorded) || !strings.Contains(err.Error(), "comments were published") {
		t.Fatalf("err %v", err)
	}
	if len(*calls) != 1 || (*calls)[0].Body["reviewer_state"] != "requested_changes" {
		t.Fatalf("writes %+v", *calls)
	}
}

// A read-back that fails after the comments are published still says they
// were published, rather than reading as a failed submit.
func TestAnUnconfirmedRequestForChangesSaysTheCommentsWerePublished(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4/user" {
			http.Error(w, `{"message":"502 Bad Gateway"}`, 502)
			return
		}
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	c, ref := client(t, srv, secret)
	err := c.Submit(context.Background(), ref, forge.VerdictChangesRequested)
	if err == nil || !strings.Contains(err.Error(), "comments were published") || !strings.Contains(err.Error(), "could not confirm") {
		t.Fatalf("err %v", err)
	}
	var api *APIError
	if !errors.As(err, &api) {
		t.Fatalf("the read error is not kept: %v", err)
	}
}

func TestAnAnonymousClientWritesNothing(t *testing.T) {
	c, ref, calls := recorder(t)
	c.token = config.Secret{}
	if err := c.Submit(context.Background(), ref, forge.VerdictCommented); !errors.Is(err, errNoToken) {
		t.Fatalf("err %v", err)
	}
	if len(*calls) != 0 {
		t.Fatal("an anonymous write reached the server")
	}
	if err := c.UpdateDraft(context.Background(), ref, "../../approve", "x"); err == nil {
		t.Fatal("a draft id that is a path was accepted")
	}
}

func TestARefusedApprovalSaysWhatA401UsuallyMeans(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"401 Unauthorized"}`, 401)
	}))
	defer srv.Close()
	c, ref := client(t, srv, secret)
	err := c.Approve(context.Background(), ref, true)
	if err == nil || !strings.Contains(err.Error(), "your own") {
		t.Fatalf("err %v", err)
	}
	if strings.Count(err.Error(), "401 Unauthorized") != 1 {
		t.Fatalf("the status is repeated: %v", err)
	}
}
