package projects

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/terva-sh/vuoro/forge"
)

var t0 = time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)

func at(h int) time.Time { return t0.Add(time.Duration(h) * time.Hour) }

// change is an open, ready merge request by ann in team/repo, opened at t0
// and pushed once then.
func change(n int, edit ...func(*forge.Change)) forge.Change {
	ch := forge.Change{Kind: forge.GitLab, Ref: forge.Ref{Host: "lab.test", Project: "team/repo", Number: n}, State: forge.Open,
		Title: "change", Author: forge.User{Username: "ann"}, Created: t0, SourceProject: "team/repo",
		SourceBranch: "feature-" + string(rune('a'+n)), TargetBranch: "main", Pushes: []forge.Push{{At: t0}}}
	for _, e := range edit {
		e(&ch)
	}
	return ch
}

func requested(who string, h int) func(*forge.Change) {
	return func(ch *forge.Change) {
		ch.Reviewers = append(ch.Reviewers, forge.Reviewer{User: forge.User{Username: who}, State: forge.Unreviewed})
		ch.Requests = append(ch.Requests, forge.ReviewRequest{Reviewer: who, At: at(h)})
	}
}

func verdict(who string, v forge.Verdict, h int) func(*forge.Change) {
	return func(ch *forge.Change) {
		state := map[forge.Verdict]forge.ReviewState{forge.VerdictApproved: forge.Approved, forge.VerdictChangesRequested: forge.RequestedChanges}[v]
		for i := range ch.Reviewers {
			if ch.Reviewers[i].Username == who {
				ch.Reviewers[i].State = state
			}
		}
		ch.Reviews = append(ch.Reviews, forge.Review{Reviewer: who, Verdict: v, At: at(h)})
	}
}

func thread(notes ...forge.Note) func(*forge.Change) {
	return func(ch *forge.Change) {
		ch.Threads = append(ch.Threads, forge.Thread{ID: "t", Resolvable: true, Notes: notes})
	}
}

func note(who string, h int) forge.Note {
	return forge.Note{Author: forge.User{Username: who}, At: at(h), Body: "x"}
}

func sit(ch forge.Change) Sitting { return Sit(ch, below{}) }

func TestEachPlace(t *testing.T) {
	for _, c := range []struct {
		name  string
		ch    forge.Change
		place Place
		since *time.Time
	}{
		{"draft", change(1, func(ch *forge.Change) { ch.Draft = true }), Draft, nil},
		{"no reviewers", change(1), NoReviewers, nil},
		{"waiting on review", change(1, requested("bob", 2)), WaitingReview, ptr(at(2))},
		{"changes requested", change(1, requested("bob", 2), verdict("bob", forge.VerdictChangesRequested, 5)), WaitingAuthor, ptr(at(5))},
		{"thread waits on author", change(1, requested("bob", 2), verdict("bob", forge.VerdictApproved, 4), thread(note("bob", 3))), WaitingAuthor, ptr(at(3))},
		{"approved", change(1, requested("bob", 2), verdict("bob", forge.VerdictApproved, 6)), Approved, ptr(at(6))},
		{"pipeline failed", change(1, requested("bob", 2), func(ch *forge.Change) { ch.Pipeline = "failed" }), Blocked, nil},
		{"conflicts", change(1, requested("bob", 2), func(ch *forge.Change) { ch.Conflicts = true }), Blocked, nil},
	} {
		got := sit(c.ch)
		if got.Place != c.place {
			t.Errorf("%s: sits in %s (%+v)", c.name, got.Place, got.Reasons)
			continue
		}
		if (got.Since == nil) != (c.since == nil) || (got.Since != nil && !got.Since.Equal(*c.since)) {
			t.Errorf("%s: since %v, want %v", c.name, got.Since, c.since)
		}
	}
}

func ptr(t time.Time) *time.Time { return &t }

