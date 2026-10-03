package inbox

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/turn"
)

type AccountStatus struct {
	Key         string    `json:"key"`
	Name        string    `json:"name"`
	Kind        string    `json:"kind"`
	Host        string    `json:"host"`
	Username    string    `json:"username"`
	LastAttempt time.Time `json:"last_attempt"`
	LastSuccess time.Time `json:"last_success"`
	Error       string    `json:"error,omitempty"`
}

type Row struct {
	Ref          forge.Ref    `json:"ref"`
	Kind         string       `json:"kind"`
	AccountKey   string       `json:"account_key"`
	Account      string       `json:"account"`
	Title        string       `json:"title"`
	WebURL       string       `json:"web_url"`
	Draft        bool         `json:"draft"`
	Roles        []forge.Role `json:"roles"`
	Turn         turn.Turn    `json:"turn"`
	SnoozedUntil *time.Time   `json:"snoozed_until,omitempty"`
}

// Display is the window's lightweight inbox, without patches or private drafts.
type Display struct {
	Accounts []AccountStatus `json:"accounts"`
	Mine     []Row           `json:"mine"`
	Theirs   []Row           `json:"theirs"`
	Snoozed  []Row           `json:"snoozed"`
}

func (s *Service) Display(now time.Time) Display {
	view := s.rawSnapshot()
	out := Display{Accounts: []AccountStatus{}, Mine: []Row{}, Theirs: []Row{}, Snoozed: []Row{}}
	for i, a := range view.Accounts {
		out.Accounts = append(out.Accounts, AccountStatus{a.Key, a.Name, a.Kind, a.Host, a.Username, a.LastAttempt, a.LastSuccess, a.Error})
		for _, item := range a.Items {
			ch := item.Change
			if ch.State != forge.Open {
				continue
			}
			annotation := view.Annotations[ch.Ref.String()]
			snoozed := annotation.SnoozedUntil.After(now) && (annotation.EventKey == "" || annotation.EventKey == eventKey(ch))
			classify(&ch, s.accounts[i].Bots)
			row := Row{Ref: ch.Ref, Kind: ch.Kind, AccountKey: a.Key, Account: a.Name, Title: ch.Title, WebURL: ch.WebURL, Draft: ch.Draft, Roles: item.Roles, Turn: turn.Derive(ch, a.Username, item.Roles)}
			if snoozed {
				until := annotation.SnoozedUntil
				row.SnoozedUntil = &until
				out.Snoozed = append(out.Snoozed, row)
			} else if row.Turn.Owner == turn.Mine {
				out.Mine = append(out.Mine, row)
			} else {
				out.Theirs = append(out.Theirs, row)
			}
		}
	}
	for _, rows := range [][]Row{out.Mine, out.Theirs, out.Snoozed} {
		slices.SortFunc(rows, compareRows)
	}
	return out
}

func compareRows(a, b Row) int {
	x, y := a.Turn.Since, b.Turn.Since
	if x == nil && y != nil {
		return 1
	}
	if x != nil && y == nil {
		return -1
	}
	if x != nil && y != nil {
		if n := x.Compare(*y); n != 0 {
			return n
		}
	}
	return strings.Compare(a.Ref.String(), b.Ref.String())
}

// Fingerprint raw forge events, before config-driven bot classification. Ignore
// workspace content and capabilities so entering a workspace cannot wake a row.
func eventKey(ch forge.Change) string {
	ch.Files = nil
	ch.Drafts = nil
	ch.Capabilities = forge.Capabilities{}
	ch.Checkout = forge.Checkout{}
	b, _ := json.Marshal(ch)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
