// Package server is vuoro's loopback HTTP server: the API and the embedded
// frontend.
//
// Every process on the machine can reach it (docs/plan.md section 5.2).
// TKT-01M3W4HWDY8XV92MXXQWA0AES7 (Refuse loopback API requests that did not
// come from vuoro's window) puts every route behind a check, and it lands
// before the server carries anything a forge token can reach.
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

// New returns the server's handler, serving the frontend from dist.
func New(dist fs.FS, version string) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServerFS(dist))
	mux.HandleFunc("GET /api/hello", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"version": version})
	})
	return mux
}

// HTTPServer wraps h with the timeouts every vuoro server uses.
func HTTPServer(h http.Handler) *http.Server {
	return &http.Server{Handler: h, ReadHeaderTimeout: 10 * time.Second}
}
