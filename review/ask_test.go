package review

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/terva-sh/vuoro/agent"
	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/state"
)

func answers(v RunView) []string {
	var out []string
	for _, e := range v.Sessions[0].Log {
		if e.Kind == "assistant" {
			out = append(out, e.Text)
		}
	}
	return out
}

// A conversation reads the change and nothing else, takes follow-ups on its
// one session, and carries on after a restart by resuming that session.
func TestAnAskIsReadOnlyAndCarriesOnAcrossARestart(t *testing.T) {
	var mu sync.Mutex
	var asked []string
	m, d, ch, src := setup(t, func(call toolCaller, text string) string {
		mu.Lock()
		asked = append(asked, text)
		mu.Unlock()
		if out, isErr := call("list_drafts", nil); !isErr {
			return "reached the drafts: " + out
		}
		if out, isErr := call("propose_draft", map[string]any{"severity": "low", "title": "x", "description": "y"}); !isErr {
			return "drafted: " + out
		}
		if out, isErr := call("git_log", map[string]any{"max_count": 1}); isErr || !strings.Contains(out, "head") {
			return "no history: " + out
		}
		return "answer " + text[:3]
	})
	r, err := m.StartAsk(ch, src, Reviewer{Tool: "fake"}, AskInput{Question: "Why does Add subtract now?"})
	if err != nil {
		t.Fatal(err)
	}
	v := wait(t, r, func(v RunView) bool { return len(answers(v)) == 1 && v.Status == Idle })
	if v.Mode != AskMode || v.Ask == nil || v.Ask.Title != "Why does Add subtract now?" || v.Ask.Prompt != "" || v.Summary.Scope != "change" {
		t.Fatalf("conversation %+v %+v", v.Ask, v.Summary)
	}
	d.mu.Lock()
	spec := d.specs[0]
	d.mu.Unlock()
	// Its one MCP server is vuoro's, offering the context tools and nothing else.
	if spec.Drafts == nil || spec.Drafts.Name != "vuoro" || !slices.Equal(spec.Drafts.Tools, ContextToolNames()) ||
		spec.Editor != nil || spec.Resume != "" || spec.Dir != r.dir || spec.SystemPrompt != askPrompt {
		t.Fatalf("an ask session got more than reading: %+v", spec)
	}
	if got := answers(v)[0]; got != "answer Why" {
		t.Fatalf("answer %q", got)
	}
	mu.Lock()
	first := asked[0]
	mu.Unlock()
	if !strings.HasPrefix(first, "Why does Add subtract now?") || !strings.Contains(first, src.Base+".."+src.Head) || !strings.Contains(first, "-func Add") {
		t.Fatalf("first message lacks the question, range, or diff:\n%s", first)
	}
	if len(v.Drafts) != 0 {
		t.Fatal("a conversation holds drafts")
	}

	if err := m.Say(r.ID, "s1", "And the tests?"); err != nil {
		t.Fatal(err)
	}
	wait(t, r, func(v RunView) bool { return len(answers(v)) == 2 && v.Status == Idle })
	d.mu.Lock()
	if len(d.specs) != 1 {
		t.Fatalf("a follow-up started %d sessions", len(d.specs))
	}
	d.mu.Unlock()

	m.Shutdown()
	loaded := &Manager{StateDir: m.StateDir, Repos: m.Repos, Drivers: map[string]agent.Driver{"fake": d}, Drafts: m.Drafts, MCPURL: m.MCPURL}
	t.Cleanup(loaded.Shutdown)
	if err := loaded.Load(); err != nil {
		t.Fatal(err)
	}
	back := loaded.Get(r.ID)
	if bv := back.View(); bv.Ask == nil || bv.Ask.Question != "Why does Add subtract now?" || bv.Summary == nil || len(answers(bv)) != 2 {
		t.Fatalf("reloaded %+v", bv)
	}
	if err := loaded.Say(r.ID, "s1", "One more"); err != nil {
		t.Fatal(err)
	}
	wait(t, back, func(v RunView) bool { return len(answers(v)) == 3 && v.Status == Idle })
	d.mu.Lock()
	resumed := d.specs[len(d.specs)-1]
	d.mu.Unlock()
	if resumed.Resume != "ref-1" || resumed.Drafts == nil || !slices.Equal(resumed.Drafts.Tools, ContextToolNames()) || resumed.Editor != nil || resumed.SystemPrompt != askPrompt {
		t.Fatalf("resumed as %+v", resumed)
	}
}

