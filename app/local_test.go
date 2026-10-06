package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/terva-sh/vuoro/review"
)

// checkout is a repository at dir on main, with a helper to run git in it.
func checkout(t *testing.T, dir string) func(...string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "-b", "main")
	git("commit", "-q", "--allow-empty", "-m", "base")
	return git
}

func getLocalView(t *testing.T, h http.Handler) localView {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/local", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/local: %d %s", w.Code, w.Body)
	}
	var v localView
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func postLocal(t *testing.T, h http.Handler, path string, body any) (int, localView, string) {
	t.Helper()
	code, out := call(t, h, path, body)
	b, _ := json.Marshal(out)
	var v localView
	json.Unmarshal(b, &v)
	msg, _ := out["error"].(string)
	return code, v, msg
}

func branchNamed(v localView, project, name string) *localBranch {
	for _, p := range v.Projects {
		if p.Name != project {
			continue
		}
		for i := range p.Branches {
			if p.Branches[i].Name == name {
				return &p.Branches[i]
			}
		}
	}
	return nil
}

func TestAnAddedProjectListsItsBranchesAndWhatEachNeeds(t *testing.T) {
	work := t.TempDir()
	dir := filepath.Join(work, "proj")
	git := checkout(t, dir)
	git("switch", "-q", "-c", "feature")
	git("commit", "-q", "--allow-empty", "-m", "one")
	git("switch", "-q", "main")
	a := &App{StateDir: t.TempDir()}
	h := a.Handler()

	for _, bad := range []string{"relative", filepath.Join(work, "missing"), work} {
		if code, _, _ := postLocal(t, h, "/api/local/projects", map[string]any{"path": bad}); code == http.StatusOK {
			t.Errorf("adding %q was accepted", bad)
		}
	}
	code, v, msg := postLocal(t, h, "/api/local/projects", map[string]any{"path": dir})
	if code != http.StatusOK {
		t.Fatalf("add: %d %s", code, msg)
	}
	if len(v.Projects) != 1 || !v.Projects[0].Added || v.Projects[0].Base != "main" {
		t.Fatalf("projects %+v", v.Projects)
	}
	b := branchNamed(v, "proj", "feature")
	if b == nil || b.Ahead != 1 || b.Run != nil || len(b.Needs) != 1 || b.Needs[0] != "never reviewed" || v.Needs != 1 {
		t.Fatalf("feature: %+v, needs %d", b, v.Needs)
	}
	// The store outlives a restart, in local.json.
	if v := getLocalView(t, (&App{StateDir: a.StateDir}).Handler()); len(v.Projects) != 1 {
		t.Fatalf("after a restart: %+v", v.Projects)
	}
	if code, v, _ = postLocal(t, h, "/api/local/projects/remove", map[string]any{"path": dir}); code != http.StatusOK || len(v.Projects) != 0 {
		t.Fatalf("remove: %d %+v", code, v.Projects)
	}
}

func TestARootIsScannedAndAFoundProjectCanBeHiddenOrGivenABase(t *testing.T) {
	root := t.TempDir()
	checkout(t, filepath.Join(root, "a"))
	gitB := checkout(t, filepath.Join(root, "group", "b"))
	gitB("switch", "-q", "-c", "topic")
	gitB("commit", "-q", "--allow-empty", "-m", "one")
	gitB("branch", "develop", "main")
	h := (&App{StateDir: t.TempDir()}).Handler()

	code, v, msg := postLocal(t, h, "/api/local/roots", map[string]any{"path": root})
	if code != http.StatusOK || len(v.Projects) != 2 || v.Projects[0].Added || v.Needs != 1 {
		t.Fatalf("root: %d %s %+v needs %d", code, msg, v.Projects, v.Needs)
	}
	var key string
	for _, p := range v.Projects {
		if p.Name == "b" {
			key = p.Key
		}
	}
	if code, _, _ := postLocal(t, h, "/api/local/hide", map[string]any{"key": filepath.Join(root, "elsewhere"), "hidden": true}); code == http.StatusOK {
		t.Fatal("hid a project vuoro does not list")
	}
	if code, _, _ := postLocal(t, h, "/api/local/base", map[string]any{"key": key, "base": "no-such-branch"}); code == http.StatusOK {
		t.Fatal("took a base that is not a commit")
	}
	if code, v, msg = postLocal(t, h, "/api/local/base", map[string]any{"key": key, "base": "develop"}); code != http.StatusOK || branchNamed(v, "b", "topic") == nil {
		t.Fatalf("base: %d %s %+v", code, msg, v.Projects)
	}
	for _, p := range v.Projects {
		if p.Key == key && (p.Base != "develop" || !p.BaseChosen) {
			t.Fatalf("chosen base: %+v", p)
		}
	}
	code, v, _ = postLocal(t, h, "/api/local/hide", map[string]any{"key": key, "hidden": true})
	if code != http.StatusOK || len(v.Projects) != 1 || len(v.Hidden) != 1 || v.Needs != 0 {
		t.Fatalf("hide: %d shown %d hidden %d needs %d", code, len(v.Projects), len(v.Hidden), v.Needs)
	}
	if code, v, _ = postLocal(t, h, "/api/local/hide", map[string]any{"key": key, "hidden": false}); code != http.StatusOK || len(v.Projects) != 2 {
		t.Fatalf("unhide: %d %+v", code, v.Projects)
	}
	if code, v, _ = postLocal(t, h, "/api/local/roots/remove", map[string]any{"path": root}); code != http.StatusOK || len(v.Projects) != 0 {
		t.Fatalf("remove root: %d %+v", code, v.Projects)
	}
}

