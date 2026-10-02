package server

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"
)

// TokenHeader carries the per-launch token on every API request.
const TokenHeader = "X-Vuoro-Token"

// NewToken returns a fresh per-launch token: 32 random bytes, base64url.
func NewToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand does not fail on supported platforms
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// guard refuses, with 403, a request that did not come from vuoro's own page
// (docs/plan.md section 5.2). Every route goes through it.
//
//   - Host must be exactly the address the server listens on. A page on
//     another origin that rebinds a name of its own to 127.0.0.1 sends its own
//     name.
//   - Origin, when the browser sends one, must be the server's own origin, and
//     Sec-Fetch-Site, when sent, must say same-origin or none.
//   - A route under /api/ also needs the per-launch token in TokenHeader. The
//     window reads it through a tuohi binding, which tuohi offers only to the
//     origin it trusts, and -browser hands it over in the URL fragment, which
//     is never sent to a server.
//
// The frontend's own files need no token. They are the build output, the
// same for every launch, and a page has to load before it can hold a token.
type guard struct {
	host   string // 127.0.0.1:PORT
	origin string // http://127.0.0.1:PORT
	token  []byte
	next   http.Handler
}

func (g *guard) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !g.allowed(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	g.next.ServeHTTP(w, r)
}

func (g *guard) allowed(r *http.Request) bool {
	if r.Host != g.host {
		return false
	}
	if o := r.Header.Get("Origin"); o != "" && o != g.origin {
		return false
	}
	switch r.Header.Get("Sec-Fetch-Site") {
	case "", "same-origin", "none":
	default:
		return false
	}
	if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
		got := []byte(r.Header.Get(TokenHeader))
		if subtle.ConstantTimeCompare(got, g.token) != 1 {
			return false
		}
	}
	return true
}
