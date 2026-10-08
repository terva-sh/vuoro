package adapters

import (
	"errors"
	"testing"

	"github.com/terva-sh/vuoro/forge"
)

func TestRoutingHonorsConfiguredKindsAndAnonymousPaths(t *testing.T) {
	for _, tc := range []struct {
		url, configured, kind, project string
		n                              int
		bad                            bool
	}{
		{"https://gitlab.example/a/b/c/-/merge_requests/12/diffs?x=1#note", "", forge.GitLab, "a/b/c", 12, false},
		{"https://FORGE.example/o/r/pulls/7/files#note", "", forge.Forgejo, "o/r", 7, false},
		{"https://forge.example/o/r/pulls/7", forge.GitLab, "", "", 0, true},
		{"https://gitlab.example/o/r/-/merge_requests/7", forge.Forgejo, "", "", 0, true},
		// GitHub's Forgejo-shaped path is not a GitHub pull request.
		{"https://github.com/o/r/pulls/7", "", "", "", 0, true},
		{"https://GitHub.com/o/r/pull/7/files", "", forge.GitHub, "o/r", 7, false},
		{"https://ghe.example/o/r/pull/7", "", forge.GitHub, "o/r", 7, false},
		{"https://ghe.example/o/r/pull/7", forge.Forgejo, "", "", 0, true},
		{"https://forge.example/o/r/pulls/0", "", "", "", 0, true},
		{"https://forge.example/o/r/s/pulls/1", "", "", "", 0, true},
		{"https://forge.example/o/../pulls/1", "", "", "", 0, true},
		{"https://forge.example/o/%2e%2e/pulls/1", "", "", "", 0, true},
		{"https://user:secret@forge.example/o/r/pulls/1", "", "", "", 0, true},
		{"http://forge.example/o/r/pulls/1", "", "", "", 0, true},
	} {
		ref, kind, err := ParseURL(tc.url, tc.configured)
		if tc.bad {
			if err == nil {
				t.Errorf("accepted %s", tc.url)
			}
			continue
		}
		if err != nil || kind != tc.kind || ref.Project != tc.project || ref.Number != tc.n {
			t.Errorf("%s: %+v %s %v", tc.url, ref, kind, err)
		}
	}
	_, _, err := ParseURL("https://forge.example/o/r/pulls/1", "unknown")
	if !errors.Is(err, forge.ErrUnsupported) {
		t.Fatal("unknown kind was guessed")
	}
}