func TestAnAskNeedsAQuestionAndACompletedBaseline(t *testing.T) {
	m, _, ch, src := setup(t, func(toolCaller, string) string { return "ok" })
	if _, err := m.StartAsk(ch, src, Reviewer{Tool: "fake"}, AskInput{Question: "  "}); err == nil {
		t.Fatal("an empty question started a conversation")
	}
	if _, err := m.StartAsk(ch, src, Reviewer{Tool: "fake"}, AskInput{Question: "Why?", Baseline: "nope"}); err != ErrNoRun {
		t.Fatalf("an unknown baseline: %v", err)
	}
	moved := *ch
	moved.Diff.Head = strings.Repeat("a", 40)
	if _, err := m.StartAsk(&moved, src, Reviewer{Tool: "fake"}, AskInput{Question: "Why?"}); err == nil {
		t.Fatal("a conversation started on a head the clone was not given")
	}
}

// A summary from before Ask is listed as an explain conversation, and its
// record is not rewritten.
func TestSummariesAreListedAsExplainAndLeftAsTheyAre(t *testing.T) {
	dir := t.TempDir()
	ref := forge.Ref{Host: "lab.test", Project: "g/calc", Number: 7}
	write := func(id, mode string, ref forge.Ref, at time.Time) string {
		rec := map[string]any{"id": id, "kind": forge.GitLab, "mode": mode, "ref": ref, "title": "Fix Add", "status": "ended", "created": at,
			"head": "bbbb", "sessions": []any{}, "summary": SummaryRange{Scope: "change", From: "aaaa", To: "bbbb"}}
		if mode == AskMode {
			rec["ask"] = Ask{Prompt: "risks", Title: "What is risky here", Question: "What could go wrong?"}
		}
		path := filepath.Join(dir, "runs", id, "run.json")
		if err := state.WriteJSON(path, rec); err != nil {
			t.Fatal(err)
		}
		return path
	}
	now := time.Now().UTC()
	old := write("old-summary", "summary", ref, now.Add(-time.Hour))
	write("new-ask", AskMode, ref, now)
	write("a-review", "", ref, now)
	write("other-summary", "summary", forge.Ref{Host: "lab.test", Project: "g/calc", Number: 8}, now)
	before, _ := os.ReadFile(old)
	m := &Manager{StateDir: dir}
	if err := m.Load(); err != nil {
		t.Fatal(err)
	}
	list := m.AsksOf(forge.GitLab, ref)
	if len(list) != 2 || list[0].ID != "new-ask" || list[1].ID != "old-summary" {
		t.Fatalf("asks %+v", list)
	}
	if a := list[1].Ask; a == nil || a.Prompt != Explain || a.Title != "Explain this change" || a.Question != "" {
		t.Fatalf("summary listed as %+v", a)
	}
	if a := list[0].Ask; a == nil || a.Prompt != "risks" {
		t.Fatalf("ask listed as %+v", a)
	}
	if after, _ := os.ReadFile(old); string(after) != string(before) {
		t.Fatal("listing rewrote the summary's record")
	}
	if m.Get("old-summary").View().Ask != nil {
		t.Fatal("the summary run itself gained an ask")
	}
	m.Get("old-summary").save()
	if after, _ := os.ReadFile(old); string(after) == "" || strings.Contains(string(after), `"ask"`) {
		t.Fatalf("the summary record was rewritten with an ask: %s", after)
	}
}

