package claude

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/terva-sh/vuoro/agent"
)

// The editor's lockdown (D40): its tools, the one settings source with the
// sandbox, and an environment with no forge credential.
func TestTheEditorIsSandboxedAndHoldsNoForgeCredential(t *testing.T) {
	dir, cache := t.TempDir(), t.TempDir()
	e := agent.Editor{Caches: []string{cache}, Domains: []string{"proxy.golang.org"}, Env: []string{"GOCACHE=" + cache}}

	a := args(agent.Spec{Dir: dir, Editor: &e}, "mcp.json", "sandbox.json", editTools, nil)
	flag := func(name string) string { i := slices.Index(a, name); return a[i+1] }
	if flag("--tools") != "Read,Grep,Glob,Edit,Write,Bash" || flag("--settings") != "sandbox.json" || flag("--setting-sources") != "" ||
		!slices.Contains(a, "--restricted") || flag("--permission-mode") != "dontAsk" {
		t.Fatalf("editor args %q", a)
	}
	if r := args(agent.Spec{Dir: dir}, "mcp.json", "", readTools, nil); slices.Contains(r, "--settings") || strings.Contains(strings.Join(r, " "), "Bash") {
		t.Fatalf("a reviewer got editor flags: %q", r)
	}

	sb, err := sandboxSettings(dir, e, "")
	if err != nil {
		t.Fatal(err)
	}
	s := sb["sandbox"].(map[string]any)
	fs := s["filesystem"].(map[string]any)
	real, _ := filepath.EvalSymlinks(dir)
	realCache, _ := filepath.EvalSymlinks(cache)
	// The worktree is the working directory, which the sandbox lets commands
	// write; beyond it, only the caches.
	if s["enabled"] != true || s["allowUnsandboxedCommands"] != false || s["failIfUnavailable"] != true ||
		!slices.Equal(fs["denyRead"].([]string), []string{"~/"}) || !slices.Equal(fs["allowRead"].([]string), []string{real, realCache}) ||
		!slices.Equal(fs["allowWrite"].([]string), []string{realCache}) {
		t.Fatalf("sandbox %+v", s)
	}
	if d := s["network"].(map[string]any)["allowedDomains"].([]string); !slices.Equal(d, []string{"proxy.golang.org"}) {
		t.Fatalf("domains %q", d)
	}
	creds := s["credentials"].(map[string]any)["envVars"].([]map[string]string)
	if len(creds) != len(loginVars) || creds[0]["mode"] != "deny" {
		t.Fatalf("credentials %+v", creds)
	}

	env := editorEnv([]string{"PATH=/usr/bin", "HOME=/h", "GITLAB_TOKEN=glpat-x", "VUORO_TEST_TOKEN=glpat-y", "SSH_AUTH_SOCK=/s", "ANTHROPIC_API_KEY=k"}, e.Env)
	for _, kv := range env {
		if strings.Contains(kv, "glpat") || strings.HasPrefix(kv, "SSH_AUTH_SOCK=") {
			t.Fatalf("the editor's environment holds %q", kv)
		}
	}
	for _, want := range []string{"PATH=/usr/bin", "HOME=/h", "CLAUDE_CODE_SUBPROCESS_ENV_SCRUB=1", "GOCACHE=" + cache, "ANTHROPIC_API_KEY=k"} {
		if !slices.Contains(env, want) {
			t.Fatalf("env %q lacks %q", env, want)
		}
	}

	if _, err := (&Driver{Path: "/bin/false"}).Start(context.Background(), agent.Spec{Dir: dir, Editor: &e, Drafts: drafts}); err == nil {
		t.Fatal("an editor started with a drafts server")
	}
}

// The sandbox drops GIT_CONFIG_GLOBAL from a command's environment, so the
// editor's variables come back through the file its shell sources first.
func TestTheEditorsVariablesAreExportedForItsShell(t *testing.T) {
	path, err := writeEnvFile(t.TempDir(), []string{"GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=Pat O'Neil"})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	if want := "export GIT_CONFIG_GLOBAL='/dev/null'\nexport GIT_AUTHOR_NAME='Pat O'\\''Neil'\n"; string(b) != want {
		t.Fatalf("env file %q, want %q", b, want)
	}
	if _, err := writeEnvFile(t.TempDir(), []string{"A B=c"}); err == nil {
		t.Fatal("a variable that is not a name was written")
	}
	if env := editorEnv([]string{"CLAUDE_ENV_FILE=/home/me/.env.sh", "PATH=/bin"}, nil); slices.ContainsFunc(env, func(kv string) bool { return strings.HasPrefix(kv, "CLAUDE_ENV_FILE=") }) {
		t.Fatalf("the person's own env file was passed on: %q", env)
	}
}
