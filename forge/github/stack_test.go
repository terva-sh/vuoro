package github

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
)

// stacking is a GitHub that answers the stack query with pr, and creates pull
// requests, refusing a draft when noDrafts is set.
func stacking(t *testing.T, pr map[string]any, noDrafts bool) (*Client, forge.Ref, *[]map[string]any) {
	t.Helper()
	var created []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var req map[string]any
		json.Unmarshal(b, &req)
		switch {
		case r.URL.Path == "/api/graphql":
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"repository": map[string]any{"pullRequest": pr}}})
		case r.Method == "POST" && r.URL.Path == "/api/v3/repos/owner/repo/pulls":
			created = append(created, req)
			if noDrafts && req["draft"] == true {
				w.WriteHeader(422)
				w.Write([]byte(`{"message":"Validation Failed","errors":[{"resource":"PullRequest","code":"custom","message":"Draft pull requests are not supported in this repository."}]}`))
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"html_url": "https://github.com/owner/repo/pull/9", "draft": req["draft"]})
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)
	c, err := New(srv.URL, config.SecretForTest("sentinel-token"))
	if err != nil {
		t.Fatal(err)
	}
	return c, forge.Ref{Host: strings.TrimPrefix(srv.URL, "http://"), Project: "owner/repo", Number: 1}, &created
}

func TestStackTargetIsTheHeadBranchAndRefusesAFork(t *testing.T) {
	c, ref, _ := stacking(t, map[string]any{"headRefName": "feature", "headRefOid": head, "isCrossRepository": false, "headRepository": map[string]any{"nameWithOwner": "owner/repo"}}, false)
	got, err := c.StackTarget(context.Background(), ref)
	if err != nil || got != (forge.StackTarget{Branch: "feature", Head: head}) {
		t.Fatalf("%+v %v", got, err)
	}
	for name, pr := range map[string]map[string]any{
		"a fork":         {"headRefName": "feature", "headRefOid": head, "isCrossRepository": true, "headRepository": map[string]any{"nameWithOwner": "someone/repo"}},
		"a deleted fork": {"headRefName": "feature", "headRefOid": head, "isCrossRepository": true, "headRepository": nil},
	} {
		c, ref, _ := stacking(t, pr, false)
		if _, err := c.StackTarget(context.Background(), ref); !errors.Is(err, forge.ErrFork) {
			t.Errorf("%s: %v", name, err)
		}
	}
	c, ref, _ = stacking(t, map[string]any{"headRefName": "-x", "headRefOid": head, "headRepository": map[string]any{"nameWithOwner": "owner/repo"}}, false)
	if _, err := c.StackTarget(context.Background(), ref); err == nil {
		t.Fatal("an unsafe branch name was accepted")
	}
}

func TestOpenStackedOpensADraft(t *testing.T) {
	c, ref, created := stacking(t, nil, false)
	url, err := c.OpenStacked(context.Background(), ref, forge.NewStacked{Branch: "vuoro/fix", Target: "feature", Title: "Fix review findings on #1", Description: "d"})
	if err != nil || url != "https://github.com/owner/repo/pull/9" {
		t.Fatalf("%s %v", url, err)
	}
	if req := (*created)[0]; req["draft"] != true || req["head"] != "vuoro/fix" || req["base"] != "feature" || req["title"] != "Fix review findings on #1" {
		t.Fatalf("request %v", req)
	}
}

// A repository whose plan has no drafts gets an ordinary pull request, which
// is returned with ErrNotDraft for the person to mark.
func TestOpenStackedWithoutDraftsSaysSo(t *testing.T) {
	c, ref, created := stacking(t, nil, true)
	url, err := c.OpenStacked(context.Background(), ref, forge.NewStacked{Branch: "vuoro/fix", Target: "feature", Title: "t"})
	if !errors.Is(err, forge.ErrNotDraft) || url == "" {
		t.Fatalf("%s %v", url, err)
	}
	if len(*created) != 2 || (*created)[1]["draft"] != false {
		t.Fatalf("requests %v", *created)
	}
}
