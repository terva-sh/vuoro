package server

import (
	"encoding/json"
	"errors"
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

func TestServesThePageAndTheAPI(t *testing.T) {
	dist := fstest.MapFS{"index.html": {Data: []byte("<h1>vuoro</h1>")}}
	srv := httptest.NewServer(New(dist, "v-test"))
	defer srv.Close()

	res, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /: %s", res.Status)
	}

	res, err = http.Get(srv.URL + "/api/hello")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var body struct{ Version string }
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Version != "v-test" {
		t.Fatalf("version %q", body.Version)
	}
}
