package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/terva-sh/vuoro/agent"
	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/forge/gitlab"
	"github.com/terva-sh/vuoro/review"
	"github.com/terva-sh/vuoro/review/drafts"
)

// fakeGitLab serves merge request !3991 and records every write.
type fakeGitLab struct {
	mu     sync.Mutex
	writes []string
	bodies []map[string]any
	// slow holds each write back this long, as a busy forge does.
	slow time.Duration
	// fork puts the merge request's source branch in another project.
	fork bool
}

func (g *fakeGitLab) handler(t *testing.T) http.Handler {
	gql, err := os.ReadFile("../forge/gitlab/testdata/mr3991.json")
	if err != nil {
		t.Fatal(err)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer glpat-test" {
			http.Error(w, `{"message":"401 Unauthorized"}`, 401)
			return
		}
		switch {
		case r.URL.Path == "/api/graphql":
			w.Write(gql)
		case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/diffs"):
			w.Write([]byte(`[{"old_path":"label_list.go","new_path":"label_list.go","diff":"@@ -1,3 +1,3 @@\n package list\n-// old\n+// new\n tail\n"}]`))
		case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/draft_notes"):
			w.Write([]byte(`[]`))
		case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/merge_requests/3991"):
			g.mu.Lock()
			source := 1
			if g.fork {
				source = 2
			}
			g.mu.Unlock()
			fmt.Fprintf(w, `{"source_project_id": %d, "target_project_id": 1, "source_branch": "fix/label-list-total", "sha": "abc"}`, source)
		default:
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			g.mu.Lock()
			g.writes = append(g.writes, r.Method+" "+r.URL.EscapedPath())
			g.bodies = append(g.bodies, body)
			slow := g.slow
			g.mu.Unlock()
			time.Sleep(slow)
			w.Write([]byte(`{"id": 77, "note": "x"}`))
		}
	})
}

func (g *fakeGitLab) seen() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]string(nil), g.writes...)
}

// agentDriver is an agent that drafts two findings through the drafts MCP
// server, and tries nothing else, because it can do nothing else.
type agentDriver struct{}

func (agentDriver) Name() string                               { return "Fake" }
func (agentDriver) Probe(context.Context) (agent.Probe, error) { return agent.Probe{}, nil }
func (agentDriver) Start(_ context.Context, spec agent.Spec) (agent.Session, error) {
	return &agentSession{spec: spec, events: make(chan agent.Event, 16)}, nil
}

type agentSession struct {
	spec   agent.Spec
	events chan agent.Event
	once   sync.Once
}

func (s *agentSession) Ref() string                { return "ref" }
func (s *agentSession) Events() <-chan agent.Event { return s.events }
func (s *agentSession) Cancel() error              { return s.Close() }
func (s *agentSession) Close() error               { s.once.Do(func() { close(s.events) }); return nil }
func (s *agentSession) Send(context.Context, string) error {
	go func() {
		if s.spec.Editor != nil { // an editor commits nothing here, and drafts nothing
			s.events <- agent.Event{Kind: agent.TurnDone}
			return
		}
		for _, p := range []map[string]any{
			{"severity": "medium", "title": "Comment is wrong", "description": "It says new.",
				"anchor": map[string]any{"path": "label_list.go", "start_line": 2}},
			{"severity": "low", "title": "General point", "description": "About the whole change."},
		} {
			b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call",
				"params": map[string]any{"name": "propose_draft", "arguments": p}})
			req, _ := http.NewRequest("POST", s.spec.Drafts.Config["url"].(string), bytes.NewReader(b))
			for k, v := range s.spec.Drafts.Config["headers"].(map[string]string) {
				req.Header.Set(k, v)
			}
			if res, err := http.DefaultClient.Do(req); err == nil {
				res.Body.Close()
			}
		}
		s.events <- agent.Event{Kind: agent.TurnDone}
	}()
	return nil
}

func localRemote(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) string {
		out, err := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(dir, "label_list.go"), []byte("package list\n// new\ntail\n"), 0o644)
	run("add", ".")
	run("commit", "-q", "-m", "c")
	return dir, run("rev-parse", "HEAD")
}

