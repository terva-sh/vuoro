package inbox

import (
	"encoding/json"
	"github.com/terva-sh/vuoro/forge"
)

// All persisted model fields have JSON-safe types. Checkout is deliberately
// excluded: opening an inbox row gets current transport metadata from its forge.
func clone[T any](v T) T {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	var out T
	if err := json.Unmarshal(b, &out); err != nil {
		panic(err)
	}
	return out
}

// Classify marks the change's configured bot accounts, and every account a
// bot review names, as automated. Project triage applies it as the inbox does.
func Classify(ch *forge.Change, configured []string) { classify(ch, configured) }

func classify(ch *forge.Change, configured []string) {
	bots := map[string]bool{}
	for _, name := range configured {
		bots[name] = true
	}
	for _, r := range ch.Reviews {
		if r.Bot {
			bots[r.Reviewer] = true
		}
	}
	users := []*forge.User{&ch.Author}
	for i := range ch.Reviewers {
		users = append(users, &ch.Reviewers[i].User)
	}
	for i := range ch.Assignees {
		users = append(users, &ch.Assignees[i])
	}
	for i := range ch.Participants {
		users = append(users, &ch.Participants[i])
	}
	for i := range ch.Threads {
		for j := range ch.Threads[i].Notes {
			users = append(users, &ch.Threads[i].Notes[j].Author)
		}
	}
	for _, u := range users {
		if u.Bot {
			bots[u.Username] = true
		}
	}
	for _, u := range users {
		u.Bot = u.Bot || bots[u.Username]
	}
	for i := range ch.Reviews {
		ch.Reviews[i].Bot = ch.Reviews[i].Bot || bots[ch.Reviews[i].Reviewer]
	}
}
