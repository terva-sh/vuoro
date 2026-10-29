package app

import (
	"net/http"
	"path/filepath"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/review"
)

// promptRoutes serve the canned prompts to ask about a change with
// (section 6.11, D68).
func (a *App) promptRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/prompts", a.listPrompts)
}

// promptsDir is where the person's prompt files are: prompts/ beside the
// configuration file.
func (a *App) promptsDir() string {
	if a.PromptsDir != "" {
		return a.PromptsDir
	}
	if a.ConfigPath != "" {
		return filepath.Join(filepath.Dir(a.ConfigPath), "prompts")
	}
	if d, err := config.Dir(); err == nil {
		return filepath.Join(d, "prompts")
	}
	return ""
}

// prompts reads the prompt files on each call, so an edited file shows
// without a restart.
func (a *App) prompts() ([]review.Prompt, []review.PromptError) {
	return review.LoadPrompts(a.promptsDir()) // an empty dir reads as none of theirs
}

func (a *App) listPrompts(w http.ResponseWriter, r *http.Request) {
	list, bad := a.prompts()
	if bad == nil {
		bad = []review.PromptError{}
	}
	writeJSON(w, http.StatusOK, struct {
		Dir     string               `json:"dir"`
		Prompts []review.Prompt      `json:"prompts"`
		Errors  []review.PromptError `json:"errors"`
	}{a.promptsDir(), list, bad})
}
