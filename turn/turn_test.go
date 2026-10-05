package turn

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/terva-sh/vuoro/forge"
)

func recorded(t *testing.T, id string) forge.Change {
	t.Helper()
	b, err := os.ReadFile("testdata/mr" + id + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var ch forge.Change
	if err = json.Unmarshal(b, &ch); err != nil {
		t.Fatal(err)
	}
	return ch
}
func moment(n int) time.Time { return time.Date(2026, 10, 2, n, 0, 0, 0, time.UTC) }
func base(t *testing.T) forge.Change {
	ch := recorded(t, "3991")
	ch.Reviewers = nil
	ch.Reviews = nil
	ch.Requests = nil
	ch.Pushes = nil
	ch.Threads = nil
	ch.Author = forge.User{Username: "author"}
	ch.Diff.Head = "head"
	ch.State = forge.Open
	ch.Kind = forge.GitLab
	ch.CanMerge = nil
	return ch
}
func note(name string, n int) forge.Note {
	return forge.Note{Author: forge.User{Username: name}, At: moment(n)}
}
func reviewed(ch *forge.Change) {
	ch.Reviewers = []forge.Reviewer{{User: forge.User{Username: "me"}, State: forge.Reviewed}}
	ch.Reviews = []forge.Review{{Reviewer: "me", Verdict: forge.VerdictCommented, At: moment(1), SHA: "head"}}
}
func yes() *bool { b := true; return &b }

func TestEveryRuleFromRecordedModel(t *testing.T) {
	cases := []struct {
		name, me, rule string
		role           forge.Role
		at             int
		setup          func(*forge.Change)
	}{
		{"R1 unreviewed", "me", "R1", forge.ReviewerRole, 1, func(ch *forge.Change) {
			ch.Reviewers = []forge.Reviewer{{User: forge.User{Username: "me"}, State: forge.Unreviewed}}
			ch.Requests = []forge.ReviewRequest{{Reviewer: "me", At: moment(1)}}
		}},
		{"R2 pushes", "me", "R2", forge.ReviewerRole, 2, func(ch *forge.Change) { reviewed(ch); ch.Pushes = []forge.Push{{At: moment(3)}, {At: moment(2)}} }},
		{"R2b rerequest", "me", "R2b", forge.ReviewerRole, 2, func(ch *forge.Change) {
			reviewed(ch)
			ch.Requests = []forge.ReviewRequest{{Reviewer: "me", At: moment(3)}, {Reviewer: "me", At: moment(2)}}
		}},
		{"R3 author reply", "me", "R3", forge.ReviewerRole, 2, func(ch *forge.Change) {
			reviewed(ch)
			ch.Threads = []forge.Thread{{ID: "reply", Notes: []forge.Note{note("me", 1), note("author", 2), note("author", 3)}}}
		}},
		{"A1 unresolved reviewer note", "author", "A1", forge.AuthorRole, 2, func(ch *forge.Change) {
			ch.Threads = []forge.Thread{{ID: "unresolved", Resolvable: true, Notes: []forge.Note{note("author", 1), note("me", 2), note("other", 3)}}}
		}},
		{"A2 changes requested", "author", "A2", forge.AuthorRole, 2, func(ch *forge.Change) {
			ch.Reviewers = []forge.Reviewer{{User: forge.User{Username: "me"}, State: forge.RequestedChanges}}
			ch.Reviews = []forge.Review{{Reviewer: "me", Verdict: forge.VerdictChangesRequested, At: moment(2)}}
		}},
		{"A3 approved mergeable permitted", "author", "A3", forge.AuthorRole, 2, func(ch *forge.Change) {
			ch.Mergeability = "mergeable"
			ch.CanMerge = yes()
			ch.Reviewers = []forge.Reviewer{{User: forge.User{Username: "me"}, State: forge.Approved}}
			ch.Reviews = []forge.Review{{Reviewer: "me", Verdict: forge.VerdictApproved, At: moment(2)}}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ch := base(t)
			tc.setup(&ch)
			got := Derive(ch, tc.me, []forge.Role{tc.role})
			if got.Owner != Mine || len(got.Reasons) != 1 || got.Reasons[0].Rule != tc.rule || got.Since == nil || !got.Since.Equal(moment(tc.at)) {
				t.Fatalf("turn: %+v", got)
			}
			if got.Reasons[0].Text == "" {
				t.Fatal("empty reason")
			}
		})
	}
}

func TestRecordedGitLabTurns(t *testing.T) {
	ch := recorded(t, "3991")
	got := Derive(ch, "phikai", []forge.Role{forge.ReviewerRole})
	rules := []string{}
	for _, r := range got.Reasons {
		rules = append(rules, r.Rule)
	}
	if got.Owner != Mine || !slices.Contains(rules, "R2") || !slices.Contains(rules, "R3") || got.Since == nil {
		t.Fatalf("recorded reviewer turn: %+v", got)
	}
	got = Derive(ch, ch.Author.Username, []forge.Role{forge.AuthorRole})
	for _, r := range got.Reasons {
		if r.Rule == "A2" {
			t.Fatal("a recorded push answered the change request")
		}
	}
	closed := recorded(t, "3958")
	if got := Derive(closed, "phikai", []forge.Role{forge.ReviewerRole}); got.Owner != Theirs || len(got.Reasons) != 1 || got.Reasons[0].Text == "" {
		t.Fatal("merged change asks for action")
	}
}

func TestNoiseAndEarliestUnanswered(t *testing.T) {
	ch := base(t)
	reviewed(&ch)
	ch.Pushes = []forge.Push{{At: moment(2)}, {At: moment(4)}}
	ch.Requests = []forge.ReviewRequest{{Reviewer: "me", At: moment(3)}, {Reviewer: "me", At: moment(5)}}
	ch.Threads = []forge.Thread{{ID: "reply", Notes: []forge.Note{note("me", 1), note("author", 3), note("author", 4)}}}
	before := Derive(ch, "me", []forge.Role{forge.ReviewerRole})
	bot := note("bot", 6)
	bot.Author.Bot = true
	system := note("author", 7)
	system.System = true
	ch.Threads[0].Notes = append(ch.Threads[0].Notes, bot, system)
	ch.Reviews = append(ch.Reviews, forge.Review{Reviewer: "me", Bot: true, Verdict: forge.VerdictApproved, At: moment(6)})
	got := Derive(ch, "me", []forge.Role{forge.ReviewerRole})
	if !reflect.DeepEqual(got.Reasons, before.Reasons) || got.Since == nil || !got.Since.Equal(moment(2)) {
		t.Fatalf("noise reset turn: %+v", got)
	}
	// A co-reviewer does not hand the turn to me.
	ch.Threads[0].Notes = append(ch.Threads[0].Notes, note("other", 8))
	got = Derive(ch, "me", []forge.Role{forge.ReviewerRole})
	for _, r := range got.Reasons {
		if r.Rule == "R3" {
			t.Fatal("co-reviewer triggered R3")
		}
	}
	// A newer own review answers the earlier request/push.
	ch.Reviews = append(ch.Reviews, forge.Review{Reviewer: "me", Verdict: forge.VerdictCommented, At: moment(9), SHA: "head"})
	got = Derive(ch, "me", []forge.Role{forge.ReviewerRole})
	if got.Owner != Theirs {
		t.Fatalf("own review did not answer: %+v", got)
	}
}

func TestUnknownFactsAndGuardedMerge(t *testing.T) {
	ch := base(t)
	ch.Kind = forge.Forgejo
	reviewed(&ch)
	ch.Reviews[0].SHA = "old"
	got := Derive(ch, "me", []forge.Role{forge.ReviewerRole})
	if got.Owner != Mine || got.Since != nil || got.Reasons[0].Rule != "R2" {
		t.Fatal("missing push time invented an age")
	}
	ch = base(t)
	ch.Kind = forge.Forgejo
	ch.Mergeability = "mergeable"
	ch.Reviews = []forge.Review{{Reviewer: "me", Verdict: forge.VerdictApproved, At: moment(2), SHA: "head"}}
	for _, permission := range []*bool{nil, new(bool)} {
		ch.CanMerge = permission
		if got := Derive(ch, "author", []forge.Role{forge.AuthorRole}); got.Owner == Mine {
			t.Fatal("unconfirmed permission enabled merge")
		}
	}
	ch.CanMerge = yes()
	ch.Reviews = append(ch.Reviews, forge.Review{Reviewer: "other", Verdict: forge.VerdictChangesRequested, At: moment(3), SHA: "head"})
	got = Derive(ch, "author", []forge.Role{forge.AuthorRole})
	for _, r := range got.Reasons {
		if r.Rule == "A3" {
			t.Fatal("open change request enabled merge")
		}
	}
	ch.Reviews[1].Stale = true
	got = Derive(ch, "author", []forge.Role{forge.AuthorRole})
	if got.Owner != Mine || got.Reasons[0].Rule != "A3" {
		t.Fatal("stale change request blocked merge")
	}
	ch.Reviews[0].Dismissed = true
	if got := Derive(ch, "author", []forge.Role{forge.AuthorRole}); got.Owner == Mine {
		t.Fatal("dismissed approval enabled merge")
	}
	ch = base(t)
	ch.Mergeability = "mergeable"
	ch.CanMerge = yes()
	ch.Reviewers = []forge.Reviewer{{User: forge.User{Username: "r"}, State: forge.Approved}, {User: forge.User{Username: "other"}, State: forge.RequestedChanges}}
	got = Derive(ch, "author", []forge.Role{forge.AuthorRole})
	for _, r := range got.Reasons {
		if r.Rule == "A3" {
			t.Fatal("undated current change request enabled merge")
		}
	}
}

func TestRolesResolvedThreadsAndReasons(t *testing.T) {
	ch := base(t)
	reviewed(&ch)
	ch.Threads = []forge.Thread{{ID: "x", Resolvable: true, Resolved: true, Notes: []forge.Note{note("me", 1), note("author", 2)}}}
	for _, roles := range [][]forge.Role{{forge.ReviewerRole}, {forge.AssigneeRole}, {forge.ParticipantRole}, nil} {
		got := Derive(ch, "me", roles)
		if got.Owner != Theirs || len(got.Reasons) == 0 || strings.TrimSpace(got.Reasons[0].Text) == "" {
			t.Fatalf("inactive role/resolution: %+v", got)
		}
	}
	ch.Threads[0].Resolved = false
	if got := Derive(ch, "me", []forge.Role{forge.ReviewerRole, forge.AssigneeRole}); got.Owner != Mine {
		t.Fatal("multiple roles lost reviewer action")
	}
	ch.Reviewers[0].State = forge.Reviewed
	ch.Reviews = nil
	ch.Threads = nil
	if got := Derive(ch, "me", []forge.Role{forge.ReviewerRole}); got.Owner != Theirs {
		t.Fatal("current reviewed state ignored when verdict date unavailable")
	}
	ch.Reviewers[0].Bot = true
	if got := Derive(ch, "me", []forge.Role{forge.ReviewerRole}); got.Owner != Theirs {
		t.Fatal("bot reviewer triggered turn")
	}
}

func TestLatestNonCommentVerdict(t *testing.T) {
	ch := base(t)
	ch.Kind = forge.Forgejo
	ch.Mergeability = "mergeable"
	ch.CanMerge = yes()
	ch.Reviews = []forge.Review{{Reviewer: "r", Verdict: forge.VerdictChangesRequested, At: moment(1), SHA: "head"}, {Reviewer: "r", Verdict: forge.VerdictApproved, At: moment(2), SHA: "head"}, {Reviewer: "r", Verdict: forge.VerdictCommented, At: moment(3), SHA: "head"}}
	if got := Derive(ch, "author", []forge.Role{forge.AuthorRole}); got.Owner != Mine || got.Reasons[0].Rule != "A3" {
		t.Fatal("comment replaced verdict or old request survived approval")
	}
	ch.Reviews[1].Stale = true
	if got := Derive(ch, "author", []forge.Role{forge.AuthorRole}); got.Owner == Mine {
		t.Fatal("stale latest verdict resurrected old change request")
	}
}

func TestWaitingAges(t *testing.T) {
	ch := base(t)
	ch.Reviewers = []forge.Reviewer{{User: forge.User{Username: "me"}, State: forge.Unreviewed}}
	ch.Requests = []forge.ReviewRequest{{Reviewer: "me", At: moment(1)}}
	got := Derive(ch, "author", []forge.Role{forge.AuthorRole})
	if got.Owner != Theirs || got.Since == nil || !got.Since.Equal(moment(1)) || !strings.Contains(got.Reasons[0].Text, "me") {
		t.Fatalf("waiting reviewer age: %+v", got)
	}
	reviewed(&ch)
	ch.Threads = []forge.Thread{{ID: "x", Resolvable: true, Notes: []forge.Note{note("author", 1), note("me", 2), note("me", 3)}}}
	got = Derive(ch, "me", []forge.Role{forge.ReviewerRole})
	if got.Owner != Theirs || got.Since == nil || !got.Since.Equal(moment(2)) {
		t.Fatalf("waiting author age: %+v", got)
	}
}

func TestRepeatedChangeRequestDoesNotResetWait(t *testing.T) {
	ch := base(t)
	ch.Kind = forge.Forgejo
	ch.Reviews = []forge.Review{{Reviewer: "r", Verdict: forge.VerdictChangesRequested, At: moment(1), SHA: "head"}, {Reviewer: "r", Verdict: forge.VerdictChangesRequested, At: moment(3), SHA: "head"}}
	got := Derive(ch, "author", []forge.Role{forge.AuthorRole})
	if got.Since == nil || !got.Since.Equal(moment(1)) {
		t.Fatal("repeat request reset wait")
	}
	ch.Pushes = []forge.Push{{At: moment(2)}}
	got = Derive(ch, "author", []forge.Role{forge.AuthorRole})
	if got.Since == nil || !got.Since.Equal(moment(3)) {
		t.Fatal("author push did not answer earlier request")
	}
}

func TestUndatedCurrentChangeRequestRemainsActionable(t *testing.T) {
	ch := base(t)
	ch.Reviewers = []forge.Reviewer{{User: forge.User{Username: "r"}, State: forge.RequestedChanges}}
	got := Derive(ch, "author", []forge.Role{forge.AuthorRole})
	if got.Owner != Mine || len(got.Reasons) != 1 || got.Reasons[0].Rule != "A2" || got.Since != nil {
		t.Fatalf("undated current request: %+v", got)
	}
}

func TestMixedDatedAndUndatedRequestsHaveUnknownAge(t *testing.T) {
	ch := base(t)
	ch.Reviewers = []forge.Reviewer{{User: forge.User{Username: "me"}, State: forge.Unreviewed}}
	ch.Requests = []forge.ReviewRequest{{Reviewer: "me"}, {Reviewer: "me", At: moment(2)}}
	got := Derive(ch, "me", []forge.Role{forge.ReviewerRole})
	if got.Owner != Mine || got.Since != nil || got.Reasons[0].Since != nil {
		t.Fatalf("mixed request age: %+v", got)
	}
	reviewed(&ch)
	got = Derive(ch, "me", []forge.Role{forge.ReviewerRole})
	if got.Owner != Mine || got.Reasons[0].Rule != "R2b" || got.Since != nil {
		t.Fatalf("dated rerequest lost amid undated history: %+v", got)
	}
}

func TestMixedVerdictTimestampsPreserveUnknownUnansweredAge(t *testing.T) {
	ch := base(t)
	ch.Kind = forge.Forgejo
	ch.Reviews = []forge.Review{{Reviewer: "r", Verdict: forge.VerdictChangesRequested}, {Reviewer: "r", Verdict: forge.VerdictChangesRequested, At: moment(3), SHA: "head"}}
	got := Derive(ch, "author", []forge.Role{forge.AuthorRole})
	if got.Owner != Mine || got.Reasons[0].Rule != "A2" || got.Since != nil {
		t.Fatalf("undated unanswered verdict run: %+v", got)
	}
	ch.Reviews = append(ch.Reviews, forge.Review{Reviewer: "r", Verdict: forge.VerdictApproved, At: moment(4), SHA: "head"}, forge.Review{Reviewer: "r", Verdict: forge.VerdictChangesRequested, At: moment(5), SHA: "head"})
	got = Derive(ch, "author", []forge.Role{forge.AuthorRole})
	if got.Since == nil || !got.Since.Equal(moment(5)) {
		t.Fatal("opposing verdict did not end unknown run")
	}
	ch.Reviews = append(ch.Reviews, forge.Review{Reviewer: "r", Verdict: forge.VerdictApproved, SHA: "head"})
	ch.Mergeability = "mergeable"
	ch.CanMerge = yes()
	got = Derive(ch, "author", []forge.Role{forge.AuthorRole})
	if got.Owner != Mine || got.Reasons[0].Rule != "A3" || got.Since != nil {
		t.Fatal("latest undated verdict did not supersede dated history")
	}
}

func TestUndatedPushHistoryKeepsUnknownReviewAge(t *testing.T) {
	for _, tc := range []struct{ name, reviewSHA, pushHead string }{{"complete metadata", "old", "head"}, {"missing review SHA", "", "head"}, {"missing push head", "old", ""}} {
		t.Run(tc.name, func(t *testing.T) {
			ch := base(t)
			reviewed(&ch)
			ch.Reviews[0].SHA = tc.reviewSHA
			ch.Pushes = []forge.Push{{Head: tc.pushHead}, {At: moment(3), Head: "head"}}
			got := Derive(ch, "me", []forge.Role{forge.ReviewerRole})
			if got.Owner != Mine || got.Reasons[0].Rule != "R2" || got.Since != nil {
				t.Fatal("undated push history invented known earliest age")
			}
		})
	}
}

func TestCurrentGitLabVerdictSupersedesInvalidMatchingHistory(t *testing.T) {
	for _, tc := range []struct {
		name       string
		invalidate func(*forge.Review)
	}{
		{"dismissed", func(r *forge.Review) { r.Dismissed = true }},
		{"stale", func(r *forge.Review) { r.Stale = true }},
		{"old head", func(r *forge.Review) { r.SHA = "old" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ch := base(t)
			ch.Reviewers = []forge.Reviewer{{User: forge.User{Username: "r"}, State: forge.RequestedChanges}}
			ch.Reviews = []forge.Review{{Reviewer: "r", Verdict: forge.VerdictChangesRequested, At: moment(2), SHA: "head"}}
			tc.invalidate(&ch.Reviews[0])
			got := Derive(ch, "author", []forge.Role{forge.AuthorRole})
			if got.Owner != Mine || got.Reasons[0].Rule != "A2" || got.Since != nil {
				t.Fatalf("current request lost to invalid history: %+v", got)
			}
		})
	}
}

func TestMixedThreadTimestampsKeepUnansweredAgeUnknown(t *testing.T) {
	cases := []struct {
		name, me, rule string
		role           forge.Role
		notes          []forge.Note
		owner          Owner
	}{
		{"author unanswered reviewers", "author", "A1", forge.AuthorRole, []forge.Note{note("author", 1), note("me", 2), {Author: forge.User{Username: "other"}}}, Mine},
		{"reviewer author replies", "me", "R3", forge.ReviewerRole, []forge.Note{note("me", 1), note("author", 2), {Author: forge.User{Username: "author"}}}, Mine},
		{"reviewer waiting for author", "me", "waiting", forge.ReviewerRole, []forge.Note{note("author", 1), note("me", 2), {Author: forge.User{Username: "other"}}}, Theirs},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ch := base(t)
			reviewed(&ch)
			ch.Threads = []forge.Thread{{ID: "x", Resolvable: true, Notes: tc.notes}}
			got := Derive(ch, tc.me, []forge.Role{tc.role})
			if got.Owner != tc.owner || len(got.Reasons) != 1 || got.Reasons[0].Rule != tc.rule || got.Since != nil {
				t.Fatalf("mixed thread age: %+v", got)
			}
		})
	}
	ch := base(t)
	reviewed(&ch)
	ch.Threads = []forge.Thread{{ID: "x", Resolvable: true, Notes: []forge.Note{note("me", 1), note("author", 2), {Author: forge.User{Username: "bot", Bot: true}}, {Author: forge.User{Username: "author"}, System: true}}}}
	got := Derive(ch, "me", []forge.Role{forge.ReviewerRole})
	if got.Since == nil || !got.Since.Equal(moment(2)) {
		t.Fatal("undated bot/system noise changed reply age")
	}
}

