// Package drafts holds a review run's pending drafts, and serves them to the
// run's agent sessions over MCP (docs/plan.md section 6.2, decision D9).
//
// It is an agent's only write path, and what it writes is a pending draft,
// never a forge. Posting is the person's act, elsewhere. A reviewer sees and
// changes only its own drafts. A synthesizer lists every session's drafts and
// proposes merged ones, which name the drafts they came from. A verifier
// lists them too, and marks each one's confidence, which is all it can do.
//
// The finding shape and the rule-naming errors follow
// terva-action-code-review's review-result.v1 (docs/research.md).
package drafts

import (
	"fmt"
	"path"
	"slices"
	"strings"
	"sync"
	"time"
)

// Severity orders how much a finding matters.
type Severity string

var severities = []Severity{"critical", "high", "medium", "low", "info"}

// Side says which version of the file an anchor's lines are in.
type Side string

const (
	Head Side = "head" // the change's version, the default
	Base Side = "base" // the target's version
)

// Anchor places a draft on lines of one file.
type Anchor struct {
	Path      string `json:"path"`
	Side      Side   `json:"side,omitempty"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line,omitempty"`
}

// Status is where a draft stands.
type Status string

const (
	Pending   Status = "pending"
	Withdrawn Status = "withdrawn"
)

// Draft is one pending review comment drafted by an agent.
type Draft struct {
	ID          string   `json:"id"`
	Severity    Severity `json:"severity"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Evidence    string   `json:"evidence,omitempty"`
	Anchor      *Anchor  `json:"anchor,omitempty"`
	// Sources names the drafts a synthesizer merged into this one.
	Sources []string `json:"sources,omitempty"`

	Status   Status `json:"status"`
	Revision int    `json:"revision"`
	// Decision is the person's, never an agent's: "kept" made it a pending
	// comment on the forge, NoteID, and "dropped" set it aside. Once decided,
	// the agent can no longer change it.
	Decision string `json:"decision,omitempty"`
	NoteID   string `json:"note_id,omitempty"`
	Edited   bool   `json:"edited,omitempty"` // the person changed the text before keeping it
	// Confidence is the verifier's mark, and Override the person's, which
	// wins. The verifier's stays beside it, so the person can see both.
	Confidence *Confidence `json:"confidence,omitempty"`
	Override   *Confidence `json:"override,omitempty"`
	Session    string      `json:"session"`    // the session that drafted it
	Drafted    string      `json:"drafted_by"` // the tool and model, as the person sees them
	Updated    time.Time   `json:"updated"`
}

// Level is how far a finding held up when it was checked against the code.
type Level string

const (
	Confirmed Level = "confirmed" // the code shows it
	Plausible Level = "plausible" // it may hold, and the code does not settle it
	Doubtful  Level = "doubtful"  // the code argues against it
)

var levels = []Level{Confirmed, Plausible, Doubtful}

// Confidence is a mark on a finding, with the one-line reason for it.
type Confidence struct {
	Level  Level     `json:"level"`
	Reason string    `json:"reason,omitempty"`
	By     string    `json:"by"` // the session that marked it, or "person"
	At     time.Time `json:"at"`
}

// Proposal is what an agent sends to propose or revise a draft.
type Proposal struct {
	Severity    Severity `json:"severity"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Evidence    string   `json:"evidence,omitempty"`
	Anchor      *Anchor  `json:"anchor,omitempty"`
	Sources     []string `json:"sources,omitempty"`
}

// Role is what a session may do with the run's drafts.
type Role int

const (
	Reviewer    Role = iota + 1 // its own drafts only
	Synthesizer                 // lists every draft; its proposals name sources
	Verifier                    // lists every draft, and only marks confidence
)

// Session is an agent session as the store knows it.
type Session struct {
	ID      string
	Role    Role
	Drafted string // the tool and model, recorded on each draft
}

