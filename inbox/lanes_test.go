package inbox

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
)

// The inbox of D38: lanes on the person's turn, one summary line per kind of
// reason, the threads behind them, and quiet changes folded into Dormant.
func TestLanesSummariesAndDormant(t *testing.T) {
	now := time.Now().UTC()
	day := 24 * time.Hour
	a := config.Forge{Name: "lab", Kind: forge.GitLab, URL: "https://lab.test", Username: "me"}
	ref := func(n int) forge.Ref { return forge.Ref{Host: "lab.test", Project: "team/repo", Number: n} }
	note := func(who, body string, at time.Time) forge.Note {
		return forge.Note{Author: forge.User{Username: who}, Body: body, At: at}
	}
	waiting := func(id, who string, line int) forge.Thread {
		return forge.Thread{ID: id, Resolvable: true, Anchor: &forge.Anchor{Path: "a.go", NewLine: line},
			Notes: []forge.Note{note("me", "first", now.Add(-3*day)), note(who, "Why this?\nmore", now.Add(-2*day))}}
	}
	mine := &forge.Change{Ref: ref(1), Kind: forge.GitLab, State: forge.Open, Title: "Mine", Author: forge.User{Username: "me"}, Updated: now,
		Created: now.Add(-5 * day),
		Threads: []forge.Thread{waiting("t1", "alice", 3), waiting("t2", "alice", 9), waiting("t3", "bo", 12)}}
	review := &forge.Change{Ref: ref(2), Kind: forge.GitLab, State: forge.Open, Title: "Review", Author: forge.User{Username: "carol"}, Updated: now,
		Created:   now.Add(-day),
		Reviewers: []forge.Reviewer{{User: forge.User{Username: "me"}, State: forge.Unreviewed}}}
	// Quiet for 200 days, though a bot note moved its updated stamp today.
	old := &forge.Change{Ref: ref(3), Kind: forge.GitLab, State: forge.Open, Title: "Old", Author: forge.User{Username: "dan"}, Updated: now,
		Created:   now.Add(-200 * day),
		Reviewers: []forge.Reviewer{{User: forge.User{Username: "me"}, State: forge.Unreviewed}},
		Threads:   []forge.Thread{{ID: "t9", Notes: []forge.Note{{Author: forge.User{Username: "ci-bot", Bot: true}, Body: "rebased", At: now}}}}}
	p := &fixture{changes: map[string]*forge.Change{}}
	for _, ch := range []*forge.Change{mine, review, old} {
		roles := []forge.Role{forge.ReviewerRole}
		if ch.Author.Username == "me" {
			roles = []forge.Role{forge.AuthorRole}
		}
		p.members = append(p.members, forge.Discovered{Ref: ch.Ref, Roles: roles, Updated: ch.Updated})
		p.changes[ch.Ref.String()] = ch
	}
	s, err := New("", []config.Forge{a}, func(context.Context, config.Forge) (forge.Provider, error) { return p, nil })
	if err != nil {
		t.Fatal(err)
	}
	s.Dormant = 90 * day
	s.Refresh(context.Background())
	d := s.Display(now)

	if len(d.Mine) != 2 || len(d.Dormant) != 1 || d.Dormant[0].Title != "Old" {
		t.Fatalf("mine %d, dormant %+v", len(d.Mine), d.Dormant)
	}
	byTitle := map[string]Row{}
	for _, r := range d.Mine {
		byTitle[r.Title] = r
	}
	m := byTitle["Mine"]
	if m.Lane != LaneReplies || !slices.Equal(m.Summary, []string{"3 threads await your answer: alice (2), bo."}) {
		t.Fatalf("lane %q summary %q", m.Lane, m.Summary)
	}
	if len(m.Waiting) != 3 || m.Waiting[0] != (Waiting{Rule: "A1", Who: "alice", Where: "a.go:3", Text: "Why this?", Since: m.Waiting[0].Since}) {
		t.Fatalf("waiting %+v", m.Waiting)
	}
	if r := byTitle["Review"]; r.Lane != LaneReviews || len(r.Summary) != 1 {
		t.Fatalf("review lane %q summary %q", r.Lane, r.Summary)
	}

	// With the group off, the quiet change is back on the person's turn.
	s.Dormant = 0
	if d := s.Display(now); len(d.Dormant) != 0 || len(d.Mine) != 3 {
		t.Fatalf("no threshold: dormant %d mine %d", len(d.Dormant), len(d.Mine))
	}
	// Snoozed until it changes, a row stays snoozed however far ahead.
	if err := s.Snooze(review.Ref, UntilChange); err != nil {
		t.Fatal(err)
	}
	if d := s.Display(now.Add(400 * day)); len(d.Snoozed) != 1 || d.Snoozed[0].Title != "Review" {
		t.Fatalf("snoozed %+v", d.Snoozed)
	}
}
