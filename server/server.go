// Package server is vuoro's loopback HTTP server: the API and the embedded
// frontend.
//
// Every process on the machine can reach it, so every route goes through the
// guard in guard.go (docs/plan.md section 5.2).
package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/netip"
	"runtime/debug"
	"time"
)

// Listen listens on addr, which must be a 127.0.0.1 address. Any other
// address, a hostname included, is refused, so the server is never reachable
// from another machine.
func Listen(addr string) (net.Listener, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("listen %q: %w", addr, err)
	}
	ip, err := netip.ParseAddr(host)
	if err != nil || ip != netip.MustParseAddr("127.0.0.1") {
		return nil, fmt.Errorf("listen %q: %w", addr, ErrNotLoopback)
	}
	return net.Listen("tcp4", addr)
}

// ErrNotLoopback is returned by Listen for an address other than 127.0.0.1.
var ErrNotLoopback = errors.New("vuoro serves on 127.0.0.1 only")

// Config is what New serves.
type Config struct {
	Addr    string // the address the server listens on, 127.0.0.1:PORT
	Token   string // the per-launch token the API requires
	Dist    fs.FS  // the frontend
	Version string
	// MCP serves the drafts MCP server under /mcp/. It authenticates each
	// agent session with a bearer token of its own, behind the same guard.
	MCP http.Handler
	// API serves the rest of /api/, behind the guard and its token.
	API http.Handler
}

// New returns the server's handler. Every route goes through the guard, and
// the API needs the per-launch token.
func New(c Config) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServerFS(c.Dist))
	mux.HandleFunc("GET /api/hello", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"version": c.Version})
	})
	if c.API != nil {
		for _, m := range []string{"GET", "POST"} {
			mux.Handle(m+" /api/", c.API)
		}
	}
	if c.MCP != nil {
		for _, m := range []string{"POST", "GET", "DELETE"} {
			mux.Handle(m+" /mcp/", c.MCP)
		}
	}
	return &guard{host: c.Addr, origin: "http://" + c.Addr, token: []byte(c.Token), next: recovered{mux}}
}

// recovered answers a request whose handler panicked with a fixed error, so
// the window shows that something failed rather than a dropped connection.
// What panicked, and where, goes only to vuoro's own log, because a panic's
// value can hold anything the handler was holding.
type recovered struct{ next http.Handler }

func (h recovered) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		v := recover()
		if v == nil || v == http.ErrAbortHandler {
			if v != nil {
				panic(v)
			}
			return
		}
		log.Printf("vuoro: panic serving %s %s: %v\n%s", r.Method, r.URL.Path, v, debug.Stack())
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "vuoro hit an internal error and logged it. This is a bug; the request did not complete."})
	}()
	h.next.ServeHTTP(w, r)
}

// HTTPServer wraps h with the timeouts every vuoro server uses.
func HTTPServer(h http.Handler) *http.Server {
	return &http.Server{Handler: h, ReadHeaderTimeout: 10 * time.Second}
}
