package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sync"

	"github.com/terva-sh/vuoro/state"
)

// ErrChanged means the configuration file is no longer the one a save was
// based on: someone edited it meanwhile, and a save would lose that edit.
var ErrChanged = errors.New("config: the file changed since vuoro read it; open setup again to see what it holds now")

// Read reads the configuration file at path, and the hash a later Save must
// match. A missing file reads as an empty configuration with an empty hash.
func Read(path string) (*Config, string, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &Config{}, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	c, err := Parse(b)
	if err != nil {
		return nil, "", err
	}
	return c, hash(b), nil
}

func hash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// saveMu makes each save's check and write one step, so two saves from the
// same base cannot both pass the check.
var saveMu sync.Mutex

// Save writes c to path, mode 0600, replacing the file whole. base is the
// hash Read returned: a file that changed since, or one that does not parse,
// is not overwritten. c is checked as Load would check it before anything is
// written, so a save cannot leave a file vuoro refuses to start with.
//
// Saves in this process are serialized. An edit by another program that
// lands between the check and the rename, a window of one write, cannot be
// seen without file locking, which the platforms do not share.
func Save(path string, c *Config, base string) error {
	saveMu.Lock()
	defer saveMu.Unlock()
	out := *c
	if out.Forges == nil {
		out.Forges = []Forge{}
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if _, err := Parse(b); err != nil {
		return err
	}
	cur, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if base != "" {
			return ErrChanged
		}
	case err != nil:
		return err
	default:
		if _, perr := Parse(cur); perr != nil {
			return fmt.Errorf("config: the file at %s does not parse, so vuoro will not overwrite it: %w", path, perr)
		}
		if hash(cur) != base {
			return ErrChanged
		}
	}
	return state.WriteFile(path, b)
}