func call(t *testing.T, h http.Handler, path string, body any) (int, map[string]any) {
	t.Helper()
	b, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", path, bytes.NewReader(b)))
	var out map[string]any
	json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func TestAnAgentDraftsAndOnlyThePersonPosts(t *testing.T) {
	g := &fakeGitLab{}
	forgeSrv := httptest.NewServer(g.handler(t))
	defer forgeSrv.Close()
	host := strings.TrimPrefix(forgeSrv.URL, "http://")
	t.Setenv("VUORO_TEST_TOKEN", "glpat-test")

	mcp := drafts.NewServer()
	mcpSrv := httptest.NewServer(mcp)
	defer mcpSrv.Close()
	stateDir := t.TempDir()
	a := &App{
		Config: &config.Config{Forges: []config.Forge{{Name: "fake", Kind: "gitlab", URL: "https://" + host, Username: "me", TokenEnv: "VUORO_TEST_TOKEN"}}},
		Runs: &review.Manager{StateDir: stateDir, Repos: &review.Repos{Dir: filepath.Join(stateDir, "repos")},
			Drafts: mcp, MCPURL: mcpSrv.URL + "/mcp/"},
		Tools: map[string]agent.Driver{"fake": agentDriver{}},
		Dial: func(string, string, config.Secret) (forge.Provider, error) {
			tok, _ := (&config.Forge{TokenEnv: "VUORO_TEST_TOKEN"}).Token(context.Background())
			return gitlab.New(forgeSrv.URL, tok)
		},
	}
	a.probeTools()
	h := a.Handler()
	mrURL := (&url.URL{Scheme: "https", Host: host, Path: "/gitlab-org/cli/-/merge_requests/3991"}).String()

	// A review runs, and its agent drafts.
	ch, err := a.open(context.Background(), mrURL)
	if err != nil {
		t.Fatal(err)
	}
	remote, sha := localRemote(t)
	run, err := a.Runs.Start(ch, review.Source{Remote: remote, Key: "fake/cli", Head: sha,
		Refspecs: []string{"+refs/heads/main:refs/vuoro/head"}}, []review.Reviewer{{Profile: "code"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for run.View().Status != review.Idle && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	v := run.View()
	if len(v.Drafts) != 2 {
		t.Fatalf("drafts %+v, status %s %s", v.Drafts, v.Status, v.Error)
	}
	if w := g.seen(); len(w) != 0 {
		t.Fatalf("the agent's run wrote to the forge: %q", w)
	}

	// On the person's own change, a review posts nothing (D42).
	a.Config.Forges[0].Username = "unstable-code"
	if code, out := call(t, h, fmt.Sprintf("/api/runs/%s/drafts/d1/keep", run.ID), map[string]any{"url": mrURL}); code == 200 || !strings.Contains(fmt.Sprint(out["error"]), "your own change") {
		t.Fatalf("a finding was kept on the person's own change: %d %v", code, out)
	}
	a.Config.Forges[0].Username = "me"
	if len(g.seen()) != 0 {
		t.Fatalf("a refused keep wrote %q", g.seen())
	}

	// The person keeps one, twice in quick succession, and it becomes one
	// pending draft note of theirs.
	g.mu.Lock()
	g.slow = 300 * time.Millisecond
	g.mu.Unlock()
	codes := make(chan int, 2)
	for range 2 {
		go func() {
			code, _ := call(t, h, fmt.Sprintf("/api/runs/%s/drafts/d1/keep", run.ID), map[string]any{"url": mrURL})
			codes <- code
		}()
		time.Sleep(50 * time.Millisecond)
	}
	if a, b := <-codes, <-codes; a+b != 200+400 {
		t.Fatalf("two keeps answered %d and %d, want one 200 and one 400", a, b)
	}
	g.mu.Lock()
	g.slow = 0
	g.mu.Unlock()
	w := g.seen()
	if len(w) != 1 || !strings.HasSuffix(w[0], "/merge_requests/3991/draft_notes") || !strings.HasPrefix(w[0], "POST ") {
		t.Fatalf("keep wrote %q", w)
	}
	pos := g.bodies[0]["position"].(map[string]any)
	if pos["new_line"] != 2.0 || pos["old_line"] != nil || pos["head_sha"] != ch.Diff.Head {
		t.Fatalf("position %v", pos)
	}
	if !strings.HasPrefix(g.bodies[0]["note"].(string), "**Comment is wrong**") {
		t.Fatalf("note %v", g.bodies[0]["note"])
	}
	if d := run.View().Drafts[0]; d.Decision != drafts.Kept || d.NoteID != "77" {
		t.Fatalf("draft %+v", d)
	}
	// The agent can no longer change what the person kept.
	if code, _ := call(t, h, fmt.Sprintf("/api/runs/%s/drafts/d1/keep", run.ID), map[string]any{"url": mrURL}); code == 200 {
		t.Fatal("a draft was kept twice")
	}

	// The person drops the other, and nothing is written.
	if code, out := call(t, h, fmt.Sprintf("/api/runs/%s/drafts/d2/drop", run.ID), map[string]any{"url": mrURL}); code != 200 {
		t.Fatalf("drop: %d %v", code, out)
	}
	if len(g.seen()) != 1 {
		t.Fatalf("drop wrote %q", g.seen())
	}

	// Asking for an editor writes nothing to the forge. The recorded change's
	// head is not in this test's clone, so the editor is refused there,
	// after the head and identity checks have passed; review's own test
	// carries an editor through.
	gitconfig := filepath.Join(t.TempDir(), "gitconfig")
	os.WriteFile(gitconfig, []byte("[user]\n\tname = Pat\n\temail = pat@example.com\n"), 0o644)
	t.Setenv("GIT_CONFIG_GLOBAL", gitconfig)
	if code, out := call(t, h, fmt.Sprintf("/api/runs/%s/editor", run.ID), map[string]any{"url": mrURL}); code != 400 || !strings.Contains(fmt.Sprint(out["error"]), "no longer holds") {
		t.Fatalf("editor: %d %v", code, out)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", fmt.Sprintf("/api/runs/%s/editor", run.ID), nil))
	if rec.Code == 200 || run.View().Editor != nil {
		t.Fatalf("a failed editor left %d %s", rec.Code, rec.Body)
	}
	// Nothing pushes or opens a merge request without the person's
	// confirmation, and a fork is refused before anything is pushed.
	pushPath := fmt.Sprintf("/api/runs/%s/editor/push", run.ID)
	if code, out := call(t, h, pushPath, map[string]any{"url": mrURL, "title": "t", "sha": "abc"}); code != 400 || !strings.Contains(fmt.Sprint(out["error"]), "confirmation") {
		t.Fatalf("an unconfirmed push: %d %v", code, out)
	}
	if code, out := call(t, h, pushPath, map[string]any{"url": mrURL, "confirm": "push", "title": "t", "sha": "abc"}); code != 400 || !strings.Contains(fmt.Sprint(out["error"]), "no editor") {
		t.Fatalf("a push with no editor: %d %v", code, out)
	}
	g.mu.Lock()
	g.fork = true
	g.mu.Unlock()
	if code, out := call(t, h, fmt.Sprintf("/api/runs/%s/editor/stack", run.ID), map[string]any{"url": mrURL}); code != 400 || !strings.Contains(fmt.Sprint(out["error"]), "fork") {
		t.Fatalf("a fork's preview: %d %v", code, out)
	}
	if len(g.seen()) != 1 {
		t.Fatalf("the editor wrote %q", g.seen())
	}

	// Nothing is published until the person submits, and then all at once.
	if code, out := call(t, h, "/api/changes/submit", map[string]any{"url": mrURL}); code != 200 {
		t.Fatalf("submit: %d %v", code, out)
	}
	if w := g.seen(); len(w) != 2 || !strings.HasSuffix(w[1], "/draft_notes/bulk_publish") {
		t.Fatalf("submit wrote %q", w)
	}
	a.Runs.Close(context.Background(), run.ID)
}
