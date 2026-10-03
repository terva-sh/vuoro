// Package turn derives whose turn it is from forge facts. It has no I/O and
// holds no local review state. Callers classify configured bots beforehand.
package turn

import (
	"fmt"
	"slices"
	"time"

	"github.com/terva-sh/vuoro/forge"
)

type Owner string

const (
	Mine   Owner = "mine"
	Theirs Owner = "theirs"
)

type Reason struct {
	Rule     string     `json:"rule"`
	Text     string     `json:"text"`
	Since    *time.Time `json:"since"`
	ThreadID string     `json:"thread_id,omitempty"`
}

type Turn struct {
	Owner        Owner      `json:"owner"`
	Since        *time.Time `json:"since"`
	Reasons      []Reason   `json:"reasons"`
	LastActivity *time.Time `json:"last_activity"`
}

// Derive implements plan section 4.1. Unknown event times remain nil; if any
// unanswered reason has an unknown time, the overall age is unknown too.
func Derive(ch forge.Change, me string, roles []forge.Role) Turn {
	out := Turn{Owner: Theirs, LastActivity: activity(ch)}
	add := func(rule, text string, at time.Time, thread string) {
		out.Reasons = append(out.Reasons, Reason{Rule: rule, Text: text, Since: stamp(at), ThreadID: thread})
	}
	if ch.State != forge.Open {
		add("inactive", "This change is "+string(ch.State)+".", time.Time{}, "")
		return out
	}
	reviewer := slices.Contains(roles, forge.ReviewerRole)
	author := slices.Contains(roles, forge.AuthorRole) && ch.Author.Username == me
	meBot := ch.Author.Username == me && ch.Author.Bot
	var current *forge.Reviewer
	for i := range ch.Reviewers {
		if ch.Reviewers[i].Username == me {
			meBot = meBot || ch.Reviewers[i].Bot
			current = &ch.Reviewers[i]
			break
		}
	}
	if me == "" || meBot {
		reviewer = false
		author = false
	}
	last := latestReview(ch.Reviews, me, false)
	if reviewer {
		pending := current != nil && current.State == forge.Unreviewed
		if pending || (last == nil && (current == nil || current.State == "")) {
			at, _ := requestSince(ch.Requests, me, last)
			add("R1", "Your review is requested.", at, "")
		}
		if last != nil {
			pushed := earliestPush(ch.Pushes, last.At)
			hasDatedPush := !pushed.IsZero()
			differentHead := last.Stale || (last.SHA != "" && ch.Diff.Head != "" && last.SHA != ch.Diff.Head)
			if slices.ContainsFunc(ch.Pushes, func(p forge.Push) bool { return p.At.IsZero() }) {
				pushed = time.Time{}
			}
			if hasDatedPush || differentHead {
				add("R2", "New commits arrived since your review.", pushed, "")
			}
			if at, found := requestSince(ch.Requests, me, last); found {
				add("R2b", "Your review was requested again.", at, "")
			}
		}
		for _, thread := range ch.Threads {
			notes := realNotes(thread)
			if len(notes) == 0 || thread.Resolved || notes[len(notes)-1].Author.Username != ch.Author.Username {
				continue
			}
			mine := -1
			for i, n := range notes {
				if n.Author.Username == me {
					mine = i
				}
			}
			if mine < 0 {
				continue
			}
			replies := notes[mine+1:]
			if slices.ContainsFunc(replies, func(n forge.Note) bool { return n.Author.Username == ch.Author.Username }) {
				add("R3", ch.Author.Username+" answered in a thread you are in.", notesSince(replies, ch.Author.Username), thread.ID)
			}
		}
	}
	if author {
		for _, thread := range ch.Threads {
			notes := realNotes(thread)
			if !thread.Resolvable || thread.Resolved || len(notes) == 0 || notes[len(notes)-1].Author.Username == me {
				continue
			}
			start := 0
			for i, n := range notes {
				if n.Author.Username == me {
					start = i + 1
				}
			}
			add("A1", notes[len(notes)-1].Author.Username+" is waiting for your answer in an unresolved thread.", notesSince(notes[start:], ""), thread.ID)
		}
		effective := verdicts(ch)
		approved := false
		blocked := slices.ContainsFunc(ch.Reviewers, func(r forge.Reviewer) bool { return !r.Bot && r.State == forge.RequestedChanges })
		var approvalAt time.Time
		approvalUnknown := false
		for _, r := range effective {
			if r.Verdict == forge.VerdictChangesRequested {
				blocked = true
				text := r.Reviewer + " requested changes; no new commits have answered them."
				if r.At.IsZero() {
					text = r.Reviewer + " currently requests changes; the request’s age is unknown."
				}
				add("A2", text, r.At, "")
			}
			if r.Verdict == forge.VerdictApproved {
				approved = true
				if r.At.IsZero() {
					approvalUnknown = true
				} else if approvalAt.IsZero() || r.At.Before(approvalAt) {
					approvalAt = r.At
				}
			}
		}
		if approved && !blocked && ch.CanMerge != nil && *ch.CanMerge && !ch.Draft && mergeable(ch.Mergeability) {
			if approvalUnknown {
				approvalAt = time.Time{}
			}
			add("A3", "Approved and mergeable; you can merge this change.", approvalAt, "")
		}
	}
	if len(out.Reasons) > 0 {
		out.Owner = Mine
		out.Since = earliestReason(out.Reasons)
		return out
	}
	// Waiting turns use the same unanswered events, so both inbox groups can
	// show an age when the forge supplied one.
	if author {
		for _, r := range ch.Reviewers {
			if r.Bot || r.Username == me {
				continue
			}
			pending := Derive(ch, r.Username, []forge.Role{forge.ReviewerRole})
			if pending.Owner == Mine {
				out.Reasons = append(out.Reasons, Reason{Rule: "waiting", Text: "Waiting for review from " + r.Username + ".", Since: pending.Since})
			}
		}
		if len(out.Reasons) == 0 {
			add("waiting", "Waiting for reviewer activity or merge readiness.", time.Time{}, "")
		}
	} else {
		name := ch.Author.Username
		if name == "" {
			name = "the author"
		}
		for _, r := range verdicts(ch) {
			if r.Reviewer == me && r.Verdict == forge.VerdictChangesRequested {
				add("waiting", name+" is answering your requested changes.", r.At, "")
			}
		}
		for _, thread := range ch.Threads {
			notes := realNotes(thread)
			if !thread.Resolvable || thread.Resolved || len(notes) == 0 || notes[len(notes)-1].Author.Username == ch.Author.Username {
				continue
			}
			mine := slices.ContainsFunc(notes, func(n forge.Note) bool { return n.Author.Username == me })
			if !mine {
				continue
			}
			start := 0
			for i, n := range notes {
				if n.Author.Username == ch.Author.Username {
					start = i + 1
				}
			}
			add("waiting", name+" is answering an unresolved thread you are in.", notesSince(notes[start:], ""), thread.ID)
		}
		if len(out.Reasons) == 0 {
			add("waiting", fmt.Sprintf("Waiting for %s; no unanswered review action is recorded for you.", name), time.Time{}, "")
		}
	}
	out.Since = earliestReason(out.Reasons)
	return out
}

