package forgejo

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

func TestForgejoScopesPlaceATokenOnTheGauge(t *testing.T) {
	for _, c := range []struct {
		scopes  []string
		admin   bool
		want    forge.AccessLevel
		missing []string
	}{
		{[]string{"read:user"}, false, forge.AccessNone, []string{"read:repository", "read:issue"}},
		{[]string{"read:repository", "read:issue", "read:user"}, false, forge.AccessRead, []string{"write:repository", "write:issue"}},
		{[]string{"write:repository", "read:issue", "read:user"}, false, forge.AccessRead, []string{"write:issue"}},
		{[]string{"write:repository", "write:issue", "read:user"}, false, forge.AccessFull, nil},
		{[]string{"write:repository", "write:issue", "read:user"}, true, forge.AccessFull, nil}, // an admin's token without admin scope
		{[]string{"write:repository", "write:issue", "write:admin"}, false, forge.AccessBroad, nil},
		{[]string{"all"}, false, forge.AccessBroad, nil},
		{[]string{"write:repository", "write:issue", "write:organization"}, false, forge.AccessBroad, nil},
		{[]string{"read:user", "write:admin"}, false, forge.AccessNone, []string{"read:repository", "read:issue"}}, // broad, and cannot read
	} {
		got := scopeAccess(c.scopes, c.admin)
		if got.Level != c.want || !slices.Equal(got.Missing, c.missing) {
			t.Errorf("%v admin %v: %s, missing %v; want %s, missing %v", c.scopes, c.admin, got.Level, got.Missing, c.want, c.missing)
		}
	}
}

// The token is found among the person's tokens by its last eight
// characters, and every request the check makes is a read.
func TestForgejoAccessFindsTheTokenAmongThePersonsTokens(t *testing.T) {
	c, _ := fixtureClient(t, false)
	c.token = config.SecretForTest("not-a-token-fixture-12345678")
	c.http.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.Method != "GET" {
			t.Fatalf("the access check sent %s %s", r.Method, r.URL.Path)
		}
		switch r.URL.Path {
		case "/api/v1/user":
			return response(`{"login":"pat","is_admin":false}`), nil
		case "/api/v1/users/pat/tokens":
			if r.URL.Query().Get("page") != "1" {
				return response(`[]`), nil
			}
			return response(`[{"scopes":["all"],"token_last_eight":"aaaaaaaa"},{"scopes":["write:repository","write:issue","read:user"],"token_last_eight":"12345678"}]`), nil
		}
		t.Fatalf("unexpected %s", r.URL.Path)
		return nil, nil
	})
	a, err := c.Access(context.Background())
	if err != nil || a.Level != forge.AccessFull || len(a.Broad) != 0 {
		t.Fatalf("%+v %v", a, err)
	}
}

// Where the list is refused, a read of a repository that does not exist
// says which read scopes are missing: Forgejo checks scope before repository.
func TestForgejoAccessProbesWithReadsWhenTheListIsRefused(t *testing.T) {
	c, _ := fixtureClient(t, false)
	c.token = config.SecretForTest("not-a-token-fixture")
	status := func(code int, body string) *http.Response {
		return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}
	}
	c.http.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.Method != "GET" {
			t.Fatalf("the access check sent %s %s", r.Method, r.URL.Path)
		}
		switch r.URL.Path {
		case "/api/v1/user":
			return response(`{"login":"pat"}`), nil
		case "/api/v1/users/pat/tokens":
			return status(403, `{"message":"token does not have at least one of required scope(s): [read:user]"}`), nil
		case "/api/v1/repos/pat/vuoro-access-check.invalid/pulls":
			return status(404, `{"message":"The target couldn't be found."}`), nil
		case "/api/v1/repos/pat/vuoro-access-check.invalid/issues":
			return status(403, `{"message":"token does not have at least one of required scope(s): [read:issue]"}`), nil
		}
		t.Fatalf("unexpected %s", r.URL.Path)
		return nil, nil
	})
	a, err := c.Access(context.Background())
	if err != nil || a.Level != forge.AccessNone || !slices.Equal(a.Missing, []string{"read:issue"}) {
		t.Fatalf("%+v %v", a, err)
	}
}
