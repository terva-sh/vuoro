package gitlab

import (
	"context"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/terva-sh/vuoro/forge"
)

// TestLiveAdminister round-trips each administrative write on a real merge
// request the person may write to, and puts every field back as it found it.
// It runs only when asked, on a merge request named for it:
//
//	VUORO_LIVE_GITLAB_ADMIN_MR=https://gitlab.com/group/project/-/merge_requests/1 \
//	  go test -run TestLiveAdminister -v ./forge/gitlab
//
// The token comes from vuoro's own config, as TestLiveWrite's does. Adding the
// label vuoro-live creates it in the project if it is missing.
func TestLiveAdminister(t *testing.T) {
	raw := os.Getenv("VUORO_LIVE_GITLAB_ADMIN_MR")
	if raw == "" {
		t.Skip("set VUORO_LIVE_GITLAB_ADMIN_MR to a merge request you may write to")
	}
	ref, err := ParseURL(raw)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	c, me := liveWriter(ctx, t, ref.Host)
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
	isReviewer := func(ch *forge.Change) bool {
		return slices.ContainsFunc(ch.Reviewers, func(r forge.Reviewer) bool { return r.Username == me })
	}
	isAssignee := func(ch *forge.Change) bool {
		return slices.ContainsFunc(ch.Assignees, func(u forge.User) bool { return u.Username == me })
	}
	before := read()

	// Each field goes the other way and back, so the merge request ends as
	// it began.
	if isReviewer(before) {
		apply("remove reviewer", forge.Changes{RemoveReviewers: []string{me}})
		if isReviewer(read()) {
			t.Fatal("still a reviewer after removal")
		}
		apply("re-add reviewer", forge.Changes{AddReviewers: []string{me}})
	} else {
		apply("add reviewer", forge.Changes{AddReviewers: []string{me}})
		if !isReviewer(read()) {
			t.Fatal("not a reviewer after adding")
		}
		apply("remove reviewer", forge.Changes{RemoveReviewers: []string{me}})
	}
	if isReviewer(read()) != isReviewer(before) {
		t.Fatal("reviewers did not round-trip")
	}
	if isAssignee(before) {
		apply("unassign", forge.Changes{RemoveAssignees: []string{me}})
		apply("reassign", forge.Changes{AddAssignees: []string{me}})
	} else {
		apply("assign", forge.Changes{AddAssignees: []string{me}})
		if !isAssignee(read()) {
			t.Fatal("not assigned after assigning")
		}
		apply("unassign", forge.Changes{RemoveAssignees: []string{me}})
	}
	if isAssignee(read()) != isAssignee(before) {
		t.Fatal("assignees did not round-trip")
	}
	had := slices.Contains(before.Labels, "vuoro-live")
	apply("label", forge.Changes{AddLabels: []string{"vuoro-live"}})
	if !slices.Contains(read().Labels, "vuoro-live") {
		t.Fatal("label not added")
	}
	if !had {
		apply("unlabel", forge.Changes{RemoveLabels: []string{"vuoro-live"}})
	}
	flip := !before.Draft
	apply("draft", forge.Changes{Draft: &flip})
	if read().Draft != flip {
		t.Fatal("draft state did not change")
	}
	apply("draft back", forge.Changes{Draft: &before.Draft})

	after := read()
	if after.Draft != before.Draft || !slices.Equal(after.Labels, before.Labels) {
		t.Fatalf("the merge request did not end as it began: draft %v→%v, labels %v→%v", before.Draft, after.Draft, before.Labels, after.Labels)
	}
	// GitLab reports a username it cannot find in the mutation's errors.
	if err := c.Administer(ctx, ref, forge.Changes{AddReviewers: []string{"vuoro-no-such-user-" + time.Now().Format("150405")}}); err == nil {
		t.Fatal("an unknown reviewer was accepted")
	}
	members, err := c.Members(ctx, ref.Project, me)
	if err != nil || !slices.ContainsFunc(members, func(m forge.Member) bool { return m.Username == me }) {
		t.Fatalf("members %v %v", len(members), err)
	}
	if _, err := c.Labels(ctx, ref.Project); err != nil {
		t.Fatal(err)
	}
}
