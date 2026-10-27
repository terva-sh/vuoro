package forge

import (
	"errors"
	"testing"
)

func TestProjectURLKeepsOnlyTheProject(t *testing.T) {
	for _, c := range []struct {
		raw   string
		parts int
		want  string
	}{
		{"https://gitlab.com/group/sub/repo", 0, "group/sub/repo"},
		{"https://gitlab.com/group/sub/repo/-/merge_requests/3", 0, "group/sub/repo"},
		{"https://GitLab.com/group/repo.git", 0, "group/repo"},
		{"https://github.com/owner/repo/pull/9", 2, "owner/repo"},
		{"https://git.example/owner/repo/", 2, "owner/repo"},
	} {
		p, err := ProjectURL(c.raw, c.parts)
		if err != nil || p.Path != c.want {
			t.Errorf("%s: %+v %v", c.raw, p, err)
		}
	}
	for _, raw := range []string{"http://gitlab.com/a/b", "https://gitlab.com/a", "https://user:pw@gitlab.com/a/b", "https://gitlab.com/a/../b", "not a url"} {
		if _, err := ProjectURL(raw, 0); !errors.Is(err, ErrNotAProject) {
			t.Errorf("%s accepted: %v", raw, err)
		}
	}
	if _, err := ProjectURL("https://github.com/owner", 2); !errors.Is(err, ErrNotAProject) {
		t.Error("an owner alone is not a project")
	}
}
