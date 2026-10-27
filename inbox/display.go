package inbox

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
	Warning     string    `json:"warning,omitempty"`
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
	// Error says why the last refresh could not read this change, whose
	// facts are then the last ones read.
	Error string `json:"error,omitempty"`
	// Lane is where a row on the person's turn sits (D38): their own
	// changes, replies they owe, or reviews. It is empty on their turn.
	Lane Lane `json:"lane,omitempty"`
	// Summary is the turn's reasons, one line per kind, with counts.
	Summary []string `json:"summary"`
	// Waiting are the threads behind the reasons, for the detail pane.
	Waiting []Waiting `json:"waiting,omitempty"`
	// Quiet is the last human activity on the change.
	Quiet *time.Time `json:"quiet_since,omitempty"`
}

// Lane is a part of "Your turn" (D38), in the order the inbox shows them.
type Lane string

const (
	LaneYours   Lane = "yours"   // changes requested on, or ready to merge, the person's own change
	LaneReplies Lane = "replies" // threads waiting on the person's answer
	LaneReviews Lane = "reviews" // reviews requested, or due again
)

// Waiting is one thread a reason points at.
type Waiting struct {
	Rule  string     `json:"rule"`
	Who   string     `json:"who"`
	Where string     `json:"where"` // path:line, or "general"
	Text  string     `json:"text"`  // the first line of the thread's last real note
	Since *time.Time `json:"since"`
}

// UntilChange is the snooze that only a new forge event ends (D38).
var UntilChange = time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC)

// Display is the window's lightweight inbox, without patches or private drafts.
type Display struct {
	Accounts []AccountStatus `json:"accounts"`
	Mine     []Row           `json:"mine"`
	Theirs   []Row           `json:"theirs"`
	Snoozed  []Row           `json:"snoozed"`
	// Unread are discovered changes never read, so whose turn it is is
	// unknown. Each carries its Error.
	Unread []Row `json:"unread"`
	// Dormant are changes with no human activity for the dormant threshold,
	// whoever's turn it is (D38).
	Dormant []Row `json:"dormant"`
}