func stamp(at time.Time) *time.Time {
	if at.IsZero() {
		return nil
	}
	at = at.UTC()
	return &at
}
func mergeable(s string) bool { return s == "mergeable" || s == "clean" }
func earliestReason(rs []Reason) *time.Time {
	var at *time.Time
	for _, r := range rs {
		if r.Since == nil {
			return nil
		}
		if at == nil || r.Since.Before(*at) {
			at = stamp(*r.Since)
		}
	}
	return at
}
func requestSince(rs []forge.ReviewRequest, me string, last *forge.Review) (time.Time, bool) {
	var at time.Time
	unknown := false
	for _, r := range rs {
		if r.Reviewer != me {
			continue
		}
		if r.At.IsZero() {
			unknown = true
			continue
		}
		if last != nil && (last.At.IsZero() || !r.At.After(last.At)) {
			continue
		}
		if at.IsZero() || r.At.Before(at) {
			at = r.At
		}
	}
	found := !at.IsZero()
	if unknown {
		return time.Time{}, found
	}
	return at, found
}
func earliestPush(ps []forge.Push, after time.Time) time.Time {
	if after.IsZero() {
		return time.Time{}
	}
	var at time.Time
	for _, p := range ps {
		if p.At.After(after) && (at.IsZero() || p.At.Before(at)) {
			at = p.At
		}
	}
	return at
}

// reviewHistory orders fully dated facts and otherwise retains the adapter's
// chronological order. Filter bots/comments before deciding the ordering so
// excluded noise cannot change which human verdict is latest.
func reviewHistory(rs []forge.Review, me string, verdictOnly bool) []forge.Review {
	history := []forge.Review{}
	dated := true
	for _, r := range rs {
		if r.Reviewer != me || r.Bot || (verdictOnly && r.Verdict == forge.VerdictCommented) {
			continue
		}
		history = append(history, r)
		dated = dated && !r.At.IsZero()
	}
	if dated {
		slices.SortStableFunc(history, func(a, b forge.Review) int { return a.At.Compare(b.At) })
	}
	return history
}
func latestReview(rs []forge.Review, me string, verdictOnly bool) *forge.Review {
	history := reviewHistory(rs, me, verdictOnly)
	for i := len(history) - 1; i >= 0; i-- {
		if !history[i].Dismissed {
			r := history[i]
			return &r
		}
	}
	return nil
}