// RuleError is a draft that broke a rule. The agent sees Rule and Field, and
// can correct its proposal. It never quotes the proposal back.
type RuleError struct {
	Rule   string // a stable name, such as "anchor.range"
	Field  string // where, such as "anchor/end_line"
	Detail string // what the rule asks
}

func (e *RuleError) Error() string {
	return fmt.Sprintf("rule %s at %s: %s", e.Rule, e.Field, e.Detail)
}

const (
	maxReason = 512
	maxTitle  = 256
	maxText   = 20000
	maxLine   = 1_000_000
)

// Validate checks a proposal from a session of the given role.
func Validate(p Proposal, role Role) error {
	switch {
	case !slices.Contains(severities, p.Severity):
		return &RuleError{"severity.value", "severity", "one of critical, high, medium, low, info"}
	case strings.TrimSpace(p.Title) == "":
		return &RuleError{"title.empty", "title", "a title that is not blank"}
	case len(p.Title) > maxTitle:
		return &RuleError{"title.length", "title", fmt.Sprintf("at most %d bytes", maxTitle)}
	case strings.ContainsAny(p.Title, "\r\n"):
		return &RuleError{"title.oneLine", "title", "one line"}
	case strings.TrimSpace(p.Description) == "":
		return &RuleError{"description.empty", "description", "a description that is not blank"}
	case len(p.Description) > maxText:
		return &RuleError{"description.length", "description", fmt.Sprintf("at most %d bytes", maxText)}
	case len(p.Evidence) > maxText:
		return &RuleError{"evidence.length", "evidence", fmt.Sprintf("at most %d bytes", maxText)}
	}
	if a := p.Anchor; a != nil {
		if err := validateAnchor(a); err != nil {
			return err
		}
	}
	switch {
	case role == Reviewer && len(p.Sources) > 0:
		return &RuleError{"sources.reviewer", "sources", "only a synthesizer names sources"}
	case role == Synthesizer && len(p.Sources) == 0:
		return &RuleError{"sources.required", "sources", "the drafts this one merges, at least one"}
	}
	return nil
}

func validateAnchor(a *Anchor) error {
	clean := path.Clean(a.Path)
	switch {
	case a.Path == "":
		return &RuleError{"anchor.path", "anchor/path", "the file's path, relative to the repository root"}
	case strings.HasPrefix(a.Path, "/") || clean == ".." || strings.HasPrefix(clean, "../") || clean != a.Path:
		return &RuleError{"anchor.path", "anchor/path", "a clean path relative to the repository root, with no .. or leading /"}
	case a.Side != "" && a.Side != Head && a.Side != Base:
		return &RuleError{"anchor.side", "anchor/side", "head or base"}
	case a.StartLine < 1 || a.StartLine > maxLine:
		return &RuleError{"anchor.line", "anchor/start_line", "a line number from 1"}
	case a.EndLine != 0 && (a.EndLine < a.StartLine || a.EndLine > maxLine):
		return &RuleError{"anchor.range", "anchor/end_line", "a line at or after start_line"}
	}
	return nil
}

// Store holds one review run's drafts. It is safe for concurrent use, since
// the run's sessions work in parallel.
type Store struct {
	// CheckAnchor, when set, checks an anchor against the change, such as
	// that its lines are in the diff. It returns a *RuleError. Set it before
	// any session starts.
	CheckAnchor func(Anchor) error

	mu     sync.Mutex
	drafts []*Draft
	next   int
	now    func() time.Time
}

// NewStore returns an empty store.
func NewStore() *Store { return &Store{now: time.Now} }

// Restore returns a store holding drafts, as a run record kept them.
func Restore(drafts []Draft) *Store {
	st := NewStore()
	for i := range drafts {
		d := drafts[i]
		st.drafts = append(st.drafts, &d)
		var n int
		if _, err := fmt.Sscanf(d.ID, "d%d", &n); err == nil && n > st.next {
			st.next = n
		}
	}
	return st
}