func (s *Service) Display(now time.Time) Display {
	view := s.rawSnapshot()
	out := Display{Accounts: []AccountStatus{}, Mine: []Row{}, Theirs: []Row{}, Snoozed: []Row{}, Unread: []Row{}, Dormant: []Row{}}
	for i, a := range view.Accounts {
		out.Accounts = append(out.Accounts, AccountStatus{a.Key, a.Name, a.Kind, a.Host, a.Username, a.LastAttempt, a.LastSuccess, a.Error, a.Warning})
		for _, item := range a.Items {
			ch := item.Change
			if ch.State != forge.Open {
				continue
			}
			if item.Unread {
				out.Unread = append(out.Unread, Row{Ref: ch.Ref, Kind: ch.Kind, AccountKey: a.Key, Account: a.Name, Roles: item.Roles, Error: item.Error})
				continue
			}
			annotation := view.Annotations[ch.Ref.String()]
			snoozed := annotation.SnoozedUntil.After(now) && (annotation.EventKey == "" || annotation.EventKey == eventKey(ch))
			classify(&ch, s.accounts[i].Bots)
			row := Row{Ref: ch.Ref, Kind: ch.Kind, AccountKey: a.Key, Account: a.Name, Title: ch.Title, WebURL: ch.WebURL, Draft: ch.Draft, Roles: item.Roles, Turn: turn.Derive(ch, a.Username, item.Roles), Error: item.Error}
			row.Quiet = turn.LastHumanActivity(ch)
			row.Summary, row.Waiting = summarize(ch, row.Turn.Reasons)
			if row.Turn.Owner == turn.Mine {
				row.Lane = lane(row.Turn.Reasons)
			}
			if snoozed {
				until := annotation.SnoozedUntil
				row.SnoozedUntil = &until
				out.Snoozed = append(out.Snoozed, row)
			} else if s.Dormant > 0 && row.Quiet != nil && now.Sub(*row.Quiet) > s.Dormant {
				out.Dormant = append(out.Dormant, row)
			} else if row.Turn.Owner == turn.Mine {
				out.Mine = append(out.Mine, row)
			} else {
				out.Theirs = append(out.Theirs, row)
			}
		}
	}
	for _, rows := range [][]Row{out.Mine, out.Theirs, out.Snoozed, out.Unread, out.Dormant} {
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
	// Project triage's facts are not review events: a pipeline finishing
	// must not wake a snoozed row, and leaving them out keeps every
	// fingerprint taken before they existed.
	ch.Labels, ch.Conflicts, ch.Pipeline, ch.BlockedBy, ch.DependenciesKnown, ch.SourceProject, ch.Touched = nil, false, "", nil, false, "", nil
	b, _ := json.Marshal(ch)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// lane is the lane of a row's most pressing reason.
func lane(reasons []turn.Reason) Lane {
	best := Lane("")
	rank := map[Lane]int{"": 0, LaneReviews: 1, LaneReplies: 2, LaneYours: 3}
	for _, r := range reasons {
		var l Lane
		switch r.Rule {
		case "A2", "A3":
			l = LaneYours
		case "A1", "R3":
			l = LaneReplies
		case "R1", "R2", "R2b":
			l = LaneReviews
		}
		if rank[l] > rank[best] {
			best = l
		}
	}
	if best == "" {
		best = LaneReviews
	}
	return best
}

// summarize folds the reasons into one line per kind, so eight waiting
// threads read as one line with counts, and lists the threads they point at.
func summarize(ch forge.Change, reasons []turn.Reason) ([]string, []Waiting) {
	summary := []string{}
	var waiting []Waiting
	threads := map[string]forge.Thread{}
	for _, t := range ch.Threads {
		threads[t.ID] = t
	}
	type group struct {
		first string
		n     int
		who   map[string]int
	}
	var order []string
	groups := map[string]*group{}
	for _, r := range reasons {
		key := r.Rule
		if r.Rule != "A1" && r.Rule != "R3" {
			key = r.Rule + "\x00" + r.Text
		}
		g := groups[key]
		if g == nil {
			g = &group{first: r.Text, who: map[string]int{}}
			groups[key] = g
			order = append(order, key)
		}
		g.n++
		if r.Who != "" {
			g.who[r.Who]++
		}
		if t, ok := threads[r.ThreadID]; ok && r.ThreadID != "" {
			waiting = append(waiting, thread(t, r))
		}
	}
	for _, key := range order {
		g := groups[key]
		rule, _, _ := strings.Cut(key, "\x00")
		switch {
		case g.n == 1:
			summary = append(summary, g.first)
		case rule == "A1":
			summary = append(summary, fmt.Sprintf("%d threads await your answer: %s.", g.n, counts(g.who)))
		case rule == "R3":
			summary = append(summary, fmt.Sprintf("%s answered in %d threads you are in.", counts(g.who), g.n))
		default:
			summary = append(summary, fmt.Sprintf("%s (%d threads)", strings.TrimSuffix(g.first, "."), g.n))
		}
	}
	return summary, waiting
}

// counts lists names by how often they appear, most first: "alice (5), bo (3)".
func counts(who map[string]int) string {
	names := make([]string, 0, len(who))
	for n := range who {
		names = append(names, n)
	}
	slices.SortFunc(names, func(a, b string) int { return cmp.Or(who[b]-who[a], strings.Compare(a, b)) })
	parts := make([]string, len(names))
	for i, n := range names {
		parts[i] = n
		if who[n] > 1 {
			parts[i] = fmt.Sprintf("%s (%d)", n, who[n])
		}
	}
	return strings.Join(parts, ", ")
}

// maxExcerpt bounds a waiting thread's excerpt, in runes.
const maxExcerpt = 160

func thread(t forge.Thread, r turn.Reason) Waiting {
	w := Waiting{Rule: r.Rule, Who: r.Who, Where: "general", Since: r.Since}
	if a := t.Anchor; a != nil {
		w.Where = a.Path // a comment on the whole file has no line
		if line := max(a.NewLine, a.OldLine); line > 0 {
			w.Where = fmt.Sprintf("%s:%d", a.Path, line)
		}
	}
	for i := len(t.Notes) - 1; i >= 0; i-- {
		n := t.Notes[i]
		if n.System || n.Author.Bot {
			continue
		}
		if w.Who == "" {
			w.Who = n.Author.Username
		}
		line, _, _ := strings.Cut(strings.TrimSpace(n.Body), "\n")
		if rs := []rune(line); len(rs) > maxExcerpt {
			line = string(rs[:maxExcerpt]) + "…"
		}
		w.Text = line
		break
	}
	return w
}
