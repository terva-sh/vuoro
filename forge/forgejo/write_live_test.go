package forgejo

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/forge/patch"
	"github.com/terva-sh/vuoro/turn"
)

// TestLiveWrite reviews a real pull request through the writer, as a
// reviewer, and reads it back as its author. It posts an inline comment, a
// general comment and a reply, requests changes, and approves.
//
// tea owns authentication, as in TestLive: every request goes through
// `tea api --login`, and this test never reads a token or tea's auth file.
// Use a sandbox pull request, which this test writes to.
//
//	VUORO_LIVE_FORGEJO_WRITE_PR=https://HOST/OWNER/REPO/pulls/N \
//	VUORO_LIVE_FORGEJO_REVIEWER=<tea login> VUORO_LIVE_FORGEJO_AUTHOR=<tea login> \
//	go test -run TestLiveWrite -v ./forge/forgejo
func TestLiveWrite(t *testing.T) {
	raw := os.Getenv("VUORO_LIVE_FORGEJO_WRITE_PR")
	reviewer, author := os.Getenv("VUORO_LIVE_FORGEJO_REVIEWER"), os.Getenv("VUORO_LIVE_FORGEJO_AUTHOR")
	if raw == "" || reviewer == "" || author == "" {
		t.Skip("set VUORO_LIVE_FORGEJO_WRITE_PR, VUORO_LIVE_FORGEJO_REVIEWER and VUORO_LIVE_FORGEJO_AUTHOR to write to a sandbox pull request")
	}
	ref, err := ParseURL(raw)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	as := func(login string) *Client {
		// The secret only marks the client as signed in; tea sends its own.
		c, err := New("https://"+ref.Host, config.SecretForTest("held-by-tea"))
		if err != nil {
			t.Fatal(err)
		}
		c.http.Transport = teaWriter(login)
		return c
	}
	c, other := as(reviewer), as(author)
	store := &memStore{}
	c.KeepDraftsIn(store)
	nonce := time.Now().UTC().Format("20060102T150405Z")

	ch, err := c.Open(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if len(ch.Drafts) != 0 {
		t.Fatalf("%d pending comments already exist; submit or delete them first", len(ch.Drafts))
	}
	anchor := addedLine(t, ch)
	var thread *forge.Thread
	for i := range ch.Threads {
		if ch.Threads[i].Anchor != nil {
			thread = &ch.Threads[i]
			break
		}
	}
	if thread == nil {
		t.Fatal("the pull request needs a thread on a line to reply to")
	}

	mustDraft := func(d forge.NewDraft) forge.Draft {
		t.Helper()
		got, err := c.CreateDraft(ctx, ref, d)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	mustDraft(forge.NewDraft{Body: "vuoro live inline " + nonce, Anchor: anchor})
	general := mustDraft(forge.NewDraft{Body: "vuoro live general " + nonce})
	if err := c.UpdateDraft(ctx, ref, general.ID, "vuoro live general, edited "+nonce); err != nil {
		t.Fatal(err)
	}
	dropped := mustDraft(forge.NewDraft{Body: "vuoro live dropped " + nonce})
	if err := c.DeleteDraft(ctx, ref, dropped.ID); err != nil {
		t.Fatal(err)
	}
	mustDraft(forge.NewDraft{Body: "vuoro live reply " + nonce, ThreadID: thread.ID})
	if mine, err := c.Open(ctx, ref); err != nil || len(mine.Drafts) != 3 {
		t.Fatalf("the reviewer's pending comments: %v %v", mine.Drafts, err)
	}

	// Before Submit, the author sees none of it, in the reviews, their
	// comments, or the issue timeline that shows pending review bodies.
	if leaked := visible(ctx, t, other, ref, nonce); leaked != "" {
		t.Fatalf("a pending comment was visible to the author before Submit: %s", leaked)
	}

	owner, err := other.me(ctx)
	if err != nil {
		t.Fatal(err)
	}
	before := liveTurn(ctx, t, other, ref, owner)
	if err := c.Submit(ctx, ref, forge.VerdictChangesRequested); err != nil {
		t.Fatalf("submit: %v", err)
	}
	// The author's inbox turn, derived the inbox's way from freshly
	// discovered roles and facts, follows the reviewer's verdict.
	asked := liveTurn(ctx, t, other, ref, owner)
	t.Logf("the author's turn before: %s", describe(before))
	t.Logf("the author's turn after changes were requested: %s", describe(asked))
	if !hasRule(asked, "A2") {
		t.Fatalf("requesting changes did not make it the author's turn: %s", describe(asked))
	}
	seen, err := other.Open(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	inline := note(seen, "vuoro live inline "+nonce)
	if inline == nil || inline.Anchor == nil || inline.Anchor.Path != anchor.Path || inline.Anchor.NewLine != anchor.NewLine {
		t.Fatalf("the inline comment is missing or misplaced: %+v", inline)
	}
	if note(seen, "vuoro live general, edited "+nonce) == nil || note(seen, "vuoro live dropped "+nonce) != nil {
		t.Fatal("the general comments did not publish as written")
	}
	if r := note(seen, "vuoro live reply "+nonce); r == nil || r.ID != thread.ID {
		t.Fatalf("the reply is not in thread %s: %+v", thread.ID, r)
	}
	me, err := c.me(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st := stateOf(seen, me); st != forge.RequestedChanges {
		t.Fatalf("the reviewer's state after requesting changes: %q; reviewers %+v; reviews %+v", st, seen.Reviewers, seen.Reviews)
	}
	if ds, _ := store.Drafts(ref); len(ds) != 0 {
		t.Fatalf("pending comments left after Submit: %+v", ds)
	}
	t.Logf("one review published an inline comment on %s line %d, a general comment, and changes requested; the reply joined thread %s", anchor.Path, anchor.NewLine, thread.ID)

	if err := c.Approve(ctx, ref, true); err != nil {
		t.Fatalf("approve: %v", err)
	}
	seen, err = other.Open(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if st := stateOf(seen, me); st != forge.Approved {
		t.Fatalf("the reviewer's state after approving: %q", st)
	}
	approved := liveTurn(ctx, t, other, ref, owner)
	t.Logf("the author's turn after approval: %s", describe(approved))
	if hasRule(approved, "A2") {
		t.Fatalf("approving left the request for changes standing: %s", describe(approved))
	}
	t.Log("approving replaced the request for changes")
}

// teaWriter sends each request through `tea api --login login`, with its
// method and body, and reads back tea's status line and body.
func teaWriter(login string) roundTrip {
	return func(req *http.Request) (*http.Response, error) {
		args := []string{"api", "--login", login, "-i", "-X", req.Method}
		var body []byte
		if req.Body != nil {
			body, _ = io.ReadAll(req.Body)
		}
		if len(body) > 0 {
			args = append(args, "-d", "@-")
		}
		cmd := exec.CommandContext(req.Context(), "tea", append(args, req.URL.String())...)
		cmd.Stdin = bytes.NewReader(body)
		var out, errb bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errb
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("tea could not send %s %s", req.Method, req.URL.Path)
		}
		status := 0
		for _, l := range strings.Split(errb.String(), "\n") {
			if f := strings.Fields(l); len(f) > 1 && strings.HasPrefix(f[0], "HTTP/") {
				status, _ = strconv.Atoi(f[1])
				break
			}
		}
		if status == 0 {
			return nil, fmt.Errorf("tea gave no status for %s %s", req.Method, req.URL.Path)
		}
		return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(&out)}, nil
	}
}

// addedLine is the first line the pull request adds, as an inline anchor.
func addedLine(t *testing.T, ch *forge.Change) *forge.Anchor {
	t.Helper()
	for _, f := range ch.Files {
		for _, l := range strings.Split(f.Patch, "\n") {
			if !strings.HasPrefix(l, "@@ ") {
				continue
			}
			for n := 1; n < 10000; n++ {
				if old, cur, ok := patch.Line(f.Patch, true, n); ok && old == 0 && cur == n {
					return &forge.Anchor{Path: f.NewPath, OldPath: f.OldPath, NewLine: cur, Diff: ch.Diff}
				}
			}
		}
	}
	t.Fatal("the pull request adds no line to comment on")
	return nil
}

// visible names where text containing s shows to c, or is empty.
func visible(ctx context.Context, t *testing.T, c *Client, ref forge.Ref, s string) string {
	t.Helper()
	timeline, err := c.get(ctx, fmt.Sprintf("%s/issues/%d/timeline", repoPath(ref), ref.Number), nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(timeline), s) {
		return "the issue timeline"
	}
	ch, err := c.Open(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if note(ch, s) != nil {
		return "a thread"
	}
	return ""
}

// note is the thread holding a note containing s, or nil.
func note(ch *forge.Change, s string) *forge.Thread {
	for i := range ch.Threads {
		for _, n := range ch.Threads[i].Notes {
			if strings.Contains(n.Body, s) {
				return &ch.Threads[i]
			}
		}
	}
	return nil
}

// liveTurn is login's turn on ref, as the inbox derives it.
func liveTurn(ctx context.Context, t *testing.T, c *Client, ref forge.Ref, login string) turn.Turn {
	t.Helper()
	found, err := c.Discover(ctx, login)
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
		t.Fatalf("discovery did not find %s for %s", ref, login)
	}
	ch, err := c.ReadFacts(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	return turn.Derive(*ch, login, roles)
}

func hasRule(tn turn.Turn, rule string) bool {
	return slices.ContainsFunc(tn.Reasons, func(r turn.Reason) bool { return r.Rule == rule })
}

// describe names the turn's owner and each rule once, with how often it fired.
func describe(tn turn.Turn) string {
	var order []string
	n := map[string]int{}
	for _, r := range tn.Reasons {
		if n[r.Rule] == 0 {
			order = append(order, r.Rule)
		}
		n[r.Rule]++
	}
	var rules []string
	for _, r := range order {
		rules = append(rules, fmt.Sprintf("%s×%d", r, n[r]))
	}
	return fmt.Sprintf("%s [%s]", tn.Owner, strings.Join(rules, " "))
}

// stateOf is login's review state on ch, as the turn engine reads it.
func stateOf(ch *forge.Change, login string) forge.ReviewState {
	for _, r := range ch.Reviewers {
		if strings.EqualFold(r.Username, login) {
			return r.State
		}
	}
	return ""
}