func TestPrecedence(t *testing.T) {
	// Each pair holds both places, and sits in the earlier one, keeping the
	// later one as a reason.
	draftBlocked := change(1, func(ch *forge.Change) { ch.Draft = true; ch.Pipeline = "failed" })
	blockedAuthor := change(1, requested("bob", 2), verdict("bob", forge.VerdictChangesRequested, 3), func(ch *forge.Change) { ch.Conflicts = true })
	authorReview := change(1, requested("bob", 2), requested("cat", 2), verdict("bob", forge.VerdictChangesRequested, 3))
	reviewApproved := change(1, requested("bob", 2), requested("cat", 2), verdict("bob", forge.VerdictApproved, 3))
	draftNoReviewers := change(1, func(ch *forge.Change) { ch.Draft = true })
	for _, c := range []struct {
		ch           forge.Change
		place, other Place
	}{
		{draftBlocked, Draft, Blocked},
		{blockedAuthor, Blocked, WaitingAuthor},
		{authorReview, WaitingAuthor, WaitingReview},
		{reviewApproved, WaitingReview, Approved},
		{draftNoReviewers, Draft, NoReviewers},
	} {
		got := sit(c.ch)
		if got.Place != c.place || !slices.ContainsFunc(got.Reasons, func(r Reason) bool { return r.Place == c.other }) {
			t.Errorf("%s over %s: sits in %s with %+v", c.place, c.other, got.Place, got.Reasons)
		}
	}
}

func TestAChangesRequestAnsweredByAPushGoesBackToReview(t *testing.T) {
	ch := change(1, requested("bob", 2), verdict("bob", forge.VerdictChangesRequested, 3), func(ch *forge.Change) {
		ch.Pushes = append(ch.Pushes, forge.Push{At: at(8)})
		ch.Reviewers[0].State = forge.Reviewed
	})
	got := sit(ch)
	if got.Place != WaitingReview || got.Since == nil || !got.Since.Equal(at(8)) || got.Holder[0] != "bob" {
		t.Fatalf("after the push: %+v", got)
	}
}

func TestAnUnknownTimeStaysUnknown(t *testing.T) {
	ch := change(1, requested("bob", 2), requested("cat", 3))
	ch.Requests[1].At = time.Time{}
	if got := sit(ch); got.Place != WaitingReview || got.Since != nil {
		t.Fatalf("an undated request gave an age: %+v", got)
	}
	ch = change(1, requested("bob", 2), verdict("bob", forge.VerdictApproved, 4))
	ch.Reviews[0].At = time.Time{}
	if got := sit(ch); got.Place != Approved || got.Since != nil {
		t.Fatalf("an undated approval gave an age: %+v", got)
	}
	// Creation is not when a change started waiting for a reviewer.
	if got := sit(change(1)); got.Since != nil {
		t.Fatal("no reviewers borrowed the creation time")
	}
}

func TestBotsCountForNothing(t *testing.T) {
	duo := func(ch *forge.Change) {
		ch.Reviewers = append(ch.Reviewers, forge.Reviewer{User: forge.User{Username: "duo", Bot: true}, State: forge.Unreviewed})
		ch.Threads = append(ch.Threads, forge.Thread{ID: "b", Resolvable: true, Notes: []forge.Note{{Author: forge.User{Username: "duo", Bot: true}, At: at(1)}}})
		ch.Threads = append(ch.Threads, forge.Thread{ID: "s", Resolvable: true, Notes: []forge.Note{{Author: forge.User{Username: "bob"}, At: at(1), System: true}}})
	}
	got := sit(change(1, duo))
	if got.Place != NoReviewers || len(got.Reasons) != 1 {
		t.Fatalf("a bot reviewer or note moved the change: %+v", got)
	}
	if th := threads(change(1, duo)); th.Total != 0 {
		t.Fatalf("bot or system threads counted: %+v", th)
	}
}

func TestReviewedWithNothingOutstandingWaitsOnReviewers(t *testing.T) {
	ch := change(1, requested("bob", 2), func(ch *forge.Change) { ch.Reviewers[0].State = forge.Reviewed })
	ch.Reviews = append(ch.Reviews, forge.Review{Reviewer: "bob", Verdict: forge.VerdictCommented, At: at(3)})
	got := sit(ch)
	if got.Place != WaitingReview || !slices.Equal(got.Holder, []string{"bob"}) {
		t.Fatalf("%+v", got)
	}
}

