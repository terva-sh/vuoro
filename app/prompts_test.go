package app

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/terva-sh/vuoro/review"
	"github.com/terva-sh/vuoro/server"
)

// prompted is an app whose prompts directory holds one good file and one
// it cannot use.
func prompted(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "deps.md"), []byte("---\ntitle: Dependency impact\n---\nWhat do the new versions change?\n"), 0o600)
	os.WriteFile(filepath.Join(dir, "broken.md"), []byte("---\nscope: all\n---\nHi\n"), 0o600)
	return &App{PromptsDir: dir}
}

// The prompt list is the person's own, then the shipped ones, and names the
// files it could not use.
func TestThePromptListNamesTheFilesItCouldNotUse(t *testing.T) {
	a := prompted(t)
	var out struct {
		Prompts []review.Prompt      `json:"prompts"`
		Errors  []review.PromptError `json:"errors"`
	}
	if err := json.Unmarshal(projectCall(t, a, "GET", "/api/prompts", "", 200), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Prompts) != 1+len(review.BuiltinPrompts) || out.Prompts[0].Name != "deps" || out.Prompts[0].Title != "Dependency impact" || out.Prompts[1].Name != review.Explain {
		t.Fatalf("prompts %+v", out.Prompts)
	}
	if len(out.Errors) != 1 || out.Errors[0].File != "broken.md" || !strings.Contains(out.Errors[0].Reason, "scope") {
		t.Fatalf("errors %+v", out.Errors)
	}
}

func TestThePromptListStaysBehindWindowGuard(t *testing.T) {
	h := server.New(server.Config{Addr: "127.0.0.1:9123", Token: "fixture-window-token", Dist: fstest.MapFS{}, API: prompted(t).Handler()})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "http://127.0.0.1:9123/api/prompts", nil))
	if w.Code != 403 {
		t.Fatal("GET /api/prompts bypassed window guard")
	}
}
