package server

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestListenRefusesEveryAddressButLoopback(t *testing.T) {
	for _, addr := range []string{
		"0.0.0.0:0", ":0", "[::]:0", "[::1]:0", "localhost:0",
		"127.0.0.2:0", "192.168.1.10:0", "example.com:0", "127.0.0.1",
	} {
		ln, err := Listen(addr)
		if err == nil {
			ln.Close()
			t.Errorf("Listen(%q) succeeded", addr)
			continue
		}
		if addr != "127.0.0.1" && !errors.Is(err, ErrNotLoopback) {
			t.Errorf("Listen(%q): %v, want ErrNotLoopback", addr, err)
		}
	}
}

func TestListenBindsLoopback(t *testing.T) {
	ln, err := Listen("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if got := ln.Addr().String(); got[:10] != "127.0.0.1:" {
		t.Fatalf("bound %s", got)
	}
}

const testToken = "per-launch-token"

// start serves New on a real loopback listener, as vuoro does, and returns
// its address.
func start(t *testing.T) string {
	t.Helper()
	ln, err := Listen("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	dist := fstest.MapFS{"index.html": {Data: []byte("<h1>vuoro</h1>")}}
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTeapot) })
	srv := &httptest.Server{Listener: ln, Config: HTTPServer(New(Config{Addr: addr, Token: testToken, Dist: dist, Version: "v-test", API: api}))}
	srv.Start()
	t.Cleanup(srv.Close)
	return addr
}

// get sends a GET to path on addr, with edit applied to the request first.
func get(t *testing.T, addr, path string, edit func(*http.Request)) *http.Response {
	t.Helper()
	req, err := http.NewRequest("GET", "http://"+addr+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if edit != nil {
		edit(req)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { res.Body.Close() })
	return res
}

func withToken(tok string) func(*http.Request) {
	return func(r *http.Request) { r.Header.Set(TokenHeader, tok) }
}

func TestTheAPIAnswersWithTheToken(t *testing.T) {
	addr := start(t)
	res := get(t, addr, "/api/hello", withToken(testToken))
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/hello: %s", res.Status)
	}
	var body struct{ Version string }
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Version != "v-test" {
		t.Fatalf("version %q", body.Version)
	}
}

func TestTheAPIRefusesARequestWithoutTheToken(t *testing.T) {
	addr := start(t)
	for name, edit := range map[string]func(*http.Request){
		"no token":      nil,
		"empty token":   withToken(""),
		"wrong token":   withToken("not-the-token"),
		"token prefix":  withToken(testToken[:5]),
		"token as auth": func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+testToken) },
	} {
		for _, path := range []string{"/api/hello", "/api", "/api/nope", "/api/runs", "/api/runs/x/close"} {
			if res := get(t, addr, path, edit); res.StatusCode != http.StatusForbidden {
				t.Errorf("%s, GET %s: %s, want 403", name, path, res.Status)
			}
		}
	}
}

func TestEveryRouteRefusesAForeignHostOrOrigin(t *testing.T) {
	addr := start(t)
	foreign := map[string]func(*http.Request){
		// DNS rebinding: a page on evil.example resolves its name to
		// 127.0.0.1, and the browser sends its own name as Host.
		"rebound host":   func(r *http.Request) { r.Host = "evil.example:" + portOf(addr) },
		"localhost host": func(r *http.Request) { r.Host = "localhost:" + portOf(addr) },
		"other port":     func(r *http.Request) { r.Host = "127.0.0.1:1" },
		"foreign origin": func(r *http.Request) { r.Header.Set("Origin", "http://evil.example") },
		"other port origin": func(r *http.Request) {
			r.Header.Set("Origin", "http://127.0.0.1:1")
		},
		"null origin": func(r *http.Request) { r.Header.Set("Origin", "null") },
		"cross-site":  func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "cross-site") },
		"same-site":   func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "same-site") },
	}
	for name, edit := range foreign {
		for _, path := range []string{"/", "/index.html", "/api/hello"} {
			res := get(t, addr, path, func(r *http.Request) {
				withToken(testToken)(r)
				edit(r)
			})
			if res.StatusCode != http.StatusForbidden {
				t.Errorf("%s, GET %s: %s, want 403", name, path, res.Status)
			}
		}
	}
}

func TestThePageLoadsWithoutTheToken(t *testing.T) {
	addr := start(t)
	for name, edit := range map[string]func(*http.Request){
		"plain":       nil,
		"same origin": func(r *http.Request) { r.Header.Set("Origin", "http://"+addr) },
		"navigation":  func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "none") },
	} {
		if res := get(t, addr, "/", edit); res.StatusCode != http.StatusOK {
			t.Errorf("%s, GET /: %s", name, res.Status)
		}
	}
}

func TestTokensAreFreshAndLong(t *testing.T) {
	a, b := NewToken(), NewToken()
	if a == b {
		t.Fatal("two tokens are equal")
	}
	if len(a) < 43 {
		t.Fatalf("token %d characters, want at least 43", len(a))
	}
}

func TestTheAPIRoutesSitBehindTheToken(t *testing.T) {
	addr := start(t)
	for _, method := range []string{"GET", "POST"} {
		req, _ := http.NewRequest(method, "http://"+addr+"/api/runs", nil)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusForbidden {
			t.Fatalf("%s /api/runs without the token: %s", method, res.Status)
		}
		req.Header.Set(TokenHeader, testToken)
		if res, err = http.DefaultClient.Do(req); err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusTeapot {
			t.Fatalf("%s /api/runs with the token: %s, want it to reach the API", method, res.Status)
		}
	}
}

func portOf(addr string) string {
	_, port, _ := net.SplitHostPort(addr)
	return port
}

func TestTheMCPRouteHasItsOwnCredentialBehindTheGuard(t *testing.T) {
	ln, err := Listen("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	mcp := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTeapot) })
	srv := &httptest.Server{Listener: ln, Config: HTTPServer(New(Config{Addr: addr, Token: testToken, Dist: fstest.MapFS{}, MCP: mcp}))}
	srv.Start()
	defer srv.Close()

	// The MCP server authenticates its sessions itself, without the API token.
	if res := get(t, addr, "/mcp/", nil); res.StatusCode != http.StatusTeapot {
		t.Fatalf("GET /mcp/: %s, want it to reach the MCP handler", res.Status)
	}
	// The guard still stands in front of it.
	if res := get(t, addr, "/mcp/", func(r *http.Request) { r.Host = "evil.example:" + portOf(addr) }); res.StatusCode != http.StatusForbidden {
		t.Fatalf("GET /mcp/ with a foreign host: %s", res.Status)
	}
}
