// Package state writes vuoro's files under the state directory (docs/plan.md
// section 8). A file is written whole, to a temporary name and then renamed,
// so a crash leaves the old file or the new one and never half of either.
// Directories are mode 0700, and files mode 0600.
package state

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// MkdirAll makes dir and its parents, mode 0700.
func MkdirAll(dir string) error { return os.MkdirAll(dir, 0o700) }

// WriteFile writes b to path atomically.
func WriteFile(path string, b []byte) error {
	dir := filepath.Dir(path)
	if err := MkdirAll(dir); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp) // a no-op once renamed
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// WriteJSON writes v to path atomically, as indented JSON.
func WriteJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return WriteFile(path, append(b, '\n'))
}

// ReadJSON reads path into v.
func ReadJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}
