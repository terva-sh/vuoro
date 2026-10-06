package app

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/state"
)

// localDrafts keeps the person's pending comments for a forge that cannot
// keep them privately until they submit (D43): one file per change under
// StateDir/drafts, mode 0600. A file is named by a hash of its change, so a
// host or a project name never becomes a path.
type localDrafts struct{ dir string }

type draftFile struct {
	Change string        `json:"change"`
	Drafts []forge.Draft `json:"drafts"`
}

func (s localDrafts) path(ref forge.Ref) string {
	sum := sha256.Sum256([]byte(ref.String()))
	return filepath.Join(s.dir, hex.EncodeToString(sum[:16])+".json")
}

func (s localDrafts) Drafts(ref forge.Ref) ([]forge.Draft, error) {
	var f draftFile
	err := state.ReadJSON(s.path(ref), &f)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading your pending comments on %s: %w", ref, err)
	}
	if f.Change != ref.String() {
		return nil, fmt.Errorf("your pending comments file for %s names %s", ref, f.Change)
	}
	return f.Drafts, nil
}

// SaveDrafts writes the change's pending comments, and removes its file when
// none are left.
func (s localDrafts) SaveDrafts(ref forge.Ref, drafts []forge.Draft) error {
	if len(drafts) == 0 {
		err := os.Remove(s.path(ref))
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	return state.WriteJSON(s.path(ref), draftFile{Change: ref.String(), Drafts: drafts})
}
