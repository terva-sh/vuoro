package projects

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/turn"
)

// Place is where an open change sits (docs/plan.md section 6.9).
type Place string

const (
	Draft         Place = "draft"
	Blocked       Place = "blocked"
	WaitingAuthor Place = "author"
	WaitingReview Place = "review"
	NoReviewers   Place = "unreviewed"
	Approved      Place = "approved"
)

// Places is the order of precedence: a change sits in the first that holds.
var Places = []Place{Draft, Blocked, WaitingAuthor, WaitingReview, NoReviewers, Approved}

// Reason is one fact that puts a change in a place, and when it began.
type Reason struct {
	Place Place      `json:"place"`
	Text  string     `json:"text"`
	Since *time.Time `json:"since"`
	// Who is the person the reason is about, when there is one.
	Who string `json:"who,omitempty"`
}

// Sitting is where a change sits, since when, and why. Since is nil when the
// event that put it there has no recorded time.
type Sitting struct {
	Place   Place      `json:"place"`
	Since   *time.Time `json:"since"`
	Reasons []Reason   `json:"reasons"`
	// Holder is who the change waits on: the author, the reviewers who
	// have not answered, or nobody.
	Holder []string `json:"holder"`
}

// below is what a change is stacked on, worked out across the project.
type below struct {
	// On are the open changes beneath this one in the project.
	On []int
	// Hidden counts blockers the person cannot see, or in other projects.
	Hidden int
}

// Sit derives where ch sits. ch must be classified, so bots count for
// nothing, and open. stack is what it is stacked on.
func Sit(ch forge.Change, stack below) Sitting {
	var reasons []Reason
	add := func(p Place, text string, since *time.Time, who string) {
		reasons = append(reasons, Reason{Place: p, Text: text, Since: since, Who: who})
	}
	author := ch.Author.Username
	if author == "" {
		author = "the author"
	}
	reviewers := humanReviewers(ch)

	if ch.Draft {
		add(Draft, "Marked draft by its author.", nil, ch.Author.Username)
	}

	switch ch.Pipeline {
	case "failed":
		add(Blocked, "The pipeline failed.", nil, "")
	case "canceled", "cancelled":
		add(Blocked, "The pipeline was cancelled.", nil, "")
	}
	if ch.Conflicts {
		add(Blocked, "It conflicts with its target branch.", nil, "")
	}
	for _, n := range stack.On {
		add(Blocked, fmt.Sprintf("Stacked on %s%d, which is still open.", mark(ch.Kind), n), nil, "")
	}
	if stack.Hidden == 1 {
		add(Blocked, "Waits on a change that must merge first.", nil, "")
	} else if stack.Hidden > 1 {
		add(Blocked, fmt.Sprintf("Waits on %d changes that must merge first.", stack.Hidden), nil, "")
	}

	// The author's turn, as the inbox derives it for the author: unresolved
	// threads ending with a reviewer's note, and changes requested that no
	// push has answered. Merge readiness (A3) is the signed-in person's
	// permission, not the author's, so it plays no part here.
	if ch.Author.Username != "" && !ch.Author.Bot {
		t := turn.Derive(ch, ch.Author.Username, []forge.Role{forge.AuthorRole})
		if t.Owner == turn.Mine {
			threads, threadSince, threadWho := 0, (*time.Time)(nil), ""
			threadUnknown := false
			for _, r := range t.Reasons {
				switch r.Rule {
				case "A1":
					threads++
					threadWho = r.Who
					if r.Since == nil {
						threadUnknown = true
					} else if threadSince == nil || r.Since.Before(*threadSince) {
						threadSince = r.Since
					}
				case "A2":
					add(WaitingAuthor, r.Who+" requested changes; no push has answered them.", r.Since, r.Who)
				}
			}
			if threadUnknown {
				threadSince = nil
			}
			if threads == 1 {
				add(WaitingAuthor, fmt.Sprintf("An unresolved thread waits on %s's answer to %s.", author, threadWho), threadSince, threadWho)
			} else if threads > 1 {
				add(WaitingAuthor, fmt.Sprintf("%d unresolved threads wait on %s's answer.", threads, author), threadSince, "")
			}
		}
	}

	// Each reviewer's turn, as the inbox derives it for that reviewer.
	waiting := []string{}
	for _, r := range reviewers {
		t := turn.Derive(ch, r.Username, []forge.Role{forge.ReviewerRole})
		if t.Owner != turn.Mine {
			continue
		}
		waiting = append(waiting, r.Username)
		text := r.Username + "'s review is requested."
		for _, why := range t.Reasons {
			switch why.Rule {
			case "R2":
				text = "New commits arrived since " + r.Username + " reviewed."
			case "R2b":
				text = r.Username + "'s review was requested again."
			case "R3":
				text = author + " answered " + r.Username + " in a thread."
			}
		}
		add(WaitingReview, text, t.Since, r.Username)
	}

	approvedBy, approvedAt := approvals(ch, reviewers)
	changesRequested := slices.ContainsFunc(reviewers, func(r forge.Reviewer) bool { return r.State == forge.RequestedChanges })
	if len(reviewers) == 0 {
		add(NoReviewers, "No reviewer is assigned.", nil, "")
	} else if len(approvedBy) > 0 && !changesRequested {
		add(Approved, "Approved by "+strings.Join(approvedBy, ", ")+"; not merged.", approvedAt, "")
	}

	out := Sitting{Reasons: reasons, Holder: []string{}}
	for _, p := range Places {
		if slices.ContainsFunc(reasons, func(r Reason) bool { return r.Place == p }) {
			out.Place = p
			break
		}
	}
	if out.Place == "" {
		// Reviewers looked and left nothing outstanding, and nobody has
		// approved: the next move is still theirs.
		out.Place = WaitingReview
		out.Reasons = append(out.Reasons, Reason{Place: WaitingReview, Text: "Reviewed, with nothing outstanding, and not approved."})
	}
	out.Since = earliest(out.Reasons, out.Place)
	switch out.Place {
	case Draft, WaitingAuthor:
		out.Holder = []string{ch.Author.Username}
	case WaitingReview:
		out.Holder = waiting
		if len(waiting) == 0 {
			for _, r := range reviewers {
				out.Holder = append(out.Holder, r.Username)
			}
		}
	case Blocked:
		if !slices.ContainsFunc(out.Reasons, func(r Reason) bool { return r.Place == Blocked && !strings.HasPrefix(r.Text, "Stacked on") }) {
			// Blocked only by the change beneath it: reviewing that one
			// is what unblocks this one.
			out.Holder = []string{}
		} else {
			out.Holder = []string{ch.Author.Username}
		}
	}
	return out
}

