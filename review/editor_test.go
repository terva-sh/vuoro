package review

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/terva-sh/vuoro/agent"
	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/review/drafts"
)

func TestTheEditorFixesKeptFindingsInARepositoryOfItsOwn(t *testing.T) {
	m, d, ch, src := setup(t, func(call toolCaller, text string) string {
		call("propose_draft", map[string]any{"severity": "high", "title": "Add subtracts", "description": "d",
			"anchor": map[string]any{"path": "calc.go", "start_line": 3}})
		call("propose_draft", map[string]any{"severity": "low", "title": "Name it", "description": "d",
			"anchor": map[string]any{"path": "calc.go", "start_line": 3}})
		return "done"
	})
	var message string
	pass := 0
	d.edit = func(spec agent.Spec, text string) string {
		message = text
		pass++ // each turn changes something, so each has a commit to make
		os.WriteFile(filepath.Join(spec.Dir, "calc.go"), []byte(fmt.Sprintf("package calc\n\nfunc Add(a, b int) int { return a + b }\n\n// pass %d\n", pass)), 0o644)
		for _, args := range [][]string{{"add", "calc.go"}, {"commit", "-q", "-m", "Add adds", "-m", "Vuoro-Finding: d1"}} {
			cmd := exec.Command("git", args...)
			cmd.Dir, cmd.Env = spec.Dir, append([]string{"PATH=" + os.Getenv("PATH"), "HOME=" + spec.Dir}, spec.Editor.Env...)
			if out, err := cmd.CombinedOutput(); err != nil {
				return "git: " + string(out)
			}
		}
		return "d1: fixed"
	}
	r, err := m.Start(ch, src, []Reviewer{{Profile: "code", Tool: "fake", Model: "m1"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	wait(t, r, idle)
	who := Identity{Name: "Pat", Email: "pat@example.com"}
	if _, err := m.StartEditor(context.Background(), r.ID, ch, who, EditorTask{}); err == nil {
		t.Fatal("an editor started with nothing kept")
	}
	m.Decide(r.ID, "d1", drafts.Kept, "41", false)
	moved := *ch
	moved.Diff.Head = "0123456789abcdef"
	if _, err := m.StartEditor(context.Background(), r.ID, &moved, who, EditorTask{}); err == nil {
		t.Fatal("an editor started on a change that moved")
	}
	if _, err := m.StartEditor(context.Background(), r.ID, ch, Identity{}, EditorTask{}); err == nil {
		t.Fatal("an editor started with no git identity")
	}
	ed, err := m.StartEditor(context.Background(), r.ID, ch, who, EditorTask{})
	if err != nil {
		t.Fatal(err)
	}
	if ed.Branch != "vuoro/fix-7-"+r.ID[len(r.ID)-6:] || ed.Base != ch.Diff.Head {
		t.Fatalf("editor %+v", ed)
	}
	if _, err := m.StartEditor(context.Background(), r.ID, ch, who, EditorTask{}); !errors.Is(err, ErrEditorBusy) {
		t.Fatalf("a second editor: %v", err)
	}
	v := wait(t, r, func(v RunView) bool { return idle(v) && len(v.Sessions) == 2 && v.Sessions[1].FirstComplete })
	if e := v.Sessions[1]; e.Role != "editor" || e.Model != "m1" || !v.ReviewComplete {
		t.Fatalf("editor session %+v", e)
	}
	if !strings.Contains(message, "Add subtracts") || strings.Contains(message, "Name it") || !strings.Contains(message, "Finding 1 is d1") {
		t.Fatalf("the editor was sent %q", message)
	}

	d.mu.Lock()
	spec := d.specs[len(d.specs)-1]
	d.mu.Unlock()
	switch {
	case spec.Drafts != nil:
		t.Fatal("the editor was given the drafts server")
	case spec.Dir == r.dir || !strings.HasPrefix(spec.Dir, filepath.Join(m.StateDir, "editor", r.ID)):
		t.Fatalf("the editor works in %s", spec.Dir)
	case !slices.Equal(spec.Editor.Domains, DefaultEditorDomains):
		t.Fatalf("domains %q", spec.Editor.Domains)
	case !slices.Contains(spec.Editor.Env, "GIT_CONFIG_GLOBAL="+os.DevNull) || !slices.Contains(spec.Editor.Env, "GIT_AUTHOR_EMAIL=pat@example.com"):
		t.Fatalf("env %q", spec.Editor.Env)
	case !strings.Contains(spec.SystemPrompt, "Vuoro-Finding"):
		t.Fatal("the editor's prompt names no trailer")
	}
	// The editor's repository borrows nothing from vuoro's clone.
	if _, err := os.Stat(filepath.Join(spec.Dir, ".git", "objects", "info", "alternates")); err == nil {
		t.Fatal("the editor's repository borrows the clone's objects")
	}

	detail, err := m.Editor(context.Background(), r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Commits) != 1 || detail.Commits[0].Subject != "Add adds" || !slices.Equal(detail.Commits[0].Findings, []string{"d1"}) {
		t.Fatalf("commits %+v", detail.Commits)
	}
	if len(detail.Files) != 1 || !strings.Contains(detail.Files[0].Patch, "+func Add(a, b int) int { return a + b }") {
		t.Fatalf("files %+v", detail.Files)
	}

	// The person pushes what they saw, once, and the draft change opens once.
	sha := detail.Commits[0].SHA
	opens := 0
	open := func() (string, error) { opens++; return "https://gitlab.example/g/calc/-/merge_requests/8", nil }
	if _, err := m.PushEditor(context.Background(), r.ID, src.Remote, forge.StackTarget{Branch: "feature", Head: ch.Diff.Head}, "0123456789012345678901234567890123456789", open); err == nil {
		t.Fatal("a commit the person never saw was pushed")
	}
	p, err := m.PushEditor(context.Background(), r.ID, src.Remote, forge.StackTarget{Branch: "feature", Head: ch.Diff.Head}, sha, open)
	if err != nil {
		t.Fatal(err)
	}
	if p.Branch != ed.Branch || p.SHA != sha || p.URL != "https://gitlab.example/g/calc/-/merge_requests/8" || opens != 1 {
		t.Fatalf("pushed %+v, opened %d", p, opens)
	}
	if out, err := exec.Command("git", "-C", src.Remote, "rev-parse", "refs/heads/"+ed.Branch).Output(); err != nil || strings.TrimSpace(string(out)) != sha {
		t.Fatalf("the forge's branch is at %q: %v", out, err)
	}
	if _, err := m.PushEditor(context.Background(), r.ID, src.Remote, forge.StackTarget{Branch: "feature", Head: ch.Diff.Head}, sha, open); err != nil || opens != 1 {
		t.Fatalf("a second push: %v, opened %d", err, opens)
	}
	desc := StackDescription(r.View(), detail, "Pat")
	if !strings.Contains(desc, sha[:8]+" Add adds (high: Add subtracts)") || !strings.Contains(desc, "Pat reviewed them") || strings.Contains(desc, "Name it") {
		t.Fatalf("description %q", desc)
	}

	m2 := &Manager{StateDir: m.StateDir, Repos: m.Repos, Drivers: m.Drivers, Drafts: m.Drafts, MCPURL: m.MCPURL}
	if err := m2.Load(); err != nil {
		t.Fatal(err)
	}
	if back := m2.Get(r.ID); back.View().Editor.Pushed == nil || back.View().Editor == nil || back.View().Sessions[1].Role != "editor" || back.who != who {
		t.Fatalf("reloaded %+v", back.View())
	}

	if err := m.DiscardEditor(context.Background(), r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(m.StateDir, "editor", r.ID)); !os.IsNotExist(err) {
		t.Fatalf("the editor's directory outlived its discard: %v", err)
	}
	if err := m.Say(r.ID, v.Sessions[1].ID, "carry on"); err == nil {
		t.Fatal("a discarded editor was resumed")
	}
	// d1 is fixed by the pushed commit, so it is not handed over again
	// (D50); a newly kept finding is.
	if got := r.View().Drafts[0]; got.FixedIn != sha {
		t.Fatalf("d1 after its push: %+v", got)
	}
	if _, err := m.StartEditor(context.Background(), r.ID, ch, who, EditorTask{}); err == nil {
		t.Fatal("an editor started with only a fixed finding kept")
	}
	m.Decide(r.ID, "d2", drafts.Kept, "42", false)
	if _, err := m.StartEditor(context.Background(), r.ID, ch, who, EditorTask{}); err != nil {
		t.Fatalf("an editor after a discard: %v", err)
	}
	wait(t, r, func(v RunView) bool { return idle(v) && len(v.Sessions) == 3 && v.Sessions[2].FirstComplete })
	m.Close(context.Background(), r.ID)
	if _, err := os.Stat(filepath.Join(m.StateDir, "editor", r.ID)); !os.IsNotExist(err) {
		t.Fatalf("the editor's directory outlived its review: %v", err)
	}

	// A closed review can still be fixed, and its editor talked to: the
	// editor needs the cache, not the run's worktree, which is gone.
	if _, err := os.Stat(r.dir); !os.IsNotExist(err) {
		t.Fatalf("the closed run's worktree is still there: %v", err)
	}
	if _, err := m.StartEditor(context.Background(), r.ID, ch, who, EditorTask{}); err != nil {
		t.Fatalf("an editor on a closed run: %v", err)
	}
	v = wait(t, r, func(v RunView) bool {
		return len(v.Sessions) == 4 && v.Sessions[3].FirstComplete && v.Sessions[3].Status == Idle
	})
	if v.Status != Closed {
		t.Fatalf("the editor reopened the run: %s", v.Status)
	}
	if err := m.Say(r.ID, v.Sessions[3].ID, "and the rest"); err != nil {
		t.Fatalf("talking to a closed run's editor: %v", err)
	}
	if err := m.Say(r.ID, v.Sessions[0].ID, "hello"); err == nil {
		t.Fatal("a closed run's reviewer was resumed")
	}
	wait(t, r, func(v RunView) bool { return v.Sessions[3].Status == Idle && len(v.Sessions[3].Log) > 3 })
	if detail, err := m.Editor(context.Background(), r.ID); err != nil || len(detail.Commits) == 0 {
		t.Fatalf("a closed run's editor: %+v %v", detail, err)
	}
	if err := m.DiscardEditor(context.Background(), r.ID); err != nil {
		t.Fatal(err)
	}
}

// A head the clone fetched but never checked out lacks its files, and the
// editor's shallow seed needs them, so they are brought in first.
func TestTheEditorSeedsAHeadWhoseFilesTheCloneLacks(t *testing.T) {
	dir, base, head := remote(t)
	git := func(args ...string) string {
		out, err := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("config", "uploadpack.allowFilter", "true")
	state := t.TempDir()
	repos := &Repos{Dir: filepath.Join(state, "repos")}
	src := Source{Remote: "file://" + dir, Key: "example/g/calc", Head: head, Base: base,
		Refspecs: []string{"+refs/heads/feature:refs/vuoro/head", "+refs/heads/main:refs/vuoro/target"}}
	ctx := context.Background()
	if err := repos.Checkout(ctx, src, filepath.Join(state, "wt")); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "more.go"), []byte("package calc\n\nconst More = 1\n"), 0o644)
	git("add", ".")
	git("commit", "-q", "-m", "more")
	src.Head = git("rev-parse", "HEAD")
	gitDir, _ := repos.gitDir(src.Key)
	if err := repos.fetch(ctx, gitDir, src); err != nil {
		t.Fatal(err)
	}
	missing, err := repos.git(ctx, gitDir, "rev-list", "--objects", "--missing=print", "--no-object-names", src.Head+"^{tree}")
	if err != nil || !strings.Contains(missing, "?") {
		t.Fatalf("the new head's files are already here, so this test proves nothing: %q %v", missing, err)
	}
	wt := filepath.Join(state, "editor", "wt")
	if err := repos.SeedEditor(ctx, src.Key, "run1", src.Head, "vuoro/fix-7-run1", wt); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(wt, "more.go")); err != nil || !strings.Contains(string(b), "More") {
		t.Fatalf("the editor's checkout: %q %v", b, err)
	}
	if err := repos.SeedEditor(ctx, src.Key, "run2", strings.Repeat("ab", 20), "b", filepath.Join(state, "e2")); err == nil || !strings.Contains(err.Error(), "no longer holds") {
		t.Fatalf("a head the clone never had: %v", err)
	}
}

func TestAStackedTitleDropsTheReviewedTitlesDraftMarker(t *testing.T) {
	for in, want := range map[string]string{
		"[PLT-5686] Draft: Cilium install": "Fix review findings on !7: [PLT-5686] Cilium install",
		"Draft: Fix Add":                   "Fix review findings on !7: Fix Add",
		"Redraft the docs":                 "Fix review findings on !7: Redraft the docs",
	} {
		if got := StackTitle(RunView{Title: in, Ref: forge.Ref{Number: 7}}); got != want {
			t.Errorf("StackTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

// A stacked change the forge opened as an ordinary one keeps its URL, and
// is reported as not a draft; any other failure opens nothing.
func TestAStackedChangeThatIsNotADraftKeepsItsURL(t *testing.T) {
	const u = "https://forge.example/o/r/pulls/9"
	for name, c := range map[string]struct {
		url      string
		err      error
		want     string
		notDraft bool
		fails    bool
	}{
		"a draft":     {url: u, want: u},
		"not a draft": {url: u, err: fmt.Errorf("forgejo: %w", forge.ErrNotDraft), want: u, notDraft: true},
		"refused":     {err: errors.New("409"), fails: true},
	} {
		url, notDraft, err := openStacked(func() (string, error) { return c.url, c.err })
		if url != c.want || notDraft != c.notDraft || (err != nil) != c.fails {
			t.Errorf("%s: %q %v %v", name, url, notDraft, err)
		}
	}
}

// On Forgejo the editor's task and the stacked title name a pull request, by
// its number.
func TestAForgejoChangeIsAPullRequest(t *testing.T) {
	v := RunView{Kind: "forgejo", Title: "WIP: Fix Add", Ref: forge.Ref{Number: 7}, WebURL: "https://forge.example/o/r/pulls/7"}
	if got := StackTitle(v); got != "Fix review findings on #7: Fix Add" {
		t.Errorf("StackTitle = %q", got)
	}
	picked := []drafts.Draft{{ID: "t1", ThreadID: "31", Title: "a finding"}}
	for name, msg := range map[string]string{"threads": threadsMessage(v, picked), "findings": findingsMessage(v, picked)} {
		if !strings.Contains(msg, "Pull request: https://forge.example/o/r/pulls/7\n") || strings.Contains(msg, "erge request") {
			t.Errorf("%s message:\n%s", name, msg)
		}
	}
	if !strings.Contains(threadsMessage(RunView{Kind: "gitlab", Ref: forge.Ref{Number: 7}}, picked), "Merge request: ") {
		t.Error("a GitLab change is not a merge request")
	}
}

// On the person's own change, the editor fixes the threads they picked and
// its commits go to the change's own branch, fast-forward only; each fixed
// thread's reply then cites its commit, unless the person wrote the reply
// themselves (D41).
func TestTheEditorFixesPickedThreadsAndPushesToTheChangesOwnBranch(t *testing.T) {
	m, d, ch, src := setup(t, func(call toolCaller, text string) string {
		call("propose_draft", map[string]any{"thread_id": "t1", "category": "must_fix", "reason": "Add subtracts", "description": "Good catch."})
		call("propose_draft", map[string]any{"thread_id": "t2", "category": "nit", "reason": "Naming", "description": "Sure."})
		call("propose_draft", map[string]any{"thread_id": "t3", "category": "discuss", "reason": "A question", "description": "Because."})
		return "Triaged."
	})
	var message string
	d.edit = func(spec agent.Spec, text string) string {
		message = text
		run := func(args ...string) {
			cmd := exec.Command("git", args...)
			cmd.Dir, cmd.Env = spec.Dir, append([]string{"PATH=" + os.Getenv("PATH"), "HOME=" + spec.Dir}, spec.Editor.Env...)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Errorf("git %v: %s", args, out)
			}
		}
		os.WriteFile(filepath.Join(spec.Dir, "calc.go"), []byte("package calc\n\nfunc Add(a, b int) int { return a + b }\n"), 0o644)
		run("commit", "-qam", "Add adds", "-m", "Vuoro-Thread: d1")
		os.WriteFile(filepath.Join(spec.Dir, "name.go"), []byte("package calc\n"), 0o644)
		run("add", "name.go")
		run("commit", "-qm", "Name it", "-m", "Vuoro-Thread: d2")
		return "d1, d2 fixed"
	}
	ch.Kind, ch.State, ch.Author.Username = forge.GitLab, forge.Open, "me"
	note := func(body string) []forge.Note {
		return []forge.Note{{ID: "n-" + body, Author: forge.User{Username: "reviewer"}, Body: body}}
	}
	ch.Threads = []forge.Thread{
		{ID: "t1", Resolvable: true, Anchor: &forge.Anchor{Path: "calc.go", NewLine: 3}, Notes: note("Add subtracts")},
		{ID: "t2", Resolvable: true, Notes: note("Call it something else")},
		{ID: "t3", Resolvable: true, Notes: note("Why this way?")},
	}
	r, err := m.StartTriage(ch, src, Reviewer{Tool: "fake", Model: "m1"})
	if err != nil {
		t.Fatal(err)
	}
	wait(t, r, func(v RunView) bool { return v.Status == Idle && len(v.Drafts) == 3 })
	ctx, who := context.Background(), Identity{Name: "Pat", Email: "pat@example.com"}

	if _, err := m.StartEditor(ctx, r.ID, ch, who, EditorTask{}); err == nil {
		t.Fatal("an editor started with no thread picked")
	}
	if _, err := m.StartEditor(ctx, r.ID, ch, who, EditorTask{Items: []string{"d9"}}); err == nil {
		t.Fatal("an editor started on a thread the run does not have")
	}
	changed := *ch
	changed.Threads = append([]forge.Thread{{ID: "t1", Resolvable: true, Anchor: ch.Threads[0].Anchor, Notes: note("Never mind")}}, ch.Threads[1:]...)
	if _, err := m.StartEditor(ctx, r.ID, &changed, who, EditorTask{Items: []string{"d1"}}); err == nil {
		t.Fatal("an editor started on a thread whose discussion changed since triage")
	}
	ed, err := m.StartEditor(ctx, r.ID, ch, who, EditorTask{Items: []string{"d1", "d2"}})
	if err != nil {
		t.Fatal(err)
	}
	if !ed.Own || !slices.Equal(ed.Items, []string{"d1", "d2"}) {
		t.Fatalf("editor %+v", ed)
	}
	wait(t, r, func(v RunView) bool {
		return len(v.Sessions) == 2 && v.Sessions[1].FirstComplete && v.Sessions[1].Status == Idle
	})
	if !strings.Contains(message, "## Thread d1 (must fix)") || !strings.Contains(message, "reviewer: Add subtracts") ||
		!strings.Contains(message, "`calc.go` line 3") || strings.Contains(message, "Why this way?") {
		t.Fatalf("the editor was sent %q", message)
	}
	d.mu.Lock()
	spec := d.specs[len(d.specs)-1]
	d.mu.Unlock()
	if spec.Drafts != nil || !strings.Contains(spec.SystemPrompt, "Vuoro-Thread") || !strings.Contains(spec.SystemPrompt, "author of a merge request") {
		t.Fatalf("editor spec %+v", spec)
	}

	// The person wrote their own reply to t1, which stands.
	if err := m.EditReply(r.ID, "d1", "Thanks, fixed.", drafts.MustFix); err != nil {
		t.Fatal(err)
	}
	detail, err := m.Editor(ctx, r.ID)
	if err != nil || len(detail.Commits) != 2 || !slices.Equal(detail.Commits[1].Findings, []string{"d2"}) {
		t.Fatalf("detail %+v %v", detail, err)
	}
	sha := detail.Commits[1].SHA
	exec.Command("git", "-C", src.Remote, "switch", "-q", "main").Run() // a forge's repository has no checkout

	if _, err := m.PushEditor(ctx, r.ID, src.Remote, forge.StackTarget{Branch: "feature", Head: "0123456789abcdef0123456789abcdef01234567"}, sha, nil); err == nil || !strings.Contains(err.Error(), "moved") {
		t.Fatalf("a push onto a branch that moved: %v", err)
	}
	p, err := m.PushEditor(ctx, r.ID, src.Remote, forge.StackTarget{Branch: "feature", Head: ch.Diff.Head}, sha, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Branch != "feature" || p.SHA != sha {
		t.Fatalf("pushed %+v", p)
	}
	if out, _ := exec.Command("git", "-C", src.Remote, "rev-parse", "feature").Output(); strings.TrimSpace(string(out)) != sha {
		t.Fatalf("the change's branch is at %q", out)
	}
	v := r.View()
	byID := map[string]drafts.Draft{}
	for _, x := range v.Drafts {
		byID[x.ID] = x
	}
	switch {
	case byID["d1"].FixedIn != detail.Commits[0].SHA || byID["d1"].Description != "Thanks, fixed.":
		t.Fatalf("d1 %+v", byID["d1"])
	case byID["d2"].FixedIn != sha || byID["d2"].Description != "Fixed in "+sha+": Name it":
		t.Fatalf("d2 %+v", byID["d2"])
	case byID["d3"].FixedIn != "" || byID["d3"].Description != "Because.":
		t.Fatalf("d3 %+v", byID["d3"])
	}
	// Once its own push moved the branch, a later push builds on it.
	if _, err := m.PushEditor(ctx, r.ID, src.Remote, forge.StackTarget{Branch: "feature", Head: sha}, sha, nil); err != nil {
		t.Fatalf("a second push: %v", err)
	}
	m.Close(ctx, r.ID)
}

// A review of the person's own merge request posts nothing: they pick
// findings for the editor, whose commits go to the merge request's branch
// (D42).
func TestTheEditorFixesPickedFindingsOfAnOwnReviewOnItsBranch(t *testing.T) {
	m, d, ch, src := setup(t, func(call toolCaller, text string) string {
		call("propose_draft", map[string]any{"severity": "high", "title": "Add subtracts", "description": "d",
			"anchor": map[string]any{"path": "calc.go", "start_line": 3}})
		call("propose_draft", map[string]any{"severity": "low", "title": "Name it", "description": "d"})
		return "done"
	})
	var message string
	d.edit = commitFix(t, &message, "Vuoro-Finding: d1")
	r, err := m.Start(ch, src, []Reviewer{{Profile: "code", Tool: "fake"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	wait(t, r, idle)
	ctx, who := context.Background(), Identity{Name: "Pat", Email: "pat@example.com"}
	m.Decide(r.ID, "d2", drafts.Dropped, "", false)
	if _, err := m.StartEditor(ctx, r.ID, ch, who, EditorTask{Own: true, Items: []string{"d2"}}); err == nil {
		t.Fatal("a dismissed finding was handed to the editor")
	}
	ed, err := m.StartEditor(ctx, r.ID, ch, who, EditorTask{Own: true, Items: []string{"d1"}})
	if err != nil {
		t.Fatal(err)
	}
	if !ed.Own || ed.Local {
		t.Fatalf("editor %+v", ed)
	}
	wait(t, r, func(v RunView) bool {
		return len(v.Sessions) == 2 && v.Sessions[1].FirstComplete && v.Sessions[1].Status == Idle
	})
	if !strings.Contains(message, "picked 1 finding") || !strings.Contains(message, "Finding 1 is d1") || strings.Contains(message, "Name it") {
		t.Fatalf("the editor was sent %q", message)
	}
	detail, err := m.Editor(ctx, r.ID)
	if err != nil || len(detail.Commits) != 1 {
		t.Fatalf("detail %+v %v", detail, err)
	}
	sha := detail.Commits[0].SHA
	exec.Command("git", "-C", src.Remote, "switch", "-q", "main").Run()
	if _, err := m.PushEditor(ctx, r.ID, src.Remote, forge.StackTarget{Branch: "feature", Head: ch.Diff.Head}, sha, nil); err != nil {
		t.Fatal(err)
	}
	if out, _ := exec.Command("git", "-C", src.Remote, "rev-parse", "feature").Output(); strings.TrimSpace(string(out)) != sha {
		t.Fatalf("the change's branch is at %q", out)
	}
	if got := r.View().Drafts[0]; got.FixedIn != sha || got.Description != "d" {
		t.Fatalf("d1 %+v", got)
	}
	m.Close(ctx, r.ID)
}

// A local range's fixes become a new branch in the person's own repository,
// and nothing else there changes: not HEAD, the working tree, or another
// branch, and none of its hooks run (D42).
func TestTheEditorHandsALocalRangesFixesBackAsANewBranch(t *testing.T) {
	m, d, _, _ := setup(t, func(call toolCaller, text string) string {
		call("propose_draft", map[string]any{"severity": "high", "title": "Add subtracts", "description": "d",
			"anchor": map[string]any{"path": "calc.go", "start_line": 3}})
		return "done"
	})
	var message string
	d.edit = commitFix(t, &message, "Vuoro-Finding: d1")
	repo, _, _ := remote(t)
	os.WriteFile(filepath.Join(repo, "notes.txt"), []byte("my own work in progress\n"), 0o644)
	hook := filepath.Join(repo, ".git", "hooks", "post-receive")
	os.WriteFile(hook, []byte("#!/bin/sh\ntouch \""+filepath.Join(repo, "HOOK-RAN")+"\"\n"), 0o755)
	ctx := context.Background()
	lt, err := LocalChange(ctx, "", repo, "main..feature")
	if err != nil {
		t.Fatal(err)
	}
	r, err := m.Start(lt.Change, lt.Source, []Reviewer{{Profile: "code", Tool: "fake"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	wait(t, r, idle)
	who := Identity{Name: "Pat", Email: "pat@example.com"}
	ed, err := m.StartEditor(ctx, r.ID, nil, who, EditorTask{Items: []string{"d1"}})
	if err != nil {
		t.Fatal(err)
	}
	if !ed.Local || ed.Own || ed.Branch != "vuoro/fix-"+r.ID {
		t.Fatalf("editor %+v", ed)
	}
	wait(t, r, func(v RunView) bool {
		return len(v.Sessions) == 2 && v.Sessions[1].FirstComplete && v.Sessions[1].Status == Idle
	})
	detail, err := m.Editor(ctx, r.ID)
	if err != nil || len(detail.Commits) != 1 {
		t.Fatalf("detail %+v %v", detail, err)
	}
	sha := detail.Commits[0].SHA
	git := func(args ...string) string {
		out, _ := exec.Command("git", append([]string{"-C", repo}, args...)...).Output()
		return strings.TrimSpace(string(out))
	}
	headBefore, branchBefore, statusBefore := git("rev-parse", "HEAD"), git("branch", "--show-current"), git("status", "--porcelain")

	// A branch of that name that the person holds is never moved.
	git("branch", ed.Branch, "main")
	if _, err := m.PushEditor(ctx, r.ID, "ignored", forge.StackTarget{}, sha, nil); err == nil {
		t.Fatal("a push moved a branch the person holds")
	}
	git("branch", "-D", ed.Branch)
	p, err := m.PushEditor(ctx, r.ID, "ignored", forge.StackTarget{}, sha, nil)
	if err != nil {
		t.Fatal(err)
	}
	switch {
	case p.Branch != ed.Branch || git("rev-parse", "refs/heads/"+ed.Branch) != sha:
		t.Fatalf("pushed %+v, the branch is at %q", p, git("rev-parse", "refs/heads/"+ed.Branch))
	case git("rev-parse", "HEAD") != headBefore || git("branch", "--show-current") != branchBefore || git("status", "--porcelain") != statusBefore:
		t.Fatal("the push changed the person's checkout")
	}
	if _, err := os.Stat(filepath.Join(repo, "HOOK-RAN")); err == nil {
		t.Fatal("the person's hook ran")
	}
	// vuoro's own branch can be pushed again.
	if _, err := m.PushEditor(ctx, r.ID, "ignored", forge.StackTarget{}, sha, nil); err != nil {
		t.Fatalf("a second push: %v", err)
	}
	if got := r.View().Drafts[0]; got.FixedIn != sha {
		t.Fatalf("d1 %+v", got)
	}
}

// commitFix is an editor turn that fixes Add and commits with trailer.
func commitFix(t *testing.T, message *string, trailer string) func(agent.Spec, string) string {
	return func(spec agent.Spec, text string) string {
		*message = text
		os.WriteFile(filepath.Join(spec.Dir, "calc.go"), []byte("package calc\n\nfunc Add(a, b int) int { return a + b }\n"), 0o644)
		cmd := exec.Command("git", "commit", "-qam", "Add adds", "-m", trailer)
		cmd.Dir, cmd.Env = spec.Dir, append([]string{"PATH=" + os.Getenv("PATH"), "HOME=" + spec.Dir}, spec.Editor.Env...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("git commit: %s", out)
		}
		return "fixed"
	}
}

// A push first asks the editor, once, about each picked finding no commit's
// trailer names (D50). The editor rewords its commit, and the push records
// each named finding's fix with its commit and subject. The person can link
// a fix by hand to one of the review's commits, and take back only a link
// they made.
func TestAPushAsksTheEditorAboutUnnamedFindingsAndThePersonCanLinkAFix(t *testing.T) {
	m, d, ch, src := setup(t, func(call toolCaller, text string) string {
		call("propose_draft", map[string]any{"severity": "high", "title": "Add subtracts", "description": "d",
			"anchor": map[string]any{"path": "calc.go", "start_line": 3}})
		call("propose_draft", map[string]any{"severity": "medium", "title": "Add needs a comment", "description": "d"})
		call("propose_draft", map[string]any{"severity": "low", "title": "Name it", "description": "d"})
		return "done"
	})
	var message, asked string
	first := commitFix(t, &message, "Vuoro-Finding: d1")
	d.edit = func(spec agent.Spec, text string) string {
		if !strings.Contains(text, "No commit names these") {
			return first(spec, text)
		}
		asked = text
		cmd := exec.Command("git", "commit", "-q", "--amend", "-m", "Add adds", "-m", "Vuoro-Finding: d1\nVuoro-Finding: d2")
		cmd.Dir, cmd.Env = spec.Dir, append([]string{"PATH=" + os.Getenv("PATH"), "HOME=" + spec.Dir}, spec.Editor.Env...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("git commit --amend: %s", out)
		}
		return "Reworded the commit to name d2."
	}
	r, err := m.Start(ch, src, []Reviewer{{Profile: "code", Tool: "fake"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	wait(t, r, idle)
	ctx, who := context.Background(), Identity{Name: "Pat", Email: "pat@example.com"}
	if _, err := m.StartEditor(ctx, r.ID, ch, who, EditorTask{Own: true, Items: []string{"d1", "d2"}}); err != nil {
		t.Fatal(err)
	}
	editorIdle := func(v RunView) bool {
		return len(v.Sessions) == 2 && v.Sessions[1].FirstComplete && v.Sessions[1].Status == Idle
	}
	wait(t, r, editorIdle)
	detail, _ := m.Editor(ctx, r.ID)
	sha := detail.Commits[0].SHA
	exec.Command("git", "-C", src.Remote, "switch", "-q", "main").Run()
	target := forge.StackTarget{Branch: "feature", Head: ch.Diff.Head}

	var ask *AskedError
	if _, err := m.PushEditor(ctx, r.ID, src.Remote, target, sha, nil); !errors.As(err, &ask) || strings.Join(ask.IDs, ",") != "d2" {
		t.Fatalf("a push with d2 unnamed: %v", err)
	}
	if out, _ := exec.Command("git", "-C", src.Remote, "rev-parse", "feature").Output(); strings.TrimSpace(string(out)) != ch.Diff.Head {
		t.Fatal("a push that stopped to ask pushed anyway")
	}
	wait(t, r, func(v RunView) bool { return editorIdle(v) && asked != "" })
	if !strings.Contains(asked, "d2: Add needs a comment") || strings.Contains(asked, "d1:") || !strings.Contains(asked, "Vuoro-Finding: <id>") {
		t.Fatalf("the editor was asked %q", asked)
	}
	detail, _ = m.Editor(ctx, r.ID)
	sha = detail.Commits[0].SHA
	if _, err := m.PushEditor(ctx, r.ID, src.Remote, target, sha, nil); err != nil {
		t.Fatalf("the push after the editor answered: %v", err)
	}
	for _, d := range r.View().Drafts[:2] {
		if d.FixedIn != sha || d.FixedSubject != "Add adds" || d.FixedBy != "editor" {
			t.Fatalf("%s's fix: %+v", d.ID, d)
		}
	}

	// The person links d3 by hand, to a commit of the review only.
	commits, err := m.FixCommits(ctx, r.ID, nil)
	if err != nil || len(commits) != 1 || commits[0].SHA != sha {
		t.Fatalf("fix commits %+v %v", commits, err)
	}
	if err := m.MarkFixed(ctx, r.ID, "d3", ch.Diff.Base, nil); err == nil {
		t.Fatal("a commit outside the review was linked as a fix")
	}
	if err := m.MarkFixed(ctx, r.ID, "d3", sha, nil); err != nil {
		t.Fatal(err)
	}
	if d3 := r.View().Drafts[2]; d3.FixedIn != sha || d3.FixedBy != "person" {
		t.Fatalf("d3 %+v", d3)
	}
	if err := m.MarkFixed(ctx, r.ID, "d1", "", nil); err == nil {
		t.Fatal("the person took back a fix the editor's commit names")
	}
	if err := m.MarkFixed(ctx, r.ID, "d3", "", nil); err != nil || r.View().Drafts[2].FixedIn != "" {
		t.Fatalf("taking back d3's fix: %v", err)
	}
	m.Close(ctx, r.ID)
}

// fixFinding is an editor turn that commits a change naming the first
// finding it was handed.
func fixFinding(t *testing.T) func(agent.Spec, string) string {
	pass := 0
	return func(spec agent.Spec, text string) string {
		m := regexp.MustCompile(`Finding 1 is (d\d+)`).FindStringSubmatch(text)
		if m == nil {
			return "nothing to fix"
		}
		pass++
		os.WriteFile(filepath.Join(spec.Dir, "calc.go"), []byte(fmt.Sprintf("package calc\n\nfunc Add(a, b int) int { return a + b }\n\n// wave %d\n", pass)), 0o644)
		cmd := exec.Command("git", "commit", "-qam", "Fix "+m[1], "-m", "Vuoro-Finding: "+m[1])
		cmd.Dir, cmd.Env = spec.Dir, append([]string{"PATH=" + os.Getenv("PATH"), "HOME=" + spec.Dir}, spec.Editor.Env...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("git commit: %s", out)
		}
		return "fixed " + m[1]
	}
}

// A review is fixed in waves (D50): once the first editor's work is pushed
// to the person's own branch, a second starts from that push with findings
// not yet fixed, and pushes on top of it. The first is kept in the run's
// waves with its commits. Anyone else's push stops a new wave.
func TestAReviewIsFixedInWavesFromTheLastPush(t *testing.T) {
	m, d, ch, src := setup(t, func(call toolCaller, text string) string {
		call("propose_draft", map[string]any{"severity": "high", "title": "Add subtracts", "description": "d",
			"anchor": map[string]any{"path": "calc.go", "start_line": 3}})
		call("propose_draft", map[string]any{"severity": "medium", "title": "Add needs a comment", "description": "d"})
		return "done"
	})
	d.edit = fixFinding(t)
	r, err := m.Start(ch, src, []Reviewer{{Profile: "code", Tool: "fake"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	wait(t, r, idle)
	ctx, who := context.Background(), Identity{Name: "Pat", Email: "pat@example.com"}
	editorIdle := func(n int) func(RunView) bool {
		return func(v RunView) bool {
			return len(v.Sessions) == n && v.Sessions[n-1].FirstComplete && v.Sessions[n-1].Status == Idle
		}
	}
	if _, err := m.StartEditor(ctx, r.ID, ch, who, EditorTask{Own: true, Items: []string{"d1"}}); err != nil {
		t.Fatal(err)
	}
	wait(t, r, editorIdle(2))
	// Work not pushed holds the review.
	if _, err := m.StartEditor(ctx, r.ID, ch, who, EditorTask{Own: true, Items: []string{"d2"}}); !errors.Is(err, ErrEditorBusy) {
		t.Fatalf("a second wave before the first was pushed: %v", err)
	}
	detail, _ := m.Editor(ctx, r.ID)
	first := detail.Commits[0].SHA
	exec.Command("git", "-C", src.Remote, "switch", "-q", "main").Run()
	if _, err := m.PushEditor(ctx, r.ID, src.Remote, forge.StackTarget{Branch: "feature", Head: ch.Diff.Head}, first, nil); err != nil {
		t.Fatal(err)
	}

	// The change is now at vuoro's own push, which does not count as moved.
	now := *ch
	now.Diff.Head = first
	if _, err := m.StartEditor(ctx, r.ID, &now, who, EditorTask{Own: true, Items: []string{"d1"}}); err == nil {
		t.Fatal("a fixed finding was handed to a new wave")
	}
	ed, err := m.StartEditor(ctx, r.ID, &now, who, EditorTask{Own: true, Items: []string{"d2"}})
	if err != nil {
		t.Fatalf("a second wave: %v", err)
	}
	if ed.Base != first || ed.Continues == nil || ed.Continues.SHA != first || ed.Branch != detail.Branch {
		t.Fatalf("the second wave %+v", ed)
	}
	v := wait(t, r, editorIdle(3))
	if len(v.Waves) != 1 || v.Waves[0].Pushed.SHA != first || len(v.Waves[0].Commits) != 1 || v.Sessions[1].Status != Closed {
		t.Fatalf("waves %+v, first editor %s", v.Waves, v.Sessions[1].Status)
	}
	detail, _ = m.Editor(ctx, r.ID)
	if len(detail.Commits) != 1 || detail.Commits[0].Subject != "Fix d2" {
		t.Fatalf("the second wave's commits %+v", detail.Commits)
	}
	second := detail.Commits[0].SHA
	if _, err := m.PushEditor(ctx, r.ID, src.Remote, forge.StackTarget{Branch: "feature", Head: first}, second, nil); err != nil {
		t.Fatalf("the second wave's push: %v", err)
	}
	if out, _ := exec.Command("git", "-C", src.Remote, "rev-parse", "feature").Output(); strings.TrimSpace(string(out)) != second {
		t.Fatalf("the change's branch is at %q", out)
	}
	if got := r.View().Drafts[1]; got.FixedIn != second {
		t.Fatalf("d2 %+v", got)
	}
	if commits, err := m.FixCommits(ctx, r.ID, nil); err != nil || len(commits) != 2 {
		t.Fatalf("fix commits across waves %+v %v", commits, err)
	}

	// A push by anyone else stops another wave.
	moved := now
	moved.Diff.Head = "0123456789abcdef0123456789abcdef01234567"
	if _, err := m.StartEditor(ctx, r.ID, &moved, who, EditorTask{Own: true, Items: []string{"d2"}}); err == nil || !strings.Contains(err.Error(), "review it again") {
		t.Fatalf("a wave on a change someone else moved: %v", err)
	}

	// Waves outlive a restart.
	m2 := &Manager{StateDir: m.StateDir, Repos: m.Repos, Drivers: m.Drivers, Drafts: m.Drafts, MCPURL: m.MCPURL}
	if err := m2.Load(); err != nil {
		t.Fatal(err)
	}
	if back := m2.Get(r.ID).View(); len(back.Waves) != 1 || back.Editor == nil || back.Editor.Continues == nil {
		t.Fatalf("reloaded waves %+v editor %+v", back.Waves, back.Editor)
	}
	m.Close(ctx, r.ID)
}

// On a local range, a second wave moves vuoro's own fix branch on from the
// first wave's push, and still never a branch the person holds (D42, D50).
func TestALocalRangesSecondWaveMovesItsOwnBranchOn(t *testing.T) {
	m, d, _, _ := setup(t, func(call toolCaller, text string) string {
		call("propose_draft", map[string]any{"severity": "high", "title": "Add subtracts", "description": "d",
			"anchor": map[string]any{"path": "calc.go", "start_line": 3}})
		call("propose_draft", map[string]any{"severity": "medium", "title": "Add needs a comment", "description": "d"})
		return "done"
	})
	d.edit = fixFinding(t)
	repo, _, _ := remote(t)
	ctx := context.Background()
	lt, err := LocalChange(ctx, "", repo, "main..feature")
	if err != nil {
		t.Fatal(err)
	}
	r, err := m.Start(lt.Change, lt.Source, []Reviewer{{Profile: "code", Tool: "fake"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	wait(t, r, idle)
	who := Identity{Name: "Pat", Email: "pat@example.com"}
	git := func(args ...string) string {
		out, _ := exec.Command("git", append([]string{"-C", repo}, args...)...).Output()
		return strings.TrimSpace(string(out))
	}
	for i, item := range []string{"d1", "d2"} {
		ed, err := m.StartEditor(ctx, r.ID, nil, who, EditorTask{Items: []string{item}})
		if err != nil {
			t.Fatalf("wave %d: %v", i+1, err)
		}
		wait(t, r, func(v RunView) bool {
			n := len(v.Sessions)
			return n == 2+i && v.Sessions[n-1].FirstComplete && v.Sessions[n-1].Status == Idle
		})
		detail, _ := m.Editor(ctx, r.ID)
		sha := detail.Commits[len(detail.Commits)-1].SHA
		if _, err := m.PushEditor(ctx, r.ID, "ignored", forge.StackTarget{}, sha, nil); err != nil {
			t.Fatalf("wave %d's push: %v", i+1, err)
		}
		if got := git("rev-parse", "refs/heads/"+ed.Branch); got != sha {
			t.Fatalf("wave %d: the branch is at %q, want %s", i+1, got, sha)
		}
	}
	if v := r.View(); len(v.Waves) != 1 || v.Drafts[0].FixedIn == "" || v.Drafts[1].FixedIn == "" {
		t.Fatalf("waves %d, drafts %+v", len(v.Waves), v.Drafts)
	}
	m.Close(ctx, r.ID)
}

// Discarding the editor's work before a push leaves the forge as it was:
// every ref of the change's repository is unchanged, and the editor's
// commits are gone from vuoro too.
func TestDiscardingUnpushedWorkLeavesTheForgeUntouched(t *testing.T) {
	m, d, ch, src := setup(t, func(call toolCaller, text string) string {
		call("propose_draft", map[string]any{"title": "Add subtracts", "description": "It subtracts.", "severity": "high",
			"anchor": map[string]any{"path": "calc.go", "start_line": 3}})
		return "done"
	})
	d.edit = func(spec agent.Spec, text string) string {
		os.WriteFile(filepath.Join(spec.Dir, "calc.go"), []byte("package calc\n\nfunc Add(a, b int) int { return a + b }\n"), 0o644)
		for _, args := range [][]string{{"add", "calc.go"}, {"commit", "-q", "-m", "Add adds", "-m", "Vuoro-Finding: d1"}} {
			cmd := exec.Command("git", args...)
			cmd.Dir, cmd.Env = spec.Dir, append([]string{"PATH=" + os.Getenv("PATH"), "HOME=" + spec.Dir}, spec.Editor.Env...)
			if out, err := cmd.CombinedOutput(); err != nil {
				return "git: " + string(out)
			}
		}
		return "d1: fixed"
	}
	refs := func() string {
		out, err := exec.Command("git", "-C", src.Remote, "for-each-ref", "--format=%(refname) %(objectname)").Output()
		if err != nil {
			t.Fatal(err)
		}
		return string(out)
	}
	before := refs()
	r, err := m.Start(ch, src, []Reviewer{{Profile: "code", Tool: "fake", Model: "m1"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	wait(t, r, idle)
	m.Decide(r.ID, "d1", drafts.Kept, "41", false)
	if _, err := m.StartEditor(context.Background(), r.ID, ch, Identity{Name: "Pat", Email: "pat@example.com"}, EditorTask{}); err != nil {
		t.Fatal(err)
	}
	wait(t, r, func(v RunView) bool { return idle(v) && len(v.Sessions) == 2 && v.Sessions[1].FirstComplete })
	detail, err := m.Editor(context.Background(), r.ID)
	if err != nil || len(detail.Commits) != 1 {
		t.Fatalf("the editor committed %+v %v", detail, err)
	}
	if err := m.DiscardEditor(context.Background(), r.ID); err != nil {
		t.Fatal(err)
	}
	if after := refs(); after != before {
		t.Fatalf("the change's repository moved on a discard:\nbefore %s\nafter  %s", before, after)
	}
	if exec.Command("git", "-C", src.Remote, "cat-file", "-e", detail.Commits[0].SHA).Run() == nil {
		t.Fatal("the discarded commit reached the change's repository")
	}
	if v := r.View(); v.Editor == nil || !v.Editor.Discarded || v.Editor.Pushed != nil {
		t.Fatalf("editor after the discard: %+v", v.Editor)
	}
	if _, err := m.Editor(context.Background(), r.ID); err == nil {
		t.Fatal("the discarded editor's commits are still offered for a push")
	}
}
