package forge

import (
	"errors"
	"testing"
)

func TestParseGitLabURL(t *testing.T) {
	for raw, want := range map[string]Ref{
		"https://gitlab.com/gitlab-org/cli/-/merge_requests/3991":               {"gitlab.com", "gitlab-org/cli", 3991},
		"https://gitlab.com/a/b/c/-/merge_requests/12/diffs?commit_id=x#note_1": {"gitlab.com", "a/b/c", 12},
		" https://git.example.com/group/proj/-/merge_requests/7/ ":              {"git.example.com", "group/proj", 7},
	} {
		got, err := ParseGitLabURL(raw)
		if err != nil || got != want {
			t.Errorf("%q: %+v %v", raw, got, err)
		}
	}
	for _, bad := range []string{
		"http://gitlab.com/a/b/-/merge_requests/1",
		"https://gitlab.com/a/b/-/issues/1",
		"https://gitlab.com/-/merge_requests/1",
		"https://gitlab.com/a/b/-/merge_requests/0",
		"https://gitlab.com/a/b/-/merge_requests/x",
		"https://user:pw@gitlab.com/a/b/-/merge_requests/1",
		"not a url",
	} {
		if _, err := ParseGitLabURL(bad); !errors.Is(err, ErrNotAChange) {
			t.Errorf("%q: %v", bad, err)
		}
	}
}
