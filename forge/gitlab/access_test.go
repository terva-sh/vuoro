package gitlab

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

func TestGitLabScopesPlaceATokenOnTheGauge(t *testing.T) {
	for _, c := range []struct {
		scopes []string
		admin  bool
		want   forge.AccessLevel
	}{
		{[]string{"read_user"}, false, forge.AccessNone},
		{[]string{"read_api", "read_user"}, false, forge.AccessRead},
		{[]string{"api"}, false, forge.AccessFull},
		{[]string{"api"}, true, forge.AccessBroad},
		{[]string{"read_api"}, true, forge.AccessRead}, // an administrator's read-only token reads
		{[]string{"api", "sudo"}, false, forge.AccessBroad},
		{[]string{"api", "admin_mode"}, false, forge.AccessBroad},
		{[]string{"sudo", "read_user"}, false, forge.AccessNone}, // broad, and cannot read
	} {
		if got := scopeAccess(c.scopes, c.admin); got.Level != c.want {
			t.Errorf("%v admin %v: %s, want %s", c.scopes, c.admin, got.Level, c.want)
		}
	}
}

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGitLabAccessReadsTheTokensOwnDescription(t *testing.T) {
	reply := func(code int, body string) *http.Response {
		return &http.Response{StatusCode: code, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
	}
	for _, c := range []struct {
		self   string
		code   int
		userUp bool
		want   forge.AccessLevel
	}{
		{`{"scopes":["read_api","read_user"]}`, 200, true, forge.AccessRead},
		{`{"message":"400 Bad request"}`, 400, true, forge.AccessUnknown}, // an OAuth token
		{`{"scopes":["api"]}`, 200, true, forge.AccessFull},
		// Whether an api token is an administrator's decides its place, so a
		// failed lookup leaves it unknown rather than full.
		{`{"scopes":["api"]}`, 200, false, forge.AccessUnknown},
		{`{"scopes":["read_api"]}`, 200, false, forge.AccessRead},
	} {
		cl, err := New("https://gitlab.example", config.SecretForTest("synthetic"))
		if err != nil {
			t.Fatal(err)
		}
		var asked []string
		cl.http.Transport = transport(func(r *http.Request) (*http.Response, error) {
			if r.Method != "GET" {
				t.Fatalf("the access check sent %s %s", r.Method, r.URL.Path)
			}
			asked = append(asked, r.URL.Path)
			if r.URL.Path == "/api/v4/personal_access_tokens/self" {
				return reply(c.code, c.self), nil
			}
			if !c.userUp {
				return reply(502, `{"message":"bad gateway"}`), nil
			}
			return reply(200, `{"username":"pat"}`), nil
		})
		a, err := cl.Access(context.Background())
		if err != nil || a.Level != c.want {
			t.Errorf("%s: %+v %v", c.self, a, err)
		}
		if !slices.Contains(asked, "/api/v4/personal_access_tokens/self") {
			t.Errorf("asked %v", asked)
		}
	}
}
