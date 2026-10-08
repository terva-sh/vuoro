package app

import (
	"encoding/json"
	"net/http/httptest"
	"runtime/debug"
	"slices"
	"testing"
)

// The About pane names the release, the source it was built from, and what
// it links, so a bug report can say exactly which vuoro it is about.
func TestAboutNamesTheBuild(t *testing.T) {
	var v aboutView
	aboutBuild(&v, &debug.BuildInfo{
		Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "f0031ce"}, {Key: "vcs.time", Value: "2026-10-09T10:00:00Z"}, {Key: "vcs.modified", Value: "true"}},
		Deps: []*debug.Module{
			{Path: "github.com/terva-sh/tuohi", Version: "v0.1.0"},
			{Path: "golang.org/x/sys", Version: "v0.48.0", Replace: &debug.Module{Path: "golang.org/x/sys", Version: "v0.49.0"}},
			{Path: "github.com/godbus/dbus/v5", Version: "v5.2.2", Replace: &debug.Module{Path: "example.com/fork/dbus", Version: "v5.2.3"}},
		},
	})
	if v.Commit != "f0031ce" || v.Built != "2026-10-09T10:00:00Z" || !v.Modified {
		t.Errorf("source: %+v", v)
	}
	want := []moduleView{
		{Path: "github.com/terva-sh/tuohi", Version: "v0.1.0"},
		{Path: "golang.org/x/sys", Version: "v0.49.0"},
		{Path: "github.com/godbus/dbus/v5", Version: "v5.2.3", Replaced: "example.com/fork/dbus"},
	}
	if !slices.Equal(v.Modules, want) {
		t.Errorf("modules, with each replacement as built and under its own name: %+v", v.Modules)
	}

	a := &App{Version: "0.2.0", ConfigPath: "/home/pat/.config/vuoro/config.json", StateDir: "/home/pat/.local/state/vuoro"}
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/api/about", nil))
	var got aboutView
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Version != "0.2.0" || got.Go == "" || got.Platform == "" || got.Config != a.ConfigPath || got.State != a.StateDir {
		t.Errorf("about: %+v", got)
	}
}
