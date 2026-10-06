package local

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/terva-sh/vuoro/state"
)

// Store is the person's own choices for the Local tab, kept in local.json in
// the state directory, never in the config file.
type Store struct {
	// Projects are the folders the person added.
	Projects []string `json:"projects"`
	// Roots are the folders vuoro scans for checkouts.
	Roots []string `json:"roots"`
	// Hidden are found projects the person set aside, by git common
	// directory.
	Hidden []string `json:"hidden"`
	// Bases are the base the person chose for a project, by git common
	// directory, in place of origin/HEAD, main, or master.
	Bases map[string]string `json:"bases"`
}

// LoadStore reads the store at path. A missing file is an empty store.
func LoadStore(path string) (Store, error) {
	var s Store
	if path != "" {
		if err := state.ReadJSON(path, &s); err != nil && !errors.Is(err, os.ErrNotExist) {
			return Store{}, err
		}
	}
	s.normalize()
	return s, nil
}

// Save writes the store to path whole.
func (s Store) Save(path string) error {
	if path == "" {
		return nil
	}
	s.normalize()
	return state.WriteJSON(path, s)
}

func (s *Store) normalize() {
	for _, l := range []*[]string{&s.Projects, &s.Roots, &s.Hidden} {
		if *l == nil {
			*l = []string{}
		}
	}
	if s.Bases == nil {
		s.Bases = map[string]string{}
	}
}

// CleanPath is a folder the window named, made absolute and clean. A
// relative path is refused rather than resolved against vuoro's own working
// directory, and a leading ~ means the person's home.
func CleanPath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		p = filepath.Join(home, p[1:])
	}
	if p == "" || strings.ContainsRune(p, 0) || !filepath.IsAbs(p) {
		return "", fmt.Errorf("local: %q is not an absolute path", p)
	}
	p = filepath.Clean(p)
	info, err := os.Stat(p)
	if err != nil {
		return "", fmt.Errorf("local: %s cannot be read", p)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("local: %s is not a folder", p)
	}
	return p, nil
}

// add puts p in l once.
func add(l []string, p string) []string {
	if slices.Contains(l, p) {
		return l
	}
	return append(l, p)
}

// remove takes p out of l.
func remove(l []string, p string) []string {
	return slices.DeleteFunc(slices.Clone(l), func(x string) bool { return x == p })
}

// AddProject, RemoveProject, AddRoot, RemoveRoot, Hide, and Unhide change
// one list. SetBase sets or, with an empty base, clears a project's base.
func (s *Store) AddProject(p string)    { s.Projects = add(s.Projects, p) }
func (s *Store) RemoveProject(p string) { s.Projects = remove(s.Projects, p) }
func (s *Store) AddRoot(p string)       { s.Roots = add(s.Roots, p) }
func (s *Store) RemoveRoot(p string)    { s.Roots = remove(s.Roots, p) }
func (s *Store) Hide(key string)        { s.Hidden = add(s.Hidden, key) }
func (s *Store) Unhide(key string)      { s.Hidden = remove(s.Hidden, key) }
func (s *Store) SetBase(key, base string) {
	s.normalize()
	if base == "" {
		delete(s.Bases, key)
		return
	}
	s.Bases[key] = base
}
