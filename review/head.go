package review

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/terva-sh/vuoro/forge"
)

var commitRE = regexp.MustCompile(`^(?:[0-9a-fA-F]{40}|[0-9a-fA-F]{64})$`)

// HeadState is derived for one exact pair of commits, never saved as run state.
// Behind is known only for a descendant; a rewrite is not a commit count.
type HeadState struct {
	Reviewed string `json:"reviewed_head"`
	Current  string `json:"current_head"`
	Relation string `json:"relation"` // same, ahead, rewritten, unavailable
	Behind   *int   `json:"commits_behind,omitempty"`
}

func (r *Repos) compare(ctx context.Context, src Source, old string) HeadState {
	s := HeadState{Reviewed: old, Current: src.Head, Relation: "unavailable"}
	if !commitRE.MatchString(old) || !commitRE.MatchString(src.Head) || !keyRE.MatchString(src.Key) || strings.Contains(src.Key, "..") {
		return s
	}
	if old == src.Head {
		s.Relation = "same"
		return s
	}
	gitDir := filepath.Join(r.Dir, filepath.FromSlash(src.Key)+".git")
	unlock, err := r.lock(ctx, gitDir)
	if err != nil {
		return s
	}
	defer unlock()
	if ctx.Err() != nil || r.fetch(ctx, gitDir, src) != nil {
		return s
	}
	shallow, err := r.git(ctx, gitDir, "rev-parse", "--is-shallow-repository")
	if err != nil || strings.TrimSpace(shallow) != "false" {
		return s
	}
	for _, sha := range []string{old, src.Head} {
		if _, err := r.git(ctx, gitDir, "cat-file", "-e", sha+"^{commit}"); err != nil {
			return s
		}
	}
	base, err := r.git(ctx, gitDir, "merge-base", old, src.Head)
	if err != nil {
		var exit *exec.ExitError
		if ctx.Err() == nil && errors.As(err, &exit) && exit.ExitCode() == 1 {
			s.Relation = "rewritten"
		}
		return s
	}
	if !strings.EqualFold(strings.TrimSpace(base), old) {
		s.Relation = "rewritten"
		return s
	}
	count, err := r.git(ctx, gitDir, "rev-list", "--count", old+".."+src.Head)
	if err != nil {
		return s
	}
	n, err := strconv.Atoi(strings.TrimSpace(count))
	if err == nil && n > 0 {
		s.Relation, s.Behind = "ahead", &n
	}
	return s
}

// CompareHead reads only forge identity and git history. It never starts tools.
func (m *Manager) CompareHead(ctx context.Context, id string, ch *forge.Change, src Source) (HeadState, error) {
	r := m.Get(id)
	if r == nil {
		return HeadState{}, ErrNoRun
	}
	if r.Ref != ch.Ref || r.Kind != ch.Kind || src.Head != ch.Diff.Head || src.Key != r.repoKey {
		return HeadState{}, errors.New("review: head comparison belongs to another change")
	}
	if m.Repos == nil {
		return HeadState{Reviewed: r.Head, Current: ch.Diff.Head, Relation: "unavailable"}, nil
	}
	return m.Repos.compare(ctx, src, r.Head), nil
}
