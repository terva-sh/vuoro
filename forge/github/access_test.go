package github

import (
	"context"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
)

func TestClassicScopesPlaceATokenOnTheGauge(t *testing.T) {
	for _, c := range []struct {
		header string
		want   forge.AccessLevel
		broad  []string
	}{
		{"", forge.AccessRead, nil},
		{"read:org, notifications", forge.AccessRead, nil},
		{"public_repo", forge.AccessFull, nil},
		{"repo, read:org", forge.AccessFull, nil},
		{"repo, admin:org, delete_repo", forge.AccessBroad, []string{"admin:org", "delete_repo"}},
		{"repo, workflow", forge.AccessBroad, []string{"workflow"}},
		{"repo, manage_billing:enterprise", forge.AccessBroad, []string{"manage_billing:enterprise"}},
	} {
		got := classicAccess(c.header)
		if got.Level != c.want || !slices.Equal(got.Broad, c.broad) {
			t.Errorf("%q: %s %v, want %s %v", c.header, got.Level, got.Broad, c.want, c.broad)
		}
	}
}

func TestGitHubAccessReadsTheScopeHeaderAndSaysWhenItCannot(t *testing.T) {
	for _, c := range []struct {
		token  string
		header []string
		want   forge.AccessLevel
	}{
		{"ghp_not-a-token", []string{"repo, admin:org"}, forge.AccessBroad},
		{"ghp_not-a-token", nil, forge.AccessUnknown},                 // a token GitHub does not describe
		{"github_pat_not-a-token", []string{""}, forge.AccessUnknown}, // fine-grained
	} {
		cl, err := New("https://github.com", config.SecretForTest(c.token))
		if err != nil {
			t.Fatal(err)
		}
		cl.http.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
			if r.Method != "GET" || r.URL.String() != "https://api.github.com/user" {
				t.Fatalf("the access check sent %s %s", r.Method, r.URL)
			}
			h := http.Header{}
			for _, v := range c.header {
				h.Add("X-OAuth-Scopes", v)
			}
			return &http.Response{StatusCode: 200, Header: h, Body: io.NopCloser(strings.NewReader(`{"login":"pat"}`))}, nil
		})
		a, err := cl.Access(context.Background())
		if err != nil || a.Level != c.want {
			t.Errorf("%s %v: %+v %v", c.token, c.header, a, err)
		}
	}
}
