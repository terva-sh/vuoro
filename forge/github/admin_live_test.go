package github

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
)

// TestLiveAdminister round-trips each administrative write on a sandbox pull
// request, puts every field back as it found it, and reads the repository's
// open pull requests, members and labels on the way. It writes to GitHub, so
// it runs only against a pull request named for it:
//
//	VUORO_LIVE_GITHUB_ADMIN_PR=https://github.com/OWNER/SANDBOX/pull/N
//	VUORO_LIVE_GITHUB_TOKEN=... go test -run TestLiveAdminister -v ./forge/github
//
// GitHub refuses a review request from a pull request's author to
// themselves, so reviewers round-trip only when VUORO_LIVE_GITHUB_REVIEWER
// names another account with access. Adding the label vuoro-live creates it
// in the repository if it is missing. The test logs counts, never content.
func TestLiveAdminister(t *testing.T) {
	raw := os.Getenv("VUORO_LIVE_GITHUB_ADMIN_PR")
	if raw == "" {
		t.Skip("set VUORO_LIVE_GITHUB_ADMIN_PR and VUORO_LIVE_GITHUB_TOKEN to write to a sandbox pull request")
	}
	ref, err := ParseURL(raw)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	token, err := config.Forge{TokenEnv: "VUORO_LIVE_GITHUB_TOKEN"}.Token(ctx)
	if err != nil {
		t.Fatal(err)
	}
	c, err := New("https://"+ref.Host, token)
	if err != nil {
		t.Fatal(err)
	}
	me, err := c.Me(ctx)
	if err != nil {
		t.Fatal(err)
	}
	open, err := c.ListOpen(ctx, ref.Project)
	if err != nil || !slices.ContainsFunc(open, func(d forge.Discovered) bool { return d.Ref.Number == ref.Number }) {
		t.Fatalf("listing: %d open, %v", len(open), err)
	}
	members, err := c.Members(ctx, ref.Project, "")
	if err != nil {
		t.Fatal(err)
	}
	labels, err := c.Labels(ctx, ref.Project)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d open, %d members, %d labels", len(open), len(members), len(labels))

	read := func() *forge.Change {
		t.Helper()
		ch, err := c.ReadFacts(ctx, ref)
		if err != nil {
			t.Fatal(err)
		}
		return ch
	}
	apply := func(what string, ch forge.Changes) {
		t.Helper()
		if err := c.Administer(ctx, ref, ch); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	// flip sets a field the other way, checks it, and sets it back.
	flip := func(name string, has func(*forge.Change) bool, add, remove forge.Changes) {
		t.Helper()
		before := has(read())
		first, second := add, remove
		if before {
			first, second = remove, add
		}
		apply(name, first)
		if has(read()) == before {
			t.Fatalf("%s did not change", name)
		}
		apply(name+" back", second)
		if has(read()) != before {
			t.Fatalf("%s did not round-trip", name)
		}
	}
	flip("assignee", func(ch *forge.Change) bool {
		return slices.ContainsFunc(ch.Assignees, func(u forge.User) bool { return strings.EqualFold(u.Username, me) })
	}, forge.Changes{AddAssignees: []string{me}}, forge.Changes{RemoveAssignees: []string{me}})
	flip("label", func(ch *forge.Change) bool { return slices.Contains(ch.Labels, "vuoro-live") },
		forge.Changes{AddLabels: []string{"vuoro-live"}}, forge.Changes{RemoveLabels: []string{"vuoro-live"}})
	yes, no := true, false
	flip("draft", func(ch *forge.Change) bool { return ch.Draft }, forge.Changes{Draft: &yes}, forge.Changes{Draft: &no})
	if who := os.Getenv("VUORO_LIVE_GITHUB_REVIEWER"); who != "" {
		flip("reviewer", func(ch *forge.Change) bool {
			return slices.ContainsFunc(ch.Reviewers, func(r forge.Reviewer) bool { return strings.EqualFold(r.Username, who) })
		}, forge.Changes{AddReviewers: []string{who}}, forge.Changes{RemoveReviewers: []string{who}})
	} else {
		t.Log("reviewers not round-tripped: set VUORO_LIVE_GITHUB_REVIEWER")
	}
}
