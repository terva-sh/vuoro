package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/terva-sh/vuoro/forge"
)

type sent struct {
	method, path string
	query        string
	vars         map[string]any
	body         map[string]any
}

// adminFake records every request and answers each mutation with errors, a
// mutation-level error list, or success.
func adminFake(t *testing.T, fail string) (*Client, forge.Ref, *[]sent) {
	t.Helper()
	var log []sent
	srv := projectFake(t, func(w http.ResponseWriter, r *http.Request, self string) bool {
		b, _ := io.ReadAll(r.Body)
		s := sent{method: r.Method, path: r.URL.EscapedPath()}
		if r.URL.Path == "/api/graphql" {
			var req struct {
				Query     string         `json:"query"`
				Variables map[string]any `json:"variables"`
			}
			_ = json.Unmarshal(b, &req)
			s.query, s.vars = req.Query, req.Variables
			name := strings.Fields(strings.SplitN(req.Query, "{", 3)[1])[0]
			name = strings.TrimSuffix(name, "(input:")
			name, _, _ = strings.Cut(name, "(")
			log = append(log, s)
			if name == fail {
				fmt.Fprintf(w, `{"data":{%q:{"errors":["nonesuch is not a visible user"]}}}`, name)
				return true
			}
			fmt.Fprintf(w, `{"data":{%q:{"errors":[]}}}`, name)
			return true
		}
		_ = json.Unmarshal(b, &s.body)
		log = append(log, s)
		fmt.Fprint(w, `{}`)
		return true
	})
	c, ref := client(t, srv, secret)
	return c, ref, &log
}

func TestAdministerSendsAdditionsAndRemovalsNeverAReplacement(t *testing.T) {
	c, ref, log := adminFake(t, "")
	draft := false
	err := c.Administer(context.Background(), ref, forge.Changes{
		AddReviewers: []string{"bob"}, RemoveReviewers: []string{"cat"}, AddAssignees: []string{"ann"}, RemoveAssignees: []string{"dan"},
		AddLabels: []string{"backend"}, RemoveLabels: []string{"needs review"}, Draft: &draft,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(*log) != 6 {
		t.Fatalf("%d requests", len(*log))
	}
	for _, s := range *log {
		if s.path != "/api/graphql" {
			continue
		}
		if strings.Contains(s.query, "mergeRequestSetDraft") {
			if s.vars["draft"] != false {
				t.Fatalf("draft %v", s.vars)
			}
			continue
		}
		// A mutation that omits the mode would replace the whole set.
		mode, _ := s.vars["mode"].(string)
		if mode != "APPEND" && mode != "REMOVE" || !strings.Contains(s.query, "operationMode: $mode") {
			t.Fatalf("a mutation went without an explicit mode: %v %s", s.vars, s.query)
		}
	}
	labels := (*log)[4]
	if labels.method != "PUT" || labels.path != "/api/v4/projects/gitlab-org%2Fcli/merge_requests/3991" ||
		labels.body["add_labels"] != "backend" || labels.body["remove_labels"] != "needs review" || labels.body["labels"] != nil {
		t.Fatalf("labels %+v", labels)
	}
}

func TestAMutationErrorFailsTheWriteAndNamesWhatLanded(t *testing.T) {
	c, ref, _ := adminFake(t, "mergeRequestSetAssignees")
	err := c.Administer(context.Background(), ref, forge.Changes{AddReviewers: []string{"bob"}, AddAssignees: []string{"nonesuch"}, AddLabels: []string{"x"}})
	if err == nil || !strings.Contains(err.Error(), "not a visible user") || !strings.Contains(err.Error(), "already added reviewers") {
		t.Fatalf("error %v", err)
	}
}

func TestAnonymousAndCommaLabelsAreRefused(t *testing.T) {
	c, ref, log := adminFake(t, "")
	if err := c.Administer(context.Background(), ref, forge.Changes{AddLabels: []string{"a,b"}}); err == nil || len(*log) != 0 {
		t.Fatal("a label with a comma was sent, and would have become two")
	}
	anon, _ := client(t, projectFake(t, func(http.ResponseWriter, *http.Request, string) bool { return false }), "")
	if err := anon.Administer(context.Background(), ref, forge.Changes{AddReviewers: []string{"bob"}}); err == nil {
		t.Fatal("an anonymous client administered")
	}
	if anon.Capabilities().Reviewers || !c.Capabilities().Reviewers {
		t.Fatal("capabilities do not follow the token")
	}
}

func TestMembersAndLabels(t *testing.T) {
	srv := projectFake(t, func(w http.ResponseWriter, r *http.Request, self string) bool {
		switch r.URL.EscapedPath() {
		case "/api/v4/projects/team%2Frepo/members/all":
			if r.URL.Query().Get("query") != "bo" {
				t.Errorf("members query %v", r.URL.Query())
			}
			if r.URL.Query().Get("page") == "1" {
				rows := []string{`{"username":"bob","name":"Bob","state":"active"}`, `{"username":"bo-gone","name":"Gone","state":"blocked"}`}
				for i := len(rows); i < 100; i++ {
					rows = append(rows, `{"username":"x","state":"blocked"}`)
				}
				w.Header().Set("X-Page", "1")
				w.Header().Set("X-Next-Page", "2")
				fmt.Fprint(w, "["+strings.Join(rows, ",")+"]")
			} else {
				w.Header().Set("X-Page", "2")
				fmt.Fprint(w, `[{"username":"bonnie","name":"Bonnie","state":"active"}]`)
			}
		case "/api/v4/projects/team%2Frepo/labels":
			if r.URL.Query().Get("include_ancestor_groups") != "true" {
				t.Error("group labels left out")
			}
			fmt.Fprint(w, `[{"name":"backend"},{"name":"group::infra"}]`)
		default:
			return false
		}
		return true
	})
	c, _ := client(t, srv, secret)
	members, err := c.Members(context.Background(), "team/repo", "bo")
	if err != nil || len(members) != 2 || members[1].Username != "bonnie" {
		t.Fatalf("members %+v %v", members, err)
	}
	labels, err := c.Labels(context.Background(), "team/repo")
	if err != nil || len(labels) != 2 {
		t.Fatalf("labels %v %v", labels, err)
	}
}
