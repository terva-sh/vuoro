package forgejo

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
// open pull requests, members and labels on the way. As in TestLiveWrite,
// tea owns authentication, and every request goes through `tea api --login`:
//
//	VUORO_LIVE_FORGEJO_ADMIN_PR=https://HOST/OWNER/REPO/pulls/N \
//	VUORO_LIVE_FORGEJO_AUTHOR=<tea login> [VUORO_LIVE_FORGEJO_REVIEWER=<another login>] \
//	VUORO_LIVE_FORGEJO_LABEL=<an existing label> \
//	go test -run TestLiveAdminister -v ./forge/forgejo
//
// Forgejo adds only labels that exist, so the label is named. Reviewers
// round-trip when a second login is named. The test logs counts, never
// content.
func TestLiveAdminister(t *testing.T) {
	raw, author, label := os.Getenv("VUORO_LIVE_FORGEJO_ADMIN_PR"), os.Getenv("VUORO_LIVE_FORGEJO_AUTHOR"), os.Getenv("VUORO_LIVE_FORGEJO_LABEL")
	if raw == "" || author == "" || label == "" {
		t.Skip("set VUORO_LIVE_FORGEJO_ADMIN_PR, VUORO_LIVE_FORGEJO_AUTHOR and VUORO_LIVE_FORGEJO_LABEL to write to a sandbox pull request")
	}
	ref, err := ParseURL(raw)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	// The secret only marks the client as signed in; tea sends its own.
	c, err := New("https://"+ref.Host, config.SecretForTest("held-by-tea"))
	if err != nil {
		t.Fatal(err)
	}
	c.http.Transport = teaWriter(author)
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
	if err != nil || !slices.Contains(labels, label) {
		t.Fatalf("labels: %d, %v; %s must exist", len(labels), err, label)
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
	flip("label", func(ch *forge.Change) bool { return slices.Contains(ch.Labels, label) },
		forge.Changes{AddLabels: []string{label}}, forge.Changes{RemoveLabels: []string{label}})
	yes, no := true, false
	flip("draft", func(ch *forge.Change) bool { return ch.Draft }, forge.Changes{Draft: &yes}, forge.Changes{Draft: &no})
	if who := os.Getenv("VUORO_LIVE_FORGEJO_REVIEWER"); who != "" {
		flip("reviewer", func(ch *forge.Change) bool {
			return slices.ContainsFunc(ch.Reviewers, func(r forge.Reviewer) bool { return strings.EqualFold(r.Username, who) })
		}, forge.Changes{AddReviewers: []string{who}}, forge.Changes{RemoveReviewers: []string{who}})
	} else {
		t.Log("reviewers not round-tripped: set VUORO_LIVE_FORGEJO_REVIEWER")
	}
}
