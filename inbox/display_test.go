package inbox

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/turn"
)

func TestMixedTurnGroupsSortByUnansweredAgeAndKeepUnknownLast(t *testing.T) {
	as := []config.Forge{account("lab.test", forge.GitLab), account("jo.test", forge.Forgejo)}
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	ps := map[string]*fixture{}
	for _, a := range as {
		p := provider(a)
		ch := p.changes[p.members[0].Ref.String()]
		ch.Author = forge.User{Username: "author"}
		ch.Reviewers = []forge.Reviewer{{User: forge.User{Username: "me"}, State: forge.Unreviewed}}
		ch.Requests = []forge.ReviewRequest{{Reviewer: "me", At: now.Add(-time.Hour)}}
		if a.Kind == forge.Forgejo {
			ch.Requests[0].At = now.Add(-2 * time.Hour)
		}
		ch.Files = []forge.FileDiff{{Patch: "private-patch"}}
		ch.Drafts = []forge.Draft{{Body: "private-draft"}}
		p.members = p.members[:1]
		// An undated request comes after known ages, regardless of Ref ordering.
		unknown := clone(*ch)
		unknown.Ref.Number = 2
		unknown.Requests[0].At = time.Time{}
		p.members = append(p.members, forge.Discovered{Ref: unknown.Ref, Roles: []forge.Role{forge.ReviewerRole}})
		p.changes[unknown.Ref.String()] = &unknown
		// Both forges also contribute an author waiting for someone else's review.
		waiting := clone(*ch)
		waiting.Ref.Number = 3
		waiting.Author = forge.User{Username: "me"}
		waiting.Reviewers[0].Username = "other"
		waiting.Requests[0].Reviewer = "other"
		p.members = append(p.members, forge.Discovered{Ref: waiting.Ref, Roles: []forge.Role{forge.AuthorRole}})
		p.changes[waiting.Ref.String()] = &waiting
		ps[a.Name] = p
	}
	s, _ := New("", as, func(_ context.Context, a config.Forge) (forge.Provider, error) { return ps[a.Name], nil })
	s.Refresh(context.Background())
	got := s.Display(now)
	if len(got.Mine) != 4 || len(got.Theirs) != 2 || got.Mine[0].Kind != forge.Forgejo || got.Mine[1].Kind != forge.GitLab || got.Mine[2].Turn.Since != nil || got.Mine[3].Turn.Since != nil || got.Mine[2].Ref.String() > got.Mine[3].Ref.String() {
		t.Fatalf("ordering %+v", got)
	}
	for _, row := range got.Mine {
		if row.Turn.Owner != turn.Mine || len(row.Turn.Reasons) == 0 || len(row.Roles) == 0 {
			t.Fatal("row lost turn/roles")
		}
	}
	b, _ := json.Marshal(got)
	if strings.Contains(string(b), "private-patch") || strings.Contains(string(b), "private-draft") {
		t.Fatal("display exposed workspace content")
	}
	ps[as[0].Name].err = errors.New("sensitive-error")
	s.Refresh(context.Background())
	got = s.Display(now)
	if len(got.Mine) != 4 || len(got.Theirs) != 2 || got.Accounts[0].Error == "" || got.Accounts[1].Error != "" || strings.Contains(got.Accounts[0].Error, "sensitive-error") {
		t.Fatal("failed account hid mixed results")
	}
}

func TestSnoozeExpiresAndNewFactsWakeItWithoutBotConfigChanges(t *testing.T) {
	a := account("lab.test", forge.GitLab)
	p := provider(a)
	s, _ := New(t.TempDir(), []config.Forge{a}, func(context.Context, config.Forge) (forge.Provider, error) { return p, nil })
	s.Refresh(context.Background())
	ref := p.members[0].Ref
	now := time.Now()
	until := now.Add(time.Hour)
	if err := s.Snooze(ref, until); err != nil {
		t.Fatal(err)
	}
	if got := s.Display(now); len(got.Snoozed) != 1 || len(got.Mine)+len(got.Theirs) != 0 {
		t.Fatal("snooze did not hide active work")
	}
	// Bot classification is applied to copied facts, not the event fingerprint.
	s.accounts[0].Bots = nil
	if len(s.Display(now).Snoozed) != 1 {
		t.Fatal("bot config changed snooze")
	}
	if len(s.Display(until).Snoozed) != 0 {
		t.Fatal("snooze did not expire")
	}
	s.accounts[0].Bots = a.Bots
	p.changes[ref.String()].Threads[0].Notes = append(p.changes[ref.String()].Threads[0].Notes, forge.Note{ID: "new", Author: forge.User{Username: "author"}, Body: "new undated event"})
	// No new Updated timestamp: a targeted fresh read must still notice the event.
	if err := s.RefreshChange(context.Background(), ref); err != nil {
		t.Fatal(err)
	}
	if len(s.Display(now).Snoozed) != 0 {
		t.Fatal("new forge event did not restore row")
	}
	if err := s.Snooze(ref, until); err != nil {
		t.Fatal(err)
	}
	if err := s.Snooze(ref, time.Time{}); err != nil || len(s.Display(now).Snoozed) != 0 {
		t.Fatal("restore failed")
	}
}
