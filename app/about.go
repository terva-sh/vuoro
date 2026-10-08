package app

import (
	"net/http"
	"runtime"
	"runtime/debug"
)

// About is what this vuoro was built from and where it keeps its files, for
// the window's About pane and for a bug report. It holds no secret: the
// config path names a file, and the file names token sources, never tokens.

// moduleView is a linked module as built. A replacement keeps the module's
// own path, with the replacement's version, and names the replacement's path
// when it is another module.
type moduleView struct {
	Path     string `json:"path"`
	Version  string `json:"version"`
	Replaced string `json:"replaced_by,omitempty"`
}

type aboutView struct {
	Version string `json:"version"`
	// Commit, Modified, and Built are the source the binary was built from,
	// when the build recorded them: go build in a checkout does.
	Commit   string       `json:"commit,omitempty"`
	Modified bool         `json:"modified,omitempty"`
	Built    string       `json:"built,omitempty"`
	Go       string       `json:"go"`
	Platform string       `json:"platform"`
	Modules  []moduleView `json:"modules"`
	Config   string       `json:"config_path,omitempty"`
	State    string       `json:"state_dir,omitempty"`
}

func (a *App) about(w http.ResponseWriter, _ *http.Request) {
	v := aboutView{Version: a.Version, Go: runtime.Version(), Platform: runtime.GOOS + "/" + runtime.GOARCH,
		Modules: []moduleView{}, Config: a.ConfigPath, State: a.StateDir}
	if v.Version == "" {
		v.Version = "dev"
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		aboutBuild(&v, info)
	}
	writeJSON(w, http.StatusOK, v)
}

// aboutBuild fills in what the build recorded: the source's commit, and the
// modules linked into the binary.
func aboutBuild(v *aboutView, info *debug.BuildInfo) {
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			v.Commit = s.Value
		case "vcs.time":
			v.Built = s.Value
		case "vcs.modified":
			v.Modified = s.Value == "true"
		}
	}
	for _, m := range info.Deps {
		mv := moduleView{Path: m.Path, Version: m.Version}
		if r := m.Replace; r != nil {
			mv.Version = r.Version
			if r.Path != m.Path {
				mv.Replaced = r.Path
			}
		}
		v.Modules = append(v.Modules, mv)
	}
}
