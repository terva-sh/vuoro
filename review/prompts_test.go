package review

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPromptsAreShippedAndThePersonsOwnSortFirst(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"deps.md":      "---\ntitle: What do the new versions change\nscope: change\ntool: claude\nmodel: opus\n---\nFor each bumped dependency, say what changed.\n",
		"explain.md":   "Explain it to me as if I wrote the base.\n",
		"bad-scope.md": "---\nscope: everything\n---\nHi\n",
		"unknown.md":   "---\nruns: yes\n---\nHi\n",
		"open.md":      "---\ntitle: never closed\nHi\n",
		"empty.md":     "---\ntitle: empty\n---\n",
		"Upper.md":     "Hi\n",
		"notes.txt":    "not a prompt",
	}
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	list, bad := LoadPrompts(dir)
	var names []string
	for _, p := range list {
		names = append(names, p.Name)
	}
	if got := strings.Join(names, ","); got != "deps,explain,risks,test,start-here" {
		t.Fatalf("prompts %s", got)
	}
	deps := list[0]
	if deps.Title != "What do the new versions change" || deps.Scope != "change" || deps.Tool != "claude" || deps.Model != "opus" || deps.Builtin ||
		deps.Text != "For each bumped dependency, say what changed." {
		t.Fatalf("deps %+v", deps)
	}
	if e := list[1]; e.Builtin || e.Title != "explain" || !strings.HasPrefix(e.Text, "Explain it to me") {
		t.Fatalf("the person's explain did not shadow the shipped one: %+v", e)
	}
	if !list[2].Builtin {
		t.Fatal("a shipped prompt is not marked")
	}
	reasons := map[string]string{}
	for _, b := range bad {
		reasons[b.File] = b.Reason
	}
	for _, f := range []string{"bad-scope.md", "unknown.md", "open.md", "empty.md", "Upper.md"} {
		if reasons[f] == "" {
			t.Fatalf("%s was not reported: %+v", f, bad)
		}
	}
	if len(bad) != 5 {
		t.Fatalf("reported %+v", bad)
	}
	if !strings.Contains(reasons["unknown.md"], `"runs"`) {
		t.Fatalf("the reason does not name the field: %s", reasons["unknown.md"])
	}
}

func TestNoPromptsDirectoryIsOnlyTheShippedOnes(t *testing.T) {
	list, bad := LoadPrompts(filepath.Join(t.TempDir(), "absent"))
	if len(bad) != 0 || len(list) != len(BuiltinPrompts) || list[0].Name != Explain {
		t.Fatalf("%+v %+v", list, bad)
	}
	if _, ok := FindPrompt(list, "risks"); !ok {
		t.Fatal("risks is not found")
	}
}
