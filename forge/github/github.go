// Package github reads GitHub pull requests into the neutral forge model.
// Reads go through GraphQL, which GitHub serves only with a token, and the
// patches through REST (D54).
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
)

const maxBody = 32 << 20

var _ forge.Provider = (*Client)(nil)

// ErrNoToken is the refusal to read without an account: GitHub answers
// anonymous GraphQL with 403 (D54).
var ErrNoToken = errors.New("github: reading GitHub needs an account set up in vuoro")

type Client struct {
	// web is the host people browse, which a Ref names. rest and graphql are
	// where its API is served: api.github.com for github.com, and /api/v3 and
	// /api/graphql on the host itself for GitHub Enterprise Server.
	web     *url.URL
	rest    string
	graphql string
	token   config.Secret
	http    *http.Client

	// drafts keeps the person's comments on the whole change until they
	// submit (write.go); nil until the application gives the client a store.
	draftMu sync.Mutex
	drafts  forge.DraftStore

	meMu  sync.Mutex
	login string
}

func New(base string, token config.Secret) (*Client, error) {
	u, err := url.Parse(strings.TrimSuffix(base, "/"))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("github: invalid base URL")
	}
	u.Host = strings.ToLower(u.Host)
	c := &Client{web: u, token: token, http: &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("github: redirects are refused") }}}
	if u.Host == "github.com" {
		c.rest, c.graphql = "https://api.github.com", "https://api.github.com/graphql"
	} else {
		c.rest, c.graphql = u.String()+"/api/v3", u.String()+"/api/graphql"
	}
	return c, nil
}

func ParseURL(raw string) (forge.Ref, error) { return forge.URLRef(raw, "/pull/", 2) }

// Capabilities are the writes this client offers. A reviewer cannot withdraw
// their own approval on GitHub, and vuoro anchors to one line, which a
// GitHub suggestion over several lines needs more than.
func (c *Client) Capabilities() forge.Capabilities {
	c.draftMu.Lock()
	write := !c.token.Empty() && c.drafts != nil
	c.draftMu.Unlock()
	return forge.Capabilities{Drafts: write, Reply: write, Resolve: write, Approve: write, RequestChanges: write}
}

func (c *Client) scrub(s string) string {
	if t := c.token.Reveal(); t != "" {
		s = strings.ReplaceAll(s, t, "[redacted]")
	}
	return s
}

// do sends one request to target, a URL under the client's API. A body is
// sent as JSON. Errors never carry the token, and a redirect is refused
// rather than followed with it.
func (c *Client) do(ctx context.Context, method, target, path string, body any) ([]byte, error) {
	if c.token.Empty() {
		return nil, ErrNoToken
	}
	var payload io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		payload = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, payload)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+c.token.Reveal())
	res, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return nil, fmt.Errorf("github: %s %s: %s", method, path, c.scrub(err.Error()))
	}
	defer res.Body.Close()
	b, err := io.ReadAll(io.LimitReader(res.Body, maxBody+1))
	if err != nil {
		return nil, fmt.Errorf("github: reading response: %s", c.scrub(err.Error()))
	}
	if len(b) > maxBody {
		return nil, errors.New("github: response exceeds 32 MiB")
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		var m struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(b, &m)
		return nil, &forge.APIError{Forge: forge.GitHub, Method: method, Path: path, Status: res.StatusCode, Message: clip(c.scrub(m.Message))}
	}
	return b, nil
}

func clip(s string) string {
	if len(s) > 200 {
		return s[:200]
	}
	return s
}

// get reads a REST path, such as /repos/o/r/pulls/1/files, with its query.
func (c *Client) get(ctx context.Context, path string, q url.Values) ([]byte, error) {
	target := c.rest + path
	if len(q) > 0 {
		target += "?" + q.Encode()
	}
	return c.do(ctx, http.MethodGet, target, path, nil)
}

// query runs one GraphQL operation and decodes its data into out. GitHub
// answers 200 with an errors list for a query it refuses; NOT_FOUND among
// them is forge.ErrNotFound.
func (c *Client) query(ctx context.Context, q string, vars map[string]any, out any) error {
	b, err := c.do(ctx, http.MethodPost, c.graphql, "/graphql", map[string]any{"query": q, "variables": vars})
	if err != nil {
		return err
	}
	var res struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(b, &res); err != nil {
		return errors.New("github: invalid GraphQL response")
	}
	if len(res.Errors) > 0 {
		e := res.Errors[0]
		status := map[string]int{"NOT_FOUND": http.StatusNotFound, "FORBIDDEN": http.StatusForbidden, "UNPROCESSABLE": http.StatusUnprocessableEntity}[e.Type]
		return &forge.APIError{Forge: forge.GitHub, Method: http.MethodPost, Path: "/graphql", Status: status, Message: clip(c.scrub(e.Message))}
	}
	if err := json.Unmarshal(res.Data, out); err != nil {
		return errors.New("github: invalid GraphQL data")
	}
	return nil
}

// Me is the login of the token's account.
func (c *Client) Me(ctx context.Context) (string, error) {
	c.meMu.Lock()
	defer c.meMu.Unlock()
	if c.login != "" {
		return c.login, nil
	}
	var out struct {
		Viewer struct {
			Login string `json:"login"`
		} `json:"viewer"`
	}
	if err := c.query(ctx, `query { viewer { login } }`, nil, &out); err != nil {
		return "", err
	}
	if out.Viewer.Login == "" {
		return "", errors.New("github: the token names no account")
	}
	c.login = out.Viewer.Login
	return c.login, nil
}
