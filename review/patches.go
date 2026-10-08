package review

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/state"
)

// maxPatch is the largest patch a run keeps of one fix (D51).
const maxPatch = 1 << 20

// Patch is a fix's commit, saved as a patch beside the run record so its diff
// outlives the editor's repository and the branch (D51). A patch over
// maxPatch is not saved: Files names what the commit touched, and TooLarge
// says why there is no diff.
type Patch struct {
	SHA      string    `json:"sha"`
	Subject  string    `json:"subject"`
	By       string    `json:"by"` // "editor", for a commit vuoro pushed or suggested, "person", for one they linked, or "suggestion", for one that applied a suggestion
	TooLarge bool      `json:"too_large,omitempty"`
	Files    []string  `json:"files,omitempty"`
	At       time.Time `json:"at"`
}

// PatchDetail is a saved patch, as the window shows it again.
type PatchDetail struct {
	Patch
	Raw      string           `json:"patch,omitempty"`
	FileDiff []forge.FileDiff `json:"diff"`
}

// ErrNoPatch means the run saved no patch of that commit.
var ErrNoPatch = errors.New("review: this review saved no patch of that commit")

func (r *Run) patchPath(sha string) string {
	return filepath.Join(filepath.Dir(r.path), "patches", strings.ToLower(sha)+".patch")
}

// savePatches saves each commit not yet saved as a patch, read from the
// repository cache with no forge call. A commit the cache does not have is
// skipped; saving is best effort, and never fails what it follows.
func (m *Manager) savePatches(ctx context.Context, r *Run, commits []EditorCommit, by string) {
	gitDir, err := m.Repos.gitDir(r.repoKey)
	if err != nil {
		return
	}
	g := localGit{bin: m.Repos.Git, dir: gitDir}
	for _, c := range commits {
		if !commitRE.MatchString(c.SHA) {
			continue
		}
		r.mu.Lock()
		saved := slices.ContainsFunc(r.patches, func(p Patch) bool { return strings.EqualFold(p.SHA, c.SHA) })
		r.mu.Unlock()
		if saved {
			continue
		}
		p := Patch{SHA: strings.ToLower(c.SHA), Subject: c.Subject, By: by, At: time.Now().UTC()}
		raw, err := g.run(ctx, maxPatch, "format-patch", "--stdout", "-1", "--no-signature", "--no-color", "-M", "--end-of-options", p.SHA)
		switch {
		case errors.Is(err, errTooLarge):
			names, err := g.run(ctx, 1<<20, "show", "--format=", "--name-only", "--no-color", "-M", "--end-of-options", p.SHA)
			if err != nil {
				continue
			}
			p.TooLarge, p.Files = true, lines(names)
		case err != nil:
			continue // not in the cache
		default:
			if err := state.WriteFile(r.patchPath(p.SHA), []byte(raw)); err != nil {
				continue
			}
		}
		r.mu.Lock()
		r.patches = append(r.patches, p)
		r.mu.Unlock()
	}
}

// lines splits git's output into its non-empty lines.
func lines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// Patch reads a saved patch again, with its diff split into files.
func (m *Manager) Patch(runID, sha string) (*PatchDetail, error) {
	r := m.Get(runID)
	if r == nil {
		return nil, ErrNoRun
	}
	r.mu.Lock()
	i := slices.IndexFunc(r.patches, func(p Patch) bool { return strings.EqualFold(p.SHA, sha) })
	var p Patch
	if i >= 0 {
		p = r.patches[i]
	}
	r.mu.Unlock()
	if i < 0 || !commitRE.MatchString(sha) {
		return nil, ErrNoPatch
	}
	out := &PatchDetail{Patch: p, FileDiff: []forge.FileDiff{}}
	if p.TooLarge {
		return out, nil
	}
	b, err := os.ReadFile(r.patchPath(p.SHA))
	if err != nil {
		return nil, ErrNoPatch
	}
	out.Raw, out.FileDiff = string(b), parsePatch(string(b))
	return out, nil
}

// parsePatch splits a patch from git format-patch into its files, as a
// forge would send them. A binary file, or one with no hunks, is omitted.
func parsePatch(raw string) []forge.FileDiff {
	var out []forge.FileDiff
	// The message ends at the first line of three dashes, as for git am, so
	// a message that quotes a diff is not read as one.
	if i := strings.Index(raw, "\n---\n"); i >= 0 {
		raw = raw[i:]
	}
	sections := strings.Split("\n"+raw, "\ndiff --git ")
	for _, sec := range sections[1:] {
		head, _, _ := strings.Cut(sec, "\n")
		f := forge.FileDiff{}
		// "a/old b/new", which is ambiguous only for a path with " b/" in
		// it; the --- and +++ lines, and a rename's lines, settle it.
		if i := strings.Index(head, " b/"); strings.HasPrefix(head, "a/") && i > 0 {
			f.OldPath, f.NewPath = head[2:i], head[i+3:]
		}
		body := sec
		if at := strings.Index(sec, "\n@@ "); at >= 0 {
			body, f.Patch = sec[:at+1], strings.TrimRight(sec[at+1:], "\n")+"\n"
		}
		for _, l := range strings.Split(body, "\n") {
			switch {
			case strings.HasPrefix(l, "new file mode"):
				f.New = true
			case strings.HasPrefix(l, "deleted file mode"):
				f.Deleted = true
			case strings.HasPrefix(l, "rename from "):
				f.OldPath, f.Renamed = strings.TrimPrefix(l, "rename from "), true
			case strings.HasPrefix(l, "rename to "):
				f.NewPath = strings.TrimPrefix(l, "rename to ")
			case strings.HasPrefix(l, "--- a/"):
				f.OldPath = strings.TrimPrefix(l, "--- a/")
			case strings.HasPrefix(l, "+++ b/"):
				f.NewPath = strings.TrimPrefix(l, "+++ b/")
			case strings.HasPrefix(l, "GIT binary patch"), strings.HasPrefix(l, "Binary files "):
				f.Omitted = true
			}
		}
		if f.New {
			f.OldPath = f.NewPath
		}
		if f.Deleted {
			f.NewPath = f.OldPath
		}
		if f.Patch == "" {
			f.Omitted = !f.Renamed
		}
		out = append(out, f)
	}
	return out
}
