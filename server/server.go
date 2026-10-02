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
	"net"
	"net/http"
	"net/netip"
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

// New returns the server's handler for a server listening on addr, serving
// the frontend from dist. Every route goes through the guard, and the API
// needs token.
func New(addr, token string, dist fs.FS, version string) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServerFS(dist))
	mux.HandleFunc("GET /api/hello", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"version": version})
	})
	return &guard{host: addr, origin: "http://" + addr, token: []byte(token), next: mux}
}

// HTTPServer wraps h with the timeouts every vuoro server uses.
func HTTPServer(h http.Handler) *http.Server {
	return &http.Server{Handler: h, ReadHeaderTimeout: 10 * time.Second}
}
