package github

import (
	"context"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
)

// TestLiveWrite round-trips the person's writes on a sandbox pull request
// they opened: inline comments, a reply, a comment on the whole change, an
// edit and a delete, Submit, and resolving. It writes to GitHub, so it runs
// only against a pull request named for it. Approving and requesting changes
// need a second account; as the author, GitHub refuses both, and the test
// checks that it says so.
//
// VUORO_LIVE_GITHUB_WRITE_PR=https://github.com/OWNER/SANDBOX/pull/N
// VUORO_LIVE_GITHUB_TOKEN=... go test -run TestLiveWrite -v ./forge/github
func TestLiveWrite(t *testing.T) {
	raw := os.Getenv("VUORO_LIVE_GITHUB_WRITE_PR")
	if raw == "" {
		t.Skip("set VUORO_LIVE_GITHUB_WRITE_PR and VUORO_LIVE_GITHUB_TOKEN to write to a sandbox pull request")
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
	c.KeepDraftsIn(&memStore{})
	ch, err := c.Open(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if len(ch.Drafts) != 0 {
		t.Fatalf("the sandbox has %d drafts already; submit or delete them first", len(ch.Drafts))
	}
	var path string
	var line int
	for _, f := range ch.Files {
		if line = addedLine(f.Patch); line > 0 {
			path = f.NewPath
			break
		}
	}
	if path == "" {
		t.Fatal("the sandbox has no added line to comment on")
	}
	stamp := time.Now().UTC().Format(time.RFC3339)
	at := &forge.Anchor{Path: path, OldPath: path, NewLine: line, Diff: ch.Diff}
	inline, err := c.CreateDraft(ctx, ref, forge.NewDraft{Body: "Inline, " + stamp, Anchor: at})
	if err != nil {
		t.Fatal(err)
	}
	gone, err := c.CreateDraft(ctx, ref, forge.NewDraft{Body: "Deleted, " + stamp, Anchor: at})
	if err != nil {
		t.Fatal(err)
	}
	var thread string
	for _, th := range ch.Threads {
		if strings.HasPrefix(th.ID, threadPrefix) {
			thread = th.ID
		}
	}
	if thread != "" {
		if _, err := c.CreateDraft(ctx, ref, forge.NewDraft{Body: "Reply, " + stamp, ThreadID: thread}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := c.CreateDraft(ctx, ref, forge.NewDraft{Body: "Summary, " + stamp}); err != nil {
		t.Fatal(err)
	}
	if err := c.UpdateDraft(ctx, ref, inline.ID, "Inline, edited, "+stamp); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteDraft(ctx, ref, gone.ID); err != nil {
		t.Fatal(err)
	}
	// The drafts are the person's alone: the facts the inbox reads have none.
	facts, err := c.ReadFacts(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(notes(facts), stamp) || len(facts.Drafts) != 0 {
		t.Fatal("a pending comment is in the facts")
	}
	open, err := c.Open(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	want := 2
	if thread != "" {
		want = 3
	}
	if len(open.Drafts) != want || !slices.ContainsFunc(open.Drafts, func(d forge.Draft) bool { return d.ID == inline.ID && strings.HasPrefix(d.Body, "Inline, edited") }) {
		t.Fatalf("drafts %+v", open.Drafts)
	}
	// The author can neither approve nor request changes, and GitHub says so.
	if err := c.Approve(ctx, ref, true); err == nil || !strings.Contains(err.Error(), "pending comments") {
		t.Fatalf("approving with pending comments: %v", err)
	}
	if err := c.Submit(ctx, ref, forge.VerdictChangesRequested); err == nil || !strings.Contains(strings.ToLower(err.Error()), "own pull request") {
		t.Fatalf("requesting changes as the author: %v", err)
	}
	if err := c.Submit(ctx, ref, forge.VerdictCommented); err != nil {
		t.Fatal(err)
	}
	after, err := c.Open(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Drafts) != 0 {
		t.Fatalf("drafts left after submitting: %+v", after.Drafts)
	}
	var body, edited bool
	var mine string
	for _, th := range after.Threads {
		for _, n := range th.Notes {
			body = body || n.Body == "Summary, "+stamp
			if n.Body == "Inline, edited, "+stamp {
				edited, mine = true, th.ID
			}
			if strings.HasPrefix(n.Body, "Deleted, ") && strings.HasSuffix(n.Body, stamp) {
				t.Fatal("the deleted comment was published")
			}
		}
	}
	if !body || !edited {
		t.Fatalf("published: body %v, inline %v", body, edited)
	}
	if err := c.Resolve(ctx, ref, mine, true); err != nil {
		t.Fatal(err)
	}
	if err := c.Resolve(ctx, ref, mine, false); err != nil {
		t.Fatal(err)
	}
	if err := c.Approve(ctx, ref, true); err == nil || !strings.Contains(strings.ToLower(err.Error()), "own pull request") {
		t.Fatalf("approving as the author: %v", err)
	}
	t.Logf("%s: inline, reply %v, summary, edit and delete kept pending, then published as one review; thread resolved and reopened", ref.Display(forge.GitHub), thread != "")
}

// addedLine is the new-side line of a patch's first added line, or 0.
func addedLine(patch string) int {
	n := 0
	for _, l := range strings.Split(patch, "\n") {
		switch {
		case strings.HasPrefix(l, "@@"):
			// @@ -a,b +c,d @@: the next new-side line is c.
			plus := strings.Fields(l)[2]
			start, _, _ := strings.Cut(strings.TrimPrefix(plus, "+"), ",")
			n, _ = strconv.Atoi(start)
		case strings.HasPrefix(l, "+"):
			return n
		case strings.HasPrefix(l, "-"):
		default:
			n++
		}
	}
	return 0
}

// notes is every published note's text, run together.
func notes(ch *forge.Change) string {
	var b strings.Builder
	for _, th := range ch.Threads {
		for _, n := range th.Notes {
			b.WriteString(n.Body + "\n")
		}
	}
	return b.String()
}
