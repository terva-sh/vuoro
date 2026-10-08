package app

import (
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/terva-sh/vuoro/agent"
	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/forge/gitlab"
	"github.com/terva-sh/vuoro/review"
	"github.com/terva-sh/vuoro/review/drafts"
)

// A fresh install reaches a first review through the window alone: with no
// configuration file, the person tests a token source, saves the forge, and
// starts a review of a merge request by its URL with no preset, which runs
// the default code reviewer. The window writes the configuration; nobody
// edits it. Only the forge and the agent are fakes, and git fetches the
// forge's own clone URL, which git's url.insteadOf points at a local copy.
func TestAFreshInstallReachesAFirstReviewWithoutEditingAFile(t *testing.T) {
	// The forge's repository: main, and the merge request's head on top.
	repo := t.TempDir()
	git := func(args ...string) string {
		out, err := exec.Command("git", append([]string{"-C", repo, "-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(repo, "label_list.go"), []byte("package list\n// old\ntail\n"), 0o644)
	git("add", ".")
	git("commit", "-q", "-m", "base")
	base := git("rev-parse", "HEAD")
	os.WriteFile(filepath.Join(repo, "label_list.go"), []byte("package list\n// new\ntail\n"), 0o644)
	git("commit", "-qam", "head")
	head := git("rev-parse", "HEAD")
	git("update-ref", "refs/merge-requests/3991/head", head)
	git("reset", "-q", "--hard", base)

	recorded, err := os.ReadFile("../forge/gitlab/testdata/mr3991.json")
	if err != nil {
		t.Fatal(err)
	}
	gql := strings.NewReplacer("8526e3d7d20ebe488c05425d32b4563b70840df9", head, "59ae78caddfe91cef7808d7fb7c41d04d5b80c77", base).Replace(string(recorded))
	g := &fakeGitLab{gql: []byte(gql)}
	forgeSrv := httptest.NewServer(g.handler(t))
	defer forgeSrv.Close()
	host := strings.TrimPrefix(forgeSrv.URL, "http://")
	t.Setenv("VUORO_TEST_TOKEN", "glpat-test")
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "url."+repo+".insteadOf")
	t.Setenv("GIT_CONFIG_VALUE_0", "https://"+host+"/gitlab-org/cli.git")

	mcp := drafts.NewServer()
	mcpSrv := httptest.NewServer(mcp)
	defer mcpSrv.Close()
	stateDir := t.TempDir()
	path := filepath.Join(t.TempDir(), "vuoro", "config.json")
	a := &App{StateDir: stateDir, Config: &config.Config{}, ConfigPath: path,
		Runs: &review.Manager{StateDir: stateDir, Repos: &review.Repos{Dir: filepath.Join(stateDir, "repos")},
			Drafts: mcp, MCPURL: mcpSrv.URL + "/mcp/"},
		Tools: map[string]agent.Driver{"fake": agentDriver{}},
		Dial:  dialFake(forgeSrv.URL),
	}
	h := a.Handler()
	if v := getSetup(t, h); !v.Editable || v.Base != "" || len(v.Forges) != 0 {
		t.Fatalf("a fresh install: %+v", v)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("a configuration file exists before setup: %v", err)
	}

	form := forgeForm{Name: "work", Kind: "gitlab", URL: "https://" + host, TokenEnv: "VUORO_TEST_TOKEN", Clone: "https"}
	if code, _, body := post(t, h, "/api/setup/test", form); code != 200 || !strings.Contains(body, `"username":"me"`) {
		t.Fatalf("testing the token: %d %s", code, body)
	}
	form.Username = "me"
	if code, v, body := post(t, h, "/api/setup/forges", map[string]any{"base": "", "forge": form}); code != 200 || len(v.Forges) != 1 {
		t.Fatalf("saving the forge: %d %s", code, body)
	}

	mrURL := (&url.URL{Scheme: "https", Host: host, Path: "/gitlab-org/cli/-/merge_requests/3991"}).String()
	code, out := call(t, h, "/api/runs", map[string]any{"url": mrURL})
	if code != 201 {
		t.Fatalf("starting a review: %d %v", code, out)
	}
	run := a.Runs.Get(out["id"].(string))
	deadline := time.Now().Add(20 * time.Second)
	for run.View().Status != review.Idle && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	v := run.View()
	if v.Status != review.Idle || len(v.Drafts) != 2 || v.Head != head {
		t.Fatalf("the first review: status %s %s, head %s, drafts %+v", v.Status, v.Error, v.Head, v.Drafts)
	}
	if len(v.Sessions) == 0 || v.Sessions[0].Profile != "code" {
		t.Fatalf("sessions %+v", v.Sessions)
	}
	if w := g.seen(); len(w) != 0 {
		t.Fatalf("the first review wrote to the forge: %q", w)
	}
	saved, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(saved), `"VUORO_TEST_TOKEN"`) || strings.Contains(string(saved), "glpat-test") {
		t.Fatalf("the saved configuration: %v %s", err, saved)
	}
}

// dialFake dials every forge at the fake GitLab server srv.
func dialFake(srv string) func(string, string, config.Secret) (forge.Provider, error) {
	return func(_, _ string, token config.Secret) (forge.Provider, error) { return gitlab.New(srv, token) }
}
