package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

func TestAWaivedConfirmationOutlivesARestartAndCanBeTakenBack(t *testing.T) {
	dir := t.TempDir()
	waived := func(a *App) []string {
		w := httptest.NewRecorder()
		a.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/api/setup", nil))
		var out struct{ Waived []string }
		json.Unmarshal(w.Body.Bytes(), &out)
		return out.Waived
	}
	a := &App{StateDir: dir}
	if got := waived(a); got == nil || len(got) != 0 {
		t.Fatalf("a fresh start waived %v; every action should ask, and the list should be [] not null", got)
	}
	if code, _ := call(t, a.Handler(), "/api/prefs/confirm", map[string]any{"action": "approve", "ask": false}); code != http.StatusOK {
		t.Fatalf("waive: %d", code)
	}
	// A new App reads the same state directory, as vuoro does after a restart.
	if got := waived(&App{StateDir: dir}); !slices.Equal(got, []string{"approve"}) {
		t.Fatalf("after a restart: %v", got)
	}
	call(t, a.Handler(), "/api/prefs/confirm", map[string]any{"action": "approve", "ask": true})
	if got := waived(&App{StateDir: dir}); len(got) != 0 {
		t.Fatalf("asking again left %v", got)
	}
	if code, _ := call(t, a.Handler(), "/api/prefs/confirm", map[string]any{"action": "post-everything", "ask": false}); code == http.StatusOK {
		t.Fatal("an unknown action was waived")
	}
}

func TestAPinnedSchemeOutlivesARestartAndCanFollowTheSystemAgain(t *testing.T) {
	dir := t.TempDir()
	scheme := func(a *App) string {
		w := httptest.NewRecorder()
		a.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/api/setup", nil))
		var out struct{ Scheme string }
		json.Unmarshal(w.Body.Bytes(), &out)
		return out.Scheme
	}
	a := &App{StateDir: dir}
	if got := scheme(a); got != "" {
		t.Fatalf("a fresh start pinned %q; it should follow the system", got)
	}
	if code, _ := call(t, a.Handler(), "/api/prefs/scheme", map[string]any{"scheme": "dark"}); code != http.StatusOK {
		t.Fatalf("pin: %d", code)
	}
	if got := scheme(&App{StateDir: dir}); got != "dark" {
		t.Fatalf("after a restart: %q", got)
	}
	call(t, a.Handler(), "/api/prefs/scheme", map[string]any{"scheme": ""})
	if got := scheme(&App{StateDir: dir}); got != "" {
		t.Fatalf("following the system again left %q", got)
	}
	if code, _ := call(t, a.Handler(), "/api/prefs/scheme", map[string]any{"scheme": "sepia"}); code == http.StatusOK {
		t.Fatal("an unknown scheme was pinned")
	}
}