// Propose validates p and adds it as a pending draft by s.
func (st *Store) Propose(s Session, p Proposal) (Draft, error) {
	if s.Role == Verifier {
		return Draft{}, roleErr
	}
	if err := st.validate(p, s.Role); err != nil {
		return Draft{}, err
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if err := st.checkSources(p.Sources); err != nil {
		return Draft{}, err
	}
	st.next++
	d := &Draft{
		ID: fmt.Sprintf("d%d", st.next), Status: Pending, Revision: 1,
		Session: s.ID, Drafted: s.Drafted, Updated: st.now(),
	}
	d.apply(p)
	st.drafts = append(st.drafts, d)
	return *d, nil
}

// Revise replaces the content of one of s's own pending drafts.
func (st *Store) Revise(s Session, id string, p Proposal) (Draft, error) {
	if s.Role == Verifier {
		return Draft{}, roleErr
	}
	if err := st.validate(p, s.Role); err != nil {
		return Draft{}, err
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	d, err := st.own(s, id)
	if err != nil {
		return Draft{}, err
	}
	if err := st.checkSources(p.Sources); err != nil {
		return Draft{}, err
	}
	d.apply(p)
	d.Revision++
	d.Updated = st.now()
	return *d, nil
}

func (st *Store) validate(p Proposal, role Role) error {
	if err := Validate(p, role); err != nil {
		return err
	}
	if p.Anchor != nil && st.CheckAnchor != nil {
		a := *p.Anchor
		if a.Side == "" {
			a.Side = Head
		}
		if a.EndLine == 0 {
			a.EndLine = a.StartLine
		}
		return st.CheckAnchor(a)
	}
	return nil
}

// Withdraw marks one of s's own pending drafts as withdrawn. It stays in the
// store, so the person can see what was set aside.
func (st *Store) Withdraw(s Session, id string) (Draft, error) {
	if s.Role == Verifier {
		return Draft{}, roleErr
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	d, err := st.own(s, id)
	if err != nil {
		return Draft{}, err
	}
	d.Status = Withdrawn
	d.Revision++
	d.Updated = st.now()
	return *d, nil
}

// roleErr refuses a verifier anything but marking.
var roleErr = &RuleError{"role.verifier", "name", "a verifier only lists drafts and rates them"}

// Rate records a verifier's mark on a pending draft. It changes nothing the
// person reads as the finding, so it does not count as a revision, and a
// decided draft can still be rated.
func (st *Store) Rate(s Session, id string, level Level, reason string) (Draft, error) {
	reason = strings.TrimSpace(reason)
	switch {
	case s.Role != Verifier:
		return Draft{}, &RuleError{"rate.role", "name", "only the verifier rates drafts"}
	case !slices.Contains(levels, level):
		return Draft{}, &RuleError{"confidence.level", "level", "one of confirmed, plausible, doubtful"}
	case reason == "":
		return Draft{}, &RuleError{"confidence.reason", "reason", "a one-line reason that is not blank"}
	case strings.ContainsAny(reason, "\r\n"):
		return Draft{}, &RuleError{"confidence.reason", "reason", "one line"}
	case len(reason) > maxReason:
		return Draft{}, &RuleError{"confidence.reason", "reason", fmt.Sprintf("at most %d bytes", maxReason)}
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	d := st.find(id)
	switch {
	case d == nil:
		return Draft{}, &RuleError{"draft.unknown", "id", "the id of a draft in this run"}
	case d.Status != Pending:
		return Draft{}, &RuleError{"draft.withdrawn", "id", "a draft that is still pending"}
	}
	d.Confidence = &Confidence{Level: level, Reason: reason, By: s.ID, At: st.now()}
	return *d, nil
}

// Override records the person's own mark on a draft, which wins over the
// verifier's. An empty level takes the person's mark away. Like Decide, it
// is vuoro's call, made from the UI, and no MCP tool reaches it.
func (st *Store) Override(id string, level Level, reason string) (Draft, error) {
	if level != "" && !slices.Contains(levels, level) {
		return Draft{}, fmt.Errorf("drafts: %q is not a confidence level", level)
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	d := st.find(id)
	if d == nil {
		return Draft{}, fmt.Errorf("drafts: no draft %s", id)
	}
	d.Override = nil
	if level != "" {
		d.Override = &Confidence{Level: level, Reason: strings.TrimSpace(reason), By: "person", At: st.now()}
	}
	return *d, nil
}

func (st *Store) find(id string) *Draft {
	for _, d := range st.drafts {
		if d.ID == id {
			return d
		}
	}
	return nil
}

// Kept and Dropped are the person's decisions on a draft.
const (
	Kept    = "kept"
	Dropped = "dropped"
)

// Decide records the person's decision on a pending draft. It is vuoro's
// call, made from the UI, and no MCP tool reaches it.
func (st *Store) Decide(id, decision, noteID string, edited bool) (Draft, error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	for _, d := range st.drafts {
		if d.ID != id {
			continue
		}
		switch {
		case d.Status != Pending:
			return Draft{}, fmt.Errorf("drafts: %s was withdrawn", id)
		case d.Decision != "":
			return Draft{}, fmt.Errorf("drafts: %s is already %s", id, d.Decision)
		}
		d.Decision, d.NoteID, d.Edited = decision, noteID, edited
		d.Updated = st.now()
		return *d, nil
	}
	return Draft{}, fmt.Errorf("drafts: no draft %s", id)
}

// Get returns one draft.
func (st *Store) Get(id string) (Draft, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	for _, d := range st.drafts {
		if d.ID == id {
			return *d, true
		}
	}
	return Draft{}, false
}

// List returns the drafts s may see: a reviewer's own, or every draft for a
// synthesizer or a verifier. Withdrawn drafts are left out unless withdrawn is set.
func (st *Store) List(s Session, withdrawn bool) []Draft {
	st.mu.Lock()
	defer st.mu.Unlock()
	var out []Draft
	for _, d := range st.drafts {
		if s.Role == Reviewer && d.Session != s.ID {
			continue
		}
		if d.Status == Withdrawn && !withdrawn {
			continue
		}
		out = append(out, *d)
	}
	return out
}

// All returns every draft, for vuoro's own use.
func (st *Store) All() []Draft {
	st.mu.Lock()
	defer st.mu.Unlock()
	out := make([]Draft, len(st.drafts))
	for i, d := range st.drafts {
		out[i] = *d
	}
	return out
}

func (st *Store) own(s Session, id string) (*Draft, error) {
	for _, d := range st.drafts {
		if d.ID != id {
			continue
		}
		switch {
		case d.Session != s.ID:
			return nil, &RuleError{"draft.owner", "id", "a draft this session proposed"}
		case d.Status != Pending:
			return nil, &RuleError{"draft.withdrawn", "id", "a draft that is still pending"}
		case d.Decision != "":
			return nil, &RuleError{"draft.decided", "id", "a draft the person has not yet kept or dropped"}
		}
		return d, nil
	}
	return nil, &RuleError{"draft.unknown", "id", "the id of a draft in this run"}
}

func (st *Store) checkSources(ids []string) error {
	for i, id := range ids {
		if !slices.ContainsFunc(st.drafts, func(d *Draft) bool { return d.ID == id }) {
			return &RuleError{"sources.unknown", fmt.Sprintf("sources/%d", i), "the id of a draft in this run"}
		}
		if slices.Contains(ids[:i], id) {
			return &RuleError{"sources.duplicate", fmt.Sprintf("sources/%d", i), "each source once"}
		}
	}
	return nil
}

func (d *Draft) apply(p Proposal) {
	d.Severity, d.Title, d.Description, d.Evidence = p.Severity, strings.TrimSpace(p.Title), p.Description, p.Evidence
	d.Anchor, d.Sources = nil, slices.Clone(p.Sources)
	if p.Anchor != nil {
		a := *p.Anchor
		if a.Side == "" {
			a.Side = Head
		}
		if a.EndLine == 0 {
			a.EndLine = a.StartLine
		}
		d.Anchor = &a
	}
}
