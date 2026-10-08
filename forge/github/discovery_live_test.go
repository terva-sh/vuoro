package github

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
)

// TestLiveDiscovery lists the token's account's open pull requests and
// reads the facts of each, without writing.
//
// VUORO_LIVE_GITHUB_DISCOVERY=1 VUORO_LIVE_GITHUB_USERNAME=login
// VUORO_LIVE_GITHUB_TOKEN=... go test -run TestLiveDiscovery -v ./forge/github
func TestLiveDiscovery(t *testing.T) {
	if os.Getenv("VUORO_LIVE_GITHUB_DISCOVERY") != "1" {
		t.Skip("set VUORO_LIVE_GITHUB_DISCOVERY=1, VUORO_LIVE_GITHUB_USERNAME and VUORO_LIVE_GITHUB_TOKEN to discover a live inbox")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	token, err := config.Forge{TokenEnv: "VUORO_LIVE_GITHUB_TOKEN"}.Token(ctx)
	if err != nil {
		t.Fatal(err)
	}
	host := os.Getenv("VUORO_LIVE_GITHUB_HOST")
	if host == "" {
		host = "github.com"
	}
	c, err := New("https://"+host, token)
	if err != nil {
		t.Fatal(err)
	}
	found, err := c.Discover(ctx, os.Getenv("VUORO_LIVE_GITHUB_USERNAME"))
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range found {
		ch, err := c.ReadFacts(ctx, d.Ref)
		if err != nil {
			t.Fatalf("%s: %v", d.Ref.Display(forge.GitHub), err)
		}
		t.Logf("%s %v: %d reviews, %d threads", d.Ref.Display(forge.GitHub), d.Roles, len(ch.Reviews), len(ch.Threads))
	}
	t.Logf("%d open pull requests", len(found))
}