// verdicts keeps each human reviewer's last non-comment verdict. Dismissed and
// stale verdicts supersede earlier ones but cannot count as current approval or
// an open change request. SHA/time evidence excludes reviews answered by pushes.
func verdicts(ch forge.Change) []forge.Review {
	latest := map[string]forge.Review{}
	for _, r := range ch.Reviews {
		if r.Bot || r.Verdict == forge.VerdictCommented {
			continue
		}
		if _, seen := latest[r.Reviewer]; seen {
			continue
		}
		history := reviewHistory(ch.Reviews, r.Reviewer, true)
		latest[r.Reviewer] = history[len(history)-1]
	}
	// A stated current review state is authoritative on every forge. It
	// supplements missing/invalid verdict history without inventing its time.
	for _, r := range ch.Reviewers {
		if r.Bot {
			delete(latest, r.Username)
			continue
		}
		verdict := map[forge.ReviewState]forge.Verdict{forge.Approved: forge.VerdictApproved, forge.RequestedChanges: forge.VerdictChangesRequested}[r.State]
		switch r.State {
		case forge.Unreviewed, forge.Reviewed:
			delete(latest, r.Username)
		case forge.Approved, forge.RequestedChanges:
			if old, ok := latest[r.Username]; !ok || old.Verdict != verdict || old.Dismissed || old.Stale || (old.SHA != "" && ch.Diff.Head != "" && old.SHA != ch.Diff.Head) {
				latest[r.Username] = forge.Review{Reviewer: r.Username, Verdict: verdict}
			}
		}
	}

	names := make([]string, 0, len(latest))
	for name := range latest {
		names = append(names, name)
	}
	slices.Sort(names)
	out := []forge.Review{}
	for _, name := range names {
		r := latest[name]
		if r.Dismissed || r.Stale || (r.SHA != "" && ch.Diff.Head != "" && r.SHA != ch.Diff.Head) {
			continue
		}
		if r.Verdict == forge.VerdictChangesRequested && !earliestPush(ch.Pushes, r.At).IsZero() {
			continue
		}
		// An explicit current change request still demands an author response
		// when its timestamp is missing. Keep the action with an unknown age.
		r.At = verdictSince(ch, r)
		out = append(out, r)
	}
	return out
}

// Repeating a verdict does not reset an unanswered run. An opposing verdict,
// dismissal/staleness, or (for a change request) an author push ends it. Unknown
// times within the run remain unknown even when the latest repeat is dated.
func verdictSince(ch forge.Change, current forge.Review) time.Time {
	if current.At.IsZero() {
		return time.Time{}
	}
	history := reviewHistory(ch.Reviews, current.Reviewer, true)
	if len(history) == 0 || history[len(history)-1].Verdict != current.Verdict {
		return current.At
	}
	at := current.At
	for i := len(history) - 1; i >= 0; i-- {
		old := history[i]
		if old.Verdict != current.Verdict || old.Dismissed || old.Stale || (old.SHA != "" && ch.Diff.Head != "" && old.SHA != ch.Diff.Head) {
			break
		}
		if old.At.IsZero() {
			return time.Time{}
		}
		if old.Verdict == forge.VerdictChangesRequested && !earliestPush(ch.Pushes, old.At).IsZero() {
			break
		}
		at = old.At
	}
	return at
}

// Only unanswered human notes are passed here. For R3, count the author's
// replies alone. Missing timestamps anywhere in that run make its age unknown.
func notesSince(notes []forge.Note, username string) time.Time {
	var at time.Time
	for _, n := range notes {
		if username != "" && n.Author.Username != username {
			continue
		}
		if n.At.IsZero() {
			return time.Time{}
		}
		if at.IsZero() || n.At.Before(at) {
			at = n.At
		}
	}
	return at
}

func realNotes(t forge.Thread) []forge.Note {
	out := []forge.Note{}
	allDated := true
	for _, n := range t.Notes {
		if n.System || n.Author.Bot || n.Author.Username == "" {
			continue
		}
		out = append(out, n)
		allDated = allDated && !n.At.IsZero()
	}
	// Without all timestamps retain the adapter's chronological order, without
	// manufacturing a relative order for an undated event.
	if allDated {
		slices.SortStableFunc(out, func(a, b forge.Note) int { return a.At.Compare(b.At) })
	}
	return out
}
func activity(ch forge.Change) *time.Time {
	at := ch.Updated
	update := func(v time.Time) {
		if v.After(at) {
			at = v
		}
	}
	for _, p := range ch.Pushes {
		update(p.At)
	}
	for _, r := range ch.Requests {
		update(r.At)
	}
	for _, r := range ch.Reviews {
		update(r.At)
	}
	for _, t := range ch.Threads {
		for _, n := range t.Notes {
			update(n.At)
		}
	}
	return stamp(at)
}
