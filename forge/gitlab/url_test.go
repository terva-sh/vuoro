package gitlab

import (
	"errors"
	"testing"

	"github.com/terva-sh/vuoro/forge"
)

func TestParseGitLabURL(t *testing.T) {
	for raw, want := range map[string]forge.Ref{
		"https://gitlab.com/gitlab-org/cli/-/merge_requests/3991":               {Host: "gitlab.com", Project: "gitlab-org/cli", Number: 3991},
		"https://gitlab.com/a/b/c/-/merge_requests/12/diffs?commit_id=x#note_1": {Host: "gitlab.com", Project: "a/b/c", Number: 12},
		" https://git.example.com/group/proj/-/merge_requests/7/ ":              {Host: "git.example.com", Project: "group/proj", Number: 7},
	} {
		got, err := ParseURL(raw)
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
		if _, err := ParseURL(bad); !errors.Is(err, forge.ErrNotAChange) {
			t.Errorf("%q: %v", bad, err)
		}
	}
}