func TestABranchKnowsItsNewestReviewWhatMovedSinceAndItsWaitingFix(t *testing.T) {
	work := t.TempDir()
	dir := filepath.Join(work, "proj")
	git := checkout(t, dir)
	git("switch", "-q", "-c", "feature")
	git("commit", "-q", "--allow-empty", "-m", "one")
	reviewed := git("rev-parse", "HEAD")
	git("branch", "vuoro/fix-r1", reviewed)
	git("switch", "-q", "vuoro/fix-r1")
	git("commit", "-q", "--allow-empty", "-m", "the fix")
	git("switch", "-q", "-c", "quiet", "main")
	git("commit", "-q", "--allow-empty", "-m", "unreviewed")
	git("switch", "-q", "feature")
	git("commit", "-q", "--allow-empty", "-m", "after the review")
	top := git("rev-parse", "--show-toplevel")

	stateDir := t.TempDir()
	rec := map[string]any{
		"id": "r1", "kind": "local", "ref": map[string]any{"host": "local", "project": "proj-1", "number": 0},
		"title": "one", "web_url": "", "head": reviewed, "base": "", "created": time.Now().UTC(), "status": "closed",
		"cost_usd": 0, "sessions": []any{}, "origin": filepath.FromSlash(top), "repo": "", "resume": map[string]any{},
		"drafts": []any{
			map[string]any{"id": "d1", "status": "pending", "severity": "high", "title": "t", "description": "x", "revision": 1, "session": "s1"},
			map[string]any{"id": "d2", "status": "pending", "severity": "low", "title": "t", "description": "x", "revision": 1, "session": "s1", "decision": "dropped"},
		},
		"editor": map[string]any{"session": "e1", "branch": "vuoro/fix-r1", "base": reviewed, "started": time.Now().UTC(), "local": true,
			"pushed": map[string]any{"branch": "vuoro/fix-r1", "sha": "x", "target": "", "at": time.Now().UTC()}},
	}
	b, _ := json.Marshal(rec)
	os.MkdirAll(filepath.Join(stateDir, "runs", "r1"), 0o700)
	os.WriteFile(filepath.Join(stateDir, "runs", "r1", "run.json"), b, 0o600)
	m := &review.Manager{StateDir: stateDir}
	if err := m.Load(); err != nil {
		t.Fatal(err)
	}
	h := (&App{StateDir: stateDir, Runs: m}).Handler()
	_, v, msg := postLocal(t, h, "/api/local/projects", map[string]any{"path": dir})
	feature := branchNamed(v, "proj", "feature")
	if feature == nil || feature.Run == nil {
		t.Fatalf("feature has no review: %+v %s", feature, msg)
	}
	if feature.Run.ID != "r1" || !feature.Run.Moved || feature.Run.Undecided != 1 {
		t.Fatalf("feature's review: %+v", feature.Run)
	}
	if len(feature.Fixes) != 1 || !feature.Fixes[0].Waiting {
		t.Fatalf("feature's fixes: %+v", feature.Fixes)
	}
	want := []string{"moved since its review", "1 finding undecided", "vuoro/fix-r1 waits to be merged"}
	if strings.Join(feature.Needs, "|") != strings.Join(want, "|") {
		t.Fatalf("feature needs %q, want %q", feature.Needs, want)
	}
	if q := branchNamed(v, "proj", "quiet"); q == nil || q.Run != nil {
		t.Fatalf("quiet took a review of another branch: %+v", q)
	}
	if branchNamed(v, "proj", "vuoro/fix-r1") != nil {
		t.Fatal("vuoro's own fix branch is a row")
	}
}
