package forgejo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
)

func TestAStackedPullRequestGoesOntoTheHeadBranchAsADraft(t *testing.T) {
	head, draft := `{"full_name": "terva-sh/vuoro"}`, true
	var opened []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/v1/repos/terva-sh/vuoro/pulls/2":
			w.Write([]byte(`{"head": {"ref": "fix/total", "sha": "abc123", "repo": ` + head + `}, "base": {"ref": "main", "repo": {"full_name": "terva-sh/vuoro"}}}`))
		case r.Method == "POST" && r.URL.Path == "/api/v1/repos/terva-sh/vuoro/pulls":
			if r.Header.Get("Authorization") != "token synthetic-forgejo-token" {
				http.Error(w, `{"message":"token is required"}`, 401)
				return
			}
			var got map[string]any
			b, _ := io.ReadAll(r.Body)
			json.Unmarshal(b, &got)
			opened = append(opened, got)
			fmt.Fprintf(w, `{"number": 9, "draft": %v, "html_url": "https://forge.example/terva-sh/vuoro/pulls/9"}`, draft)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c, err := New(srv.URL, config.SecretForTest("synthetic-forgejo-token"))
	if err != nil {
		t.Fatal(err)
	}
	ref := forge.Ref{Host: c.base.Host, Project: "terva-sh/vuoro", Number: 2}
	ctx := context.Background()

	got, err := c.StackTarget(ctx, ref)
	if err != nil || got != (forge.StackTarget{Branch: "fix/total", Head: "abc123"}) {
		t.Fatalf("target %+v %v", got, err)
	}
	url, err := c.OpenStacked(ctx, ref, forge.NewStacked{Branch: "vuoro/fix-2-abc", Target: "fix/total", Title: "Fix review findings", Description: "d"})
	if err != nil || url != "https://forge.example/terva-sh/vuoro/pulls/9" {
		t.Fatalf("opened %q %v", url, err)
	}
	if o := opened[0]; o["title"] != "WIP: Fix review findings" || o["head"] != "vuoro/fix-2-abc" || o["base"] != "fix/total" || o["body"] != "d" {
		t.Fatalf("sent %v", o)
	}
	// A title the person already marked is not marked twice.
	for _, title := range []string{"wip: mine", "[WIP] mine"} {
		if _, err := c.OpenStacked(ctx, ref, forge.NewStacked{Branch: "b", Target: "fix/total", Title: title}); err != nil {
			t.Fatal(err)
		}
		if o := opened[len(opened)-1]; o["title"] != title {
			t.Fatalf("sent title %q for %q", o["title"], title)
		}
	}

	for name, h := range map[string]string{"a fork": `{"full_name": "someone/vuoro"}`, "a deleted fork": `null`} {
		head = h
		if _, err := c.StackTarget(ctx, ref); !errors.Is(err, forge.ErrFork) {
			t.Fatalf("%s: %v", name, err)
		}
	}

	// An instance with other draft prefixes opens an ordinary pull request,
	// which is still returned, so it is not opened twice.
	draft = false
	url, err = c.OpenStacked(ctx, ref, forge.NewStacked{Branch: "b", Target: "fix/total", Title: "x"})
	if !errors.Is(err, forge.ErrNotDraft) || url != "https://forge.example/terva-sh/vuoro/pulls/9" {
		t.Fatalf("not a draft: %q %v", url, err)
	}

	bare, _ := New(srv.URL, config.Secret{})
	if _, err := bare.OpenStacked(ctx, ref, forge.NewStacked{Branch: "b", Target: "t", Title: "x"}); !errors.Is(err, errNoToken) {
		t.Fatalf("without a token: %v", err)
	}
}