// A conversation about what changed since a completed review is given only
// that range.
func TestAnAskSinceAReviewIsGivenOnlyWhatChanged(t *testing.T) {
	var mu sync.Mutex
	var asked []string
	var fromBase, fromReviewed string
	m, _, ch, src := setup(t, func(call toolCaller, text string) string {
		mu.Lock()
		defer mu.Unlock()
		asked = append(asked, text)
		if strings.Contains(text, "Exact range") {
			fromBase, _ = call("git_diff", map[string]any{"from": "base"})
			fromReviewed, _ = call("git_diff", map[string]any{"from": "reviewed"})
		}
		return "ok"
	})
	old, err := m.Start(ch, src, []Reviewer{{Profile: "code", Tool: "fake"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	reviewed := wait(t, old, func(v RunView) bool { return v.ReviewComplete }).Head
	if _, err := m.StartAsk(ch, src, Reviewer{Tool: "fake"}, AskInput{Question: "What changed?", Baseline: old.ID}); err == nil {
		t.Fatal("a conversation started on an empty range")
	}
	newHead := historyCommit(t, src.Remote, "later-file")
	ch.Diff.Head, src.Head = newHead, newHead
	r, err := m.StartAsk(ch, src, Reviewer{Tool: "fake"}, AskInput{Question: "What changed?", Baseline: old.ID})
	if err != nil {
		t.Fatal(err)
	}
	v := wait(t, r, func(v RunView) bool { return len(answers(v)) == 1 && v.Status == Idle })
	if v.Summary.Scope != "since_review" || v.Summary.ReviewID != old.ID || v.Summary.From != reviewed || v.Summary.To != newHead {
		t.Fatalf("range %+v", v.Summary)
	}
	mu.Lock()
	first := asked[len(asked)-1]
	mu.Unlock()
	if !strings.Contains(first, "later-file") || strings.Contains(first, "-func Add") || !strings.Contains(first, reviewed+".."+newHead) {
		t.Fatalf("the since-review message leaked earlier work:\n%s", first)
	}
	if !strings.Contains(first, "Forge base (base): "+src.Base) {
		t.Fatalf("the first message does not name the forge base:\n%s", first)
	}
	// base is still the change's forge base; reviewed names the review's head.
	mu.Lock()
	defer mu.Unlock()
	if !strings.Contains(fromBase, "-func Add") || !strings.Contains(fromBase, "later-file") {
		t.Fatalf("git_diff from base is not the whole change:\n%s", fromBase)
	}
	if strings.Contains(fromReviewed, "-func Add") || !strings.Contains(fromReviewed, "later-file") {
		t.Fatalf("git_diff from reviewed is not what changed since the review:\n%s", fromReviewed)
	}
}

// A conversation closed while its tool is starting gets no live session: the
// tool started late is closed, and the run stays closed.
func TestAnAskClosedWhileItsToolStartsDoesNotGoOn(t *testing.T) {
	m, _, ch, src := setup(t, nil)
	d := &summaryDriver{started: make(chan struct{}), gate: make(chan struct{})}
	m.Drivers = map[string]agent.Driver{"gated": d}
	r, err := m.StartAsk(ch, src, Reviewer{Tool: "gated"}, AskInput{Question: "Why?"})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-d.started:
	case <-time.After(10 * time.Second):
		t.Fatal("the tool never started")
	}
	if err := m.Close(context.Background(), r.ID); err != nil {
		t.Fatal(err)
	}
	close(d.gate)
	deadline := time.Now().Add(5 * time.Second)
	for {
		d.mu.Lock()
		s := d.session
		d.mu.Unlock()
		s.mu.Lock()
		closed, sends := s.closed, s.sends
		s.mu.Unlock()
		if closed {
			if sends != 0 {
				t.Fatal("the closed conversation was sent its question")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the tool started for a closed conversation was left running")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if v := r.View(); v.Status != Closed {
		t.Fatalf("the run is %s, not closed", v.Status)
	}
}

// A dependency bump is not refused for its lockfile: lockfiles and the files
// the base marks generated are left out of the diff and named with their line
// counts, and a change cannot hide its own code by marking it generated.
func TestALockfileIsLeftOutOfTheDiffAndNamed(t *testing.T) {
	var mu sync.Mutex
	var asked []string
	m, _, ch, src := setup(t, func(_ toolCaller, text string) string {
		mu.Lock()
		asked = append(asked, text)
		mu.Unlock()
		return "ok"
	})
	write := func(name, text string) {
		t.Helper()
		path := filepath.Join(src.Remote, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	historyGit(t, src.Remote, "switch", "-q", "main")
	write(".gitattributes", "gen/** linguist-generated\n")
	write("uv.lock", "version = 1\n")
	historyGit(t, src.Remote, "add", ".")
	historyGit(t, src.Remote, "commit", "-qm", "Mark generated code")
	base := historyGit(t, src.Remote, "rev-parse", "HEAD")
	historyGit(t, src.Remote, "switch", "-qC", "feature")
	write("uv.lock", strings.Repeat("lockfile-line = \"pinned\"\n", 12000)) // about 300 KiB, past the cap
	write("gen/client.go", strings.Repeat("var generatedClient = 1\n", 12000))
	write("gen/small.go", "package gen\n\nvar smallGenerated = 1\n")
	// A small edit to another lockfile is still sent: only what must go to
	// fit under the cap is left out.
	write("package-lock.json", "{\"resolved\": \"https://registry.example/evil\"}\n")
	// A new file named like a lockfile is not one: the base has none there.
	// It has the most lines, so it would be the first left out.
	write("tools/go.sum", "plantedInGoSum\n"+strings.Repeat("p\n", 13000))
	write("calc.go", "package calc\n\nfunc Add(a, b int) int { return a + b + 0 }\n")
	write(".gitattributes", "gen/** linguist-generated\nsneaky.go linguist-generated\n")
	write("sneaky.go", "package calc\n\nvar sneakyChange = 1\n")
	historyGit(t, src.Remote, "add", ".")
	historyGit(t, src.Remote, "commit", "-qm", "Bump dependencies")
	head := historyGit(t, src.Remote, "rev-parse", "HEAD")
	ch.Diff.Base, ch.Diff.Head, src.Base, src.Head = base, head, base, head

	r, err := m.StartAsk(ch, src, Reviewer{Tool: "fake"}, AskInput{Question: "What do the new versions change?"})
	if err != nil {
		t.Fatal(err)
	}
	v := wait(t, r, func(v RunView) bool { return v.Status == Idle || v.Status == Failed })
	if v.Status == Failed {
		t.Fatalf("a lockfile bump was refused: %s", v.Error)
	}
	mu.Lock()
	first := asked[0]
	mu.Unlock()
	if strings.Contains(first, "lockfile-line") || strings.Contains(first, "generatedClient") {
		t.Fatal("a lockfile or generated file's contents were sent")
	}
	for _, want := range []string{"uv.lock +12000 -1 (lockfile)", "gen/client.go +12000 -0 (generated)", "sneakyChange", "a + b + 0", "registry.example/evil", "smallGenerated", "plantedInGoSum"} {
		if !strings.Contains(first, want) {
			t.Fatalf("the first message lacks %q:\n%s", want, first)
		}
	}
	if strings.Contains(first, "sneaky.go +") || strings.Contains(first, "package-lock.json +") || strings.Contains(first, "gen/small.go +") || strings.Contains(first, "tools/go.sum +") {
		t.Fatal("the change hid its own file by marking it generated")
	}
}

// A lockfile edit small enough to send is sent, since it can be the change
// that matters.
func TestASmallLockfileEditIsSent(t *testing.T) {
	var mu sync.Mutex
	var asked []string
	m, _, ch, src := setup(t, func(_ toolCaller, text string) string {
		mu.Lock()
		asked = append(asked, text)
		mu.Unlock()
		return "ok"
	})
	historyGit(t, src.Remote, "switch", "-q", "feature")
	os.WriteFile(filepath.Join(src.Remote, "uv.lock"), []byte("resolved = \"https://registry.example/evil\"\n"), 0o600)
	historyGit(t, src.Remote, "add", ".")
	historyGit(t, src.Remote, "commit", "-qm", "Repoint a dependency")
	head := historyGit(t, src.Remote, "rev-parse", "HEAD")
	ch.Diff.Head, src.Head = head, head
	r, err := m.StartAsk(ch, src, Reviewer{Tool: "fake"}, AskInput{Question: "Anything odd?"})
	if err != nil {
		t.Fatal(err)
	}
	wait(t, r, func(v RunView) bool { return v.Status == Idle })
	mu.Lock()
	first := asked[0]
	mu.Unlock()
	if !strings.Contains(first, "registry.example/evil") || strings.Contains(first, "Files left out") {
		t.Fatalf("a small lockfile edit was left out:\n%s", first)
	}
}

// A since-review range reads attributes at the change's forge base, so an
// attribute the author added in the reviewed commit cannot hide a later edit.
func TestAttributesFromTheReviewedCommitHideNothing(t *testing.T) {
	var mu sync.Mutex
	var asked []string
	m, _, ch, src := setup(t, func(_ toolCaller, text string) string {
		mu.Lock()
		asked = append(asked, text)
		mu.Unlock()
		return "ok"
	})
	// The forge base holds the lockfile, so the later bump of it counts as one.
	historyGit(t, src.Remote, "switch", "-q", "main")
	os.WriteFile(filepath.Join(src.Remote, "uv.lock"), []byte("version = 1\n"), 0o600)
	historyGit(t, src.Remote, "add", ".")
	historyGit(t, src.Remote, "commit", "-qm", "Lock dependencies")
	base := historyGit(t, src.Remote, "rev-parse", "HEAD")
	historyGit(t, src.Remote, "switch", "-q", "feature")
	historyGit(t, src.Remote, "merge", "-q", "--no-edit", "main")
	ch.Diff.Base, src.Base = base, base
	os.WriteFile(filepath.Join(src.Remote, ".gitattributes"), []byte("auth.go linguist-generated\n"), 0o600)
	os.WriteFile(filepath.Join(src.Remote, "auth.go"), []byte("package calc\n\nvar allowed = false\n"), 0o600)
	historyGit(t, src.Remote, "add", ".")
	historyGit(t, src.Remote, "commit", "-qm", "Add auth, marked generated")
	reviewedHead := historyGit(t, src.Remote, "rev-parse", "HEAD")
	ch.Diff.Head, src.Head = reviewedHead, reviewedHead
	old, err := m.Start(ch, src, []Reviewer{{Profile: "code", Tool: "fake"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	wait(t, old, func(v RunView) bool { return v.ReviewComplete })
	// The later commit edits auth.go and is big enough to need leaving out.
	os.WriteFile(filepath.Join(src.Remote, "auth.go"), []byte("package calc\n\nvar allowed = true // later edit\n"), 0o600)
	os.WriteFile(filepath.Join(src.Remote, "uv.lock"), []byte(strings.Repeat("pinned = 1\n", 30000)), 0o600)
	historyGit(t, src.Remote, "add", ".")
	historyGit(t, src.Remote, "commit", "-qm", "Open auth, bump dependencies")
	head := historyGit(t, src.Remote, "rev-parse", "HEAD")
	ch.Diff.Head, src.Head = head, head
	r, err := m.StartAsk(ch, src, Reviewer{Tool: "fake"}, AskInput{Question: "What changed?", Baseline: old.ID})
	if err != nil {
		t.Fatal(err)
	}
	v := wait(t, r, func(v RunView) bool { return len(answers(v)) == 1 || v.Status == Failed })
	if v.Status == Failed {
		t.Fatalf("refused: %s", v.Error)
	}
	mu.Lock()
	first := asked[len(asked)-1]
	mu.Unlock()
	if !strings.Contains(first, "later edit") || strings.Contains(first, "auth.go +") || !strings.Contains(first, "uv.lock +30000") {
		t.Fatalf("the reviewed commit's attribute hid a later edit:\n%s", first)
	}
}