func TestStacksFromTargetBranches(t *testing.T) {
	bottom := change(1, requested("bob", 2))
	middle := change(2, requested("bob", 2), func(ch *forge.Change) { ch.TargetBranch = bottom.SourceBranch })
	top := change(3, requested("bob", 2), func(ch *forge.Change) { ch.TargetBranch = middle.SourceBranch })
	fork := change(4, func(ch *forge.Change) { ch.SourceProject = "someone/repo"; ch.SourceBranch = "main" })
	belows, st := stacks("team/repo", []forge.Change{bottom, middle, top, fork})
	if st[1] == nil || st[1].Depth != 0 || !slices.Equal(st[1].Above, []int{2}) {
		t.Fatalf("bottom %+v", st[1])
	}
	if st[3] == nil || st[3].Depth != 2 || st[3].Root != 1 || !slices.Equal(st[3].On, []int{2}) {
		t.Fatalf("top %+v", st[3])
	}
	if st[4] != nil || len(belows[1].On) != 0 {
		t.Fatal("a fork's branch named main stacked the project's changes on it")
	}
	got := Sit(top, belows[3])
	if got.Place != Blocked || len(got.Holder) != 0 || !strings.Contains(got.Reasons[0].Text, "!2") {
		t.Fatalf("the top of a stack: %+v", got)
	}
}

func TestStacksFromDependencies(t *testing.T) {
	a := change(1)
	b := change(2, func(ch *forge.Change) {
		ch.DependenciesKnown = true
		ch.BlockedBy = []forge.Ref{{Host: "lab.test", Project: "team/repo", Number: 1}, {Host: "lab.test", Project: "team/repo", Number: 9}, {Host: "lab.test"}}
	})
	belows, st := stacks("team/repo", []forge.Change{a, b})
	if !slices.Equal(belows[2].On, []int{1}) || belows[2].Hidden != 1 || st[2].Root != 1 {
		t.Fatalf("dependencies %+v %+v", belows[2], st[2])
	}
	got := Sit(b, belows[2])
	if got.Place != Blocked || len(got.Holder) != 1 {
		t.Fatalf("a hidden blocker is not the stack beneath: %+v", got)
	}
}

func TestACycleIsDroppedNotLooped(t *testing.T) {
	a := change(1, func(ch *forge.Change) { ch.SourceBranch, ch.TargetBranch = "x", "y" })
	b := change(2, func(ch *forge.Change) { ch.SourceBranch, ch.TargetBranch = "y", "x" })
	c := change(3, func(ch *forge.Change) { ch.TargetBranch = "x" })
	belows, st := stacks("team/repo", []forge.Change{a, b, c})
	if len(belows[1].On) != 0 || len(belows[2].On) != 0 {
		t.Fatalf("a cycle kept its links: %+v %+v", belows[1], belows[2])
	}
	if !slices.Equal(belows[3].On, []int{1}) || st[3].Depth != 1 {
		t.Fatalf("a change on a cycle lost its own link: %+v", st[3])
	}
}

func TestDisplayGroupsOldestFirst(t *testing.T) {
	snap := Snapshot{Project: Entry{Path: "team/repo"}, Items: []Item{
		{Change: change(1, requested("bob", 5))},
		{Change: change(2, requested("bob", 1))},
		{Change: change(3)},
		{Change: change(4, func(ch *forge.Change) { ch.Draft = true })},
		{Change: forge.Change{Ref: forge.Ref{Number: 5}}, Unread: true, Error: "x"},
	}}
	d := snap.Display()
	if d.Open != 5 || len(d.Unread) != 1 || len(d.Groups) != 3 {
		t.Fatalf("%+v", d)
	}
	if d.Groups[0].Place != Draft || d.Groups[1].Place != WaitingReview || d.Groups[2].Place != NoReviewers {
		t.Fatalf("group order %s %s %s", d.Groups[0].Place, d.Groups[1].Place, d.Groups[2].Place)
	}
	if d.Groups[1].Rows[0].Ref.Number != 2 {
		t.Fatal("the longest wait was not first")
	}
}
