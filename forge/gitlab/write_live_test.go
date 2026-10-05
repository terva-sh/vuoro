package gitlab

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/forge/patch"
	"github.com/terva-sh/vuoro/turn"
)

// TestLiveWrite round-trips every write against a real merge request the
// person may write to. It posts visible comments and approves, so it runs only
// when asked, and only on a merge request named for it:
//
//	VUORO_LIVE_GITLAB_WRITE_MR=https://gitlab.com/group/project/-/merge_requests/1 \
//	  go test -run TestLiveWrite -v ./forge/gitlab
//
// The token comes from the forge's token_command or token_env in vuoro's own
// config file, so it is never in the environment of the test command or its
// arguments. The change needs a file with an added line, and the person should
// be one of its reviewers so that approving moves the turn.
func TestLiveWrite(t *testing.T) {
	raw := os.Getenv("VUORO_LIVE_GITLAB_WRITE_MR")
	if raw == "" {
		t.Skip("set VUORO_LIVE_GITLAB_WRITE_MR to a merge request you may write to")
	}
	ref, err := ParseURL(raw)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	c, me := liveWriter(ctx, t, ref.Host)
	nonce := time.Now().UTC().Format("20060102T150405Z")
	open := func() *forge.Change {
		t.Helper()
		ch, err := c.Open(ctx, ref)
		if err != nil {
			t.Fatal(err)
		}
		return ch
	}

	ch := open()
	if len(ch.Drafts) != 0 {
		t.Fatalf("%d draft notes are already pending; submit or delete them first", len(ch.Drafts))
	}
	anchor := addedLine(t, ch)

	// An inline comment and a general one, both pending until Submit.
	if _, err := c.CreateDraft(ctx, ref, forge.NewDraft{Body: "vuoro live inline " + nonce, Anchor: anchor}); err != nil {
		t.Fatalf("inline draft: %v", err)
	}
	general, err := c.CreateDraft(ctx, ref, forge.NewDraft{Body: "vuoro live general " + nonce})
	if err != nil {
		t.Fatalf("general draft: %v", err)
	}
	if err := c.UpdateDraft(ctx, ref, general.ID, "vuoro live general, edited "+nonce); err != nil {
		t.Fatalf("update draft: %v", err)
	}
	dropped, err := c.CreateDraft(ctx, ref, forge.NewDraft{Body: "vuoro live dropped " + nonce})
	if err != nil {
		t.Fatalf("draft to drop: %v", err)
	}
	if err := c.DeleteDraft(ctx, ref, dropped.ID); err != nil {
		t.Fatalf("delete draft: %v", err)
	}
	ch = open()
	if got := draftBodies(ch); !slices.Equal(got, []string{"vuoro live general, edited " + nonce, "vuoro live inline " + nonce}) {
		t.Fatalf("pending drafts before submit: %q", got)
	}
	if hasNote(ch, nonce) != nil {
		t.Fatal("a draft note was visible on the change before Submit")
	}

	if err := c.Submit(ctx, ref); err != nil {
		t.Fatalf("submit: %v", err)
	}
	ch = open()
	if len(ch.Drafts) != 0 {
		t.Fatalf("%d drafts still pending after submit", len(ch.Drafts))
	}
	thread := hasNote(ch, "vuoro live inline "+nonce)
	if thread == nil {
		t.Fatal("the inline comment is not on the change after submit")
	}
	if a := thread.Anchor; a == nil || a.Path != anchor.Path || a.NewLine != anchor.NewLine || a.OldLine != anchor.OldLine {
		t.Fatalf("inline comment landed at %+v, want %s new %d old %d", thread.Anchor, anchor.Path, anchor.NewLine, anchor.OldLine)
	}
	if g := hasNote(ch, "vuoro live general, edited "+nonce); g == nil || g.Anchor != nil {
		t.Fatalf("the edited general comment is missing or anchored: %+v", g)
	}
	if hasNote(ch, "vuoro live dropped "+nonce) != nil {
		t.Fatal("the deleted draft was published")
	}
	t.Logf("inline comment and general comment published together; inline on %s new %d old %d", anchor.Path, anchor.NewLine, anchor.OldLine)

	// A reply, which is a draft note in the thread until it is submitted.
	if _, err := c.CreateDraft(ctx, ref, forge.NewDraft{Body: "vuoro live reply " + nonce, ThreadID: thread.ID}); err != nil {
		t.Fatalf("reply draft: %v", err)
	}
	if err := c.Submit(ctx, ref); err != nil {
		t.Fatalf("submit reply: %v", err)
	}
	ch = open()
	if th := hasNote(ch, "vuoro live reply "+nonce); th == nil || th.ID != thread.ID {
		t.Fatalf("the reply is not in thread %s", thread.ID)
	}

	if err := c.Resolve(ctx, ref, thread.ID, true); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if th := hasNote(open(), "vuoro live inline "+nonce); th == nil || !th.Resolved {
		t.Fatal("the thread is not resolved after Resolve")
	}
	if err := c.Resolve(ctx, ref, thread.ID, false); err != nil {
		t.Fatalf("unresolve: %v", err)
	}
	if th := hasNote(open(), "vuoro live inline "+nonce); th == nil || th.Resolved {
		t.Fatal("the thread is still resolved after unresolve")
	}
	t.Log("reply, resolve, and unresolve round-tripped")

	// Approve and revoke, each followed by the inbox's own turn derivation
	// from freshly discovered roles and facts.
	before := liveTurn(ctx, t, c, ref, me)
	if err := c.Approve(ctx, ref, true); err != nil {
		t.Fatalf("approve: %v", err)
	}
	approved := liveTurn(ctx, t, c, ref, me)
	if err := c.Approve(ctx, ref, false); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	revoked := liveTurn(ctx, t, c, ref, me)
	t.Logf("turn before approve: %s", describe(before))
	t.Logf("turn after approve:  %s", describe(approved))
	t.Logf("turn after revoke:   %s", describe(revoked))
	if !hasRule(before, "R1") {
		t.Fatalf("expected R1 before approving, as a requested reviewer; got %s", describe(before))
	}
	if hasRule(approved, "R1") {
		t.Fatalf("approving did not answer the review request: %s", describe(approved))
	}
	if !hasRule(revoked, "R1") {
		t.Fatalf("revoking did not return the review request: %s", describe(revoked))
	}
}