// mark is how the forge writes a change's number.
func mark(kind string) string {
	if kind == forge.GitHub || kind == forge.Forgejo {
		return "#"
	}
	return "!"
}

// humanReviewers are the requested reviewers who are people, other than the
// author.
func humanReviewers(ch forge.Change) []forge.Reviewer {
	var out []forge.Reviewer
	for _, r := range ch.Reviewers {
		if !r.Bot && r.Username != "" && r.Username != ch.Author.Username {
			out = append(out, r)
		}
	}
	return out
}

// approvals are the reviewers who currently approve, and when the earliest
// of those approvals was given. The time is nil when any is undated.
func approvals(ch forge.Change, reviewers []forge.Reviewer) ([]string, *time.Time) {
	var who []string
	var at time.Time
	unknown := false
	for _, r := range reviewers {
		if r.State != forge.Approved {
			continue
		}
		who = append(who, r.Username)
		var given time.Time
		for _, v := range ch.Reviews {
			if v.Reviewer == r.Username && v.Verdict == forge.VerdictApproved && !v.Dismissed && !v.Bot {
				given = v.At
			}
		}
		if given.IsZero() {
			unknown = true
		} else if at.IsZero() || given.Before(at) {
			at = given
		}
	}
	if unknown || at.IsZero() {
		return who, nil
	}
	return who, &at
}

// earliest is the oldest reason for place, or nil when any is undated.
func earliest(reasons []Reason, place Place) *time.Time {
	var out *time.Time
	for _, r := range reasons {
		if r.Place != place {
			continue
		}
		if r.Since == nil {
			return nil
		}
		if out == nil || r.Since.Before(*out) {
			out = r.Since
		}
	}
	return out
}
