package forgejo

import (
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
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
)

// teaTransport uses tea's authentication in opt-in read-only checks, without
// extracting credentials or opening its auth file. It is test-only.
type teaTransport struct{ host, login string }

func (tr teaTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Method != "GET" || r.URL.Host != tr.host {
		return nil, errors.New("live check refused an unexpected request")
	}
	endpoint := strings.TrimPrefix(r.URL.EscapedPath(), "/api/v1/")
	if r.URL.RawQuery != "" {
		endpoint += "?" + r.URL.RawQuery
	}
	cmd := exec.CommandContext(r.Context(), "tea", "api", "--login", tr.login, endpoint)
	b, err := cmd.Output()
	if err != nil {
		return nil, errors.New("tea could not complete the read-only live check")
	}
	return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(b))}, nil
}

func teaLogin(ctx context.Context, host, username string) (string, error) {
	// login list exposes account metadata, never tokens. Select by both server
	// and username rather than trusting the CLI's default login or repository.
	cmd := exec.CommandContext(ctx, "tea", "login", "list", "--output", "json")
	b, err := cmd.Output()
	if err != nil {
		return "", errors.New("could not list tea account metadata")
	}
	var rows []struct {
		Name string `json:"name"`
		URL  string `json:"url"`
		User string `json:"user"`
	}
	if json.Unmarshal(b, &rows) != nil {
		return "", errors.New("invalid tea account metadata")
	}
	var matches []string
	for _, r := range rows {
		u, err := url.Parse(r.URL)
		if err == nil && u.Scheme == "https" && strings.EqualFold(u.Host, host) && strings.EqualFold(r.User, username) {
			matches = append(matches, r.Name)
		}
	}
	if len(matches) != 1 {
		return "", errors.New("live comparison requires one tea login matching the configured host and username")
	}
	return matches[0], nil
}

// Run explicitly with VUORO_LIVE_FORGEJO_DISCOVERY=1, username and host. tea
// must already be authenticated to that same host. No credentials are arguments.
func TestLiveDiscovery(t *testing.T) {
	if os.Getenv("VUORO_LIVE_FORGEJO_DISCOVERY") != "1" {
		t.Skip("opt-in live account comparison")
	}
	host := os.Getenv("VUORO_LIVE_FORGEJO_HOST")
	username := os.Getenv("VUORO_LIVE_FORGEJO_USERNAME")
	if host == "" || username == "" {
		t.Fatal("live comparison requires host and username")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	login, err := teaLogin(ctx, host, username)
	if err != nil {
		t.Fatal(err)
	}
	c, err := New("https://"+host, config.SecretForTest("live-CLI-transport-placeholder"))
	if err != nil {
		t.Fatal(err)
	}
	c.http = &http.Client{Transport: teaTransport{host: host, login: login}}
	members, err := c.Discover(ctx, username)
	if err != nil {
		t.Fatal(err)
	}
	for _, filter := range []struct {
		name string
		role forge.Role
	}{{"created", forge.AuthorRole}, {"review_requested", forge.ReviewerRole}, {"assigned", forge.AssigneeRole}} {
		expected := []string{}
		for page := 1; page <= 1000; page++ {
			cmd := exec.CommandContext(ctx, "tea", "api", "--login", login, "repos/issues/search?state=open&type=pulls&"+filter.name+"=true&limit=100&page="+strconv.Itoa(page))
			b, err := cmd.Output()
			if err != nil {
				t.Fatal("tea could not read the independent account list")
			}
			var rows []apiIssue
			if json.Unmarshal(b, &rows) != nil {
				t.Fatal("invalid independent account response")
			}
			if len(rows) == 0 {
				break
			}
			for _, row := range rows {
				ref, err := ParseURL(row.HTMLURL)
				if err != nil {
					t.Fatal("invalid live identity")
				}
				expected = append(expected, ref.String())
			}
		}
		actual := []string{}
		for _, m := range members {
			if slices.Contains(m.Roles, filter.role) {
				actual = append(actual, m.Ref.String())
			}
		}
		slices.Sort(expected)
		slices.Sort(actual)
		if filter.role == forge.ReviewerRole {
			// Historical requested reviewers are deliberately added to the pending list.
			for _, ref := range expected {
				if !slices.Contains(actual, ref) {
					t.Fatal("pending requested review missing")
				}
			}
		} else if !slices.Equal(slices.Compact(expected), slices.Compact(actual)) {
			t.Fatal("discovery differs from the Forgejo account list")
		}
	}
	for _, m := range members {
		ch, err := c.ReadFacts(ctx, m.Ref)
		if err != nil {
			t.Fatalf("%s: complete turn facts: %v", m.Ref, err)
		}
		if ch.Checkout.HeadRef != "refs/pull/"+strconv.Itoa(m.Ref.Number)+"/head" || forge.ValidateCheckout(ch.Checkout, host) != nil {
			t.Fatal("live checkout metadata incorrect")
		}
	}
	t.Logf("Matched authored/requested/assigned lists and read complete facts/checkout metadata for %d open PRs", len(members))
}
