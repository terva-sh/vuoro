package gitlab

import (
	"context"
	"errors"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
)

// TestLiveProject is opt-in and read-only, over glab's own authenticated
// transport. Set VUORO_LIVE_GITLAB_PROJECT to a project's full path, and
// optionally VUORO_LIVE_GITLAB_HOST. It confirms the project endpoints of
// docs/plan.md section 6.9 against a real GitLab, and logs counts only.
func TestLiveProject(t *testing.T) {
	path := os.Getenv("VUORO_LIVE_GITLAB_PROJECT")
	if path == "" {
		t.Skip("opt-in live project read")
	}
	host := os.Getenv("VUORO_LIVE_GITLAB_HOST")
	if host == "" {
		host = "gitlab.com"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	c, err := New("https://"+host, config.SecretForTest("live-CLI-transport-placeholder"))
	if err != nil {
		t.Fatal(err)
	}
	c.http = &http.Client{Transport: glabTransport{host: host}}
	p, err := c.Project(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	found, err := c.SearchProjects(ctx, p.Path[len(p.Path)-min(len(p.Path), 6):])
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("search found %d projects", len(found))
	listed, err := c.ListOpen(ctx, p.Path)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d open merge requests", len(listed))
	known, unknown, labelled, piped, conflicted := 0, 0, 0, 0, 0
	for i, m := range listed {
		if i == 10 {
			break
		}
		ch, err := c.ReadFacts(ctx, m.Ref)
		if err != nil {
			t.Fatal(err)
		}
		if len(ch.Labels) > 0 {
			labelled++
		}
		if ch.Pipeline != "" {
			piped++
		}
		if ch.Conflicts {
			conflicted++
		}
		if _, err := c.Dependencies(ctx, m.Ref); errors.Is(err, forge.ErrUnsupported) {
			unknown++
		} else if err != nil {
			t.Fatal(err)
		} else {
			known++
		}
	}
	t.Logf("of the first %d: %d labelled, %d with a pipeline, %d conflicted; dependencies known for %d, unknown for %d",
		min(10, len(listed)), labelled, piped, conflicted, known, unknown)
}
