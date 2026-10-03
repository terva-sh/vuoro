package gitlab

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
)

// glabTransport lets opt-in read-only checks use the CLI's authentication without
// extracting its token or opening its auth file. It is never shipped in Vuoro.
type glabTransport struct{ host string }

func (g glabTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Host != g.host || (r.Method != "GET" && r.URL.Path != "/api/graphql") {
		return nil, errors.New("live check refused an unexpected request")
	}
	endpoint := strings.TrimPrefix(r.URL.EscapedPath(), "/api/v4/")
	if r.URL.Path == "/api/graphql" {
		endpoint = "graphql"
	}
	if r.URL.RawQuery != "" {
		endpoint += "?" + r.URL.RawQuery
	}
	args := []string{"api", "--hostname", g.host, "--include", "--method", r.Method, endpoint}
	if r.Body != nil {
		args = append(args, "--input", "-")
	}
	cmd := exec.CommandContext(r.Context(), "glab", args...)
	cmd.Stdin = r.Body
	out, err := cmd.Output()
	if err != nil {
		return nil, errors.New("glab could not complete the read-only live check")
	}
	res, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(out)), r)
	if err != nil {
		return nil, errors.New("glab returned an invalid HTTP response")
	}
	return res, nil
}

// TestLiveDiscovery is opt-in and uses glab's own authenticated API transport.
// Set VUORO_LIVE_GITLAB_DISCOVERY=1 and VUORO_LIVE_GITLAB_USERNAME; optionally
// set VUORO_LIVE_GITLAB_HOST for a self-managed host. No credential is an argument.
func TestLiveDiscovery(t *testing.T) {
	if os.Getenv("VUORO_LIVE_GITLAB_DISCOVERY") != "1" {
		t.Skip("opt-in live account comparison")
	}
	username := os.Getenv("VUORO_LIVE_GITLAB_USERNAME")
	if username == "" {
		t.Fatal("the live check requires a username")
	}
	host := os.Getenv("VUORO_LIVE_GITLAB_HOST")
	if host == "" {
		host = "gitlab.com"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	c, err := New("https://"+host, config.SecretForTest("live-CLI-transport-placeholder"))
	if err != nil {
		t.Fatal(err)
	}
	c.http = &http.Client{Transport: glabTransport{host: host}}
	members, err := c.Discover(ctx, username)
	if err != nil {
		t.Fatal(err)
	}
	actual := []string{}
	for _, m := range members {
		if slices.Contains(m.Roles, forge.ReviewerRole) {
			actual = append(actual, m.Ref.String())
		}
	}
	// Independently request the same filter backing GitLab's review-requested page.
	q := url.Values{"scope": {"all"}, "state": {"opened"}, "reviewer_username": {username}, "per_page": {"100"}}
	cmd := exec.CommandContext(ctx, "glab", "api", "--hostname", host, "--paginate", "merge_requests?"+q.Encode())
	out, err := cmd.Output()
	if err != nil {
		t.Fatal("glab could not read the requested-review list")
	}
	expected := []string{}
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var rows []struct {
			WebURL string `json:"web_url"`
		}
		err := dec.Decode(&rows)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal("invalid requested-review response")
		}
		for _, row := range rows {
			ref, err := ParseURL(row.WebURL)
			if err != nil {
				t.Fatal("invalid requested-review identity")
			}
			expected = append(expected, ref.String())
		}
	}
	slices.Sort(actual)
	slices.Sort(expected)
	if !slices.Equal(slices.Compact(actual), slices.Compact(expected)) {
		t.Fatal("discovery differs from the GitLab requested-review list")
	}
	for _, m := range members {
		ch, err := c.ReadFacts(ctx, m.Ref)
		if err != nil || ch.Ref != m.Ref {
			t.Fatal("a discovered change could not supply complete turn facts")
		}
	}
	t.Logf("Matched %d requested reviews; read complete facts for %d open memberships", len(actual), len(members))
}