// liveWriter builds a client from the configured forge on host, with its
// token resolved the way vuoro resolves it.
func liveWriter(ctx context.Context, t *testing.T, host string) (*Client, string) {
	t.Helper()
	path, err := config.Path()
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range cfg.Forges {
		if f.Kind != "gitlab" || !strings.EqualFold(strings.TrimPrefix(strings.TrimPrefix(f.URL, "https://"), "http://"), host) {
			continue
		}
		token, err := f.Token(ctx)
		if err != nil {
			t.Fatal(err)
		}
		c, err := New(f.URL, token)
		if err != nil {
			t.Fatal(err)
		}
		return c, f.Username
	}
	t.Fatalf("no gitlab forge for %s in %s", host, path)
	return nil, ""
}

func addedLine(t *testing.T, ch *forge.Change) *forge.Anchor {
	t.Helper()
	for _, f := range ch.Files {
		for n := 1; n < 10000; n++ {
			old, cur, ok := patch.Line(f.Patch, true, n)
			if ok && old == 0 {
				return &forge.Anchor{Path: f.NewPath, OldPath: f.OldPath, NewLine: cur, Diff: ch.Diff}
			}
		}
	}
	t.Fatal("the change has no added line to comment on")
	return nil
}

func draftBodies(ch *forge.Change) []string {
	var out []string
	for _, d := range ch.Drafts {
		out = append(out, d.Body)
	}
	slices.Sort(out)
	return out
}

func hasNote(ch *forge.Change, text string) *forge.Thread {
	for i := range ch.Threads {
		for _, n := range ch.Threads[i].Notes {
			if !n.System && strings.Contains(n.Body, text) {
				return &ch.Threads[i]
			}
		}
	}
	return nil
}

func liveTurn(ctx context.Context, t *testing.T, c *Client, ref forge.Ref, me string) turn.Turn {
	t.Helper()
	found, err := c.Discover(ctx, me)
	if err != nil {
		t.Fatal(err)
	}
	var roles []forge.Role
	for _, d := range found {
		if d.Ref == ref {
			roles = d.Roles
		}
	}
	if roles == nil {
		t.Fatalf("discovery did not find %s for %s", ref, me)
	}
	ch, err := c.ReadFacts(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	return turn.Derive(*ch, me, roles)
}

func hasRule(tn turn.Turn, rule string) bool {
	return slices.ContainsFunc(tn.Reasons, func(r turn.Reason) bool { return r.Rule == rule })
}

func describe(tn turn.Turn) string {
	var rules []string
	for _, r := range tn.Reasons {
		rules = append(rules, fmt.Sprintf("%s %q", r.Rule, r.Text))
	}
	return fmt.Sprintf("%s [%s]", tn.Owner, strings.Join(rules, "; "))
}