func TestCurrentStatesAreForgeNeutral(t *testing.T) {
	for _, kind := range []string{forge.GitLab, forge.Forgejo, "future-forge"} {
		for _, tc := range []struct {
			state forge.ReviewState
			rule  string
		}{{forge.RequestedChanges, "A2"}, {forge.Approved, "A3"}} {
			t.Run(kind+"/"+string(tc.state), func(t *testing.T) {
				ch := base(t)
				ch.Kind = kind
				ch.CanMerge = yes()
				ch.Mergeability = "mergeable"
				ch.Reviewers = []forge.Reviewer{{User: forge.User{Username: "r"}, State: tc.state}}
				got := Derive(ch, "author", []forge.Role{forge.AuthorRole})
				if got.Owner != Mine || got.Reasons[0].Rule != tc.rule || got.Since != nil {
					t.Fatalf("current state without history: %+v", got)
				}
			})
		}
	}
}

func TestDismissedLatestReviewStillAnswersEarlierPushAndRequest(t *testing.T) {
	ch := base(t)
	reviewed(&ch)
	ch.Reviews[0].SHA = "old"
	ch.Pushes = []forge.Push{{At: moment(2), Head: "head"}}
	ch.Requests = []forge.ReviewRequest{{Reviewer: "me", At: moment(2)}}
	ch.Reviews = append(ch.Reviews, forge.Review{Reviewer: "me", Verdict: forge.VerdictApproved, At: moment(3), SHA: "head", Dismissed: true})
	got := Derive(ch, "me", []forge.Role{forge.ReviewerRole})
	if got.Owner != Theirs {
		t.Fatalf("dismissal resurrected answered push/request: %+v", got)
	}
	ch.Diff.Head = "next"
	ch.Pushes = append(ch.Pushes, forge.Push{At: moment(4), Head: "next"})
	got = Derive(ch, "me", []forge.Role{forge.ReviewerRole})
	if got.Owner != Mine || got.Reasons[0].Rule != "R2" || got.Since == nil || !got.Since.Equal(moment(4)) {
		t.Fatalf("push after dismissed review lost boundary: %+v", got)
	}
}

func TestLastHumanActivityIgnoresBotsSystemNotesAndTheUpdatedStamp(t *testing.T) {
	at := func(d int) time.Time { return time.Date(2026, 1, d, 0, 0, 0, 0, time.UTC) }
	ch := forge.Change{Created: at(1), Updated: at(28),
		Pushes: []forge.Push{{At: at(3)}},
		Threads: []forge.Thread{{Notes: []forge.Note{
			{Author: forge.User{Username: "alice"}, At: at(5)},
			{Author: forge.User{Username: "ci", Bot: true}, At: at(20)},
			{Author: forge.User{Username: "alice"}, System: true, At: at(21)},
		}}}}
	if got := LastHumanActivity(ch); got == nil || !got.Equal(at(5)) {
		t.Fatalf("last human activity %v, want the 5th", got)
	}
}
