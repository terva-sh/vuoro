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
	"errors"
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
	// Suggestion, when set, is the text that replaces the anchor's lines at
	// the head, which the forge offers the author to apply. Only a head-side
	// anchor carries one.
	Suggestion *string `json:"suggestion,omitempty"`
	// Triage replies are local drafts attached to an existing human thread.
	ThreadID  string   `json:"thread_id,omitempty"`
	Category  Category `json:"category,omitempty"`
	Reason    string   `json:"reason,omitempty"`
	ThreadKey string   `json:"thread_key,omitempty"`
	// Change, Reviewers, Labels and Unblock are a project triage proposal:
	// the change it is for, the reviewers and labels to add, and what would
	// unblock it.
	Change    int      `json:"change,omitempty"`
	Reviewers []string `json:"reviewers,omitempty"`
	Labels    []string `json:"labels,omitempty"`
	Unblock   string   `json:"unblock,omitempty"`
	// FixedIn is the pushed commit that fixes the draft, which vuoro or the
	// person, never an agent, records (D41, D50). FixedSubject is that
	// commit's subject, and FixedBy says who linked it: "editor", from the
	// commit's trailer, "person", or "suggestion", for a commit that put a
	// suggested fix's lines in place (D59).
	FixedIn      string `json:"fixed_in,omitempty"`
	FixedSubject string `json:"fixed_subject,omitempty"`
	FixedBy      string `json:"fixed_by,omitempty"`
	// Suggested is the editor's fix sent back as suggestions on the
	// person's pending comments, rather than pushed (D59). vuoro records it,
	// never an agent, and it stands until a later head shows it applied.
	Suggested *SuggestedFix `json:"suggested,omitempty"`
	// Earlier is the finding of the review this run follows that the draft
	// restates, and Carried says its decision came from that finding (D50).
	Earlier string `json:"earlier,omitempty"`
	Carried bool   `json:"carried,omitempty"`

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
	Suggestion  *string  `json:"suggestion,omitempty"`
	ThreadID    string   `json:"thread_id,omitempty"`
	Category    Category `json:"category,omitempty"`
	Reason      string   `json:"reason,omitempty"`
	Earlier     string   `json:"earlier,omitempty"`
	Change      int      `json:"change,omitempty"`
	Reviewers   []string `json:"reviewers,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	Unblock     string   `json:"unblock,omitempty"`
}

// Role is what a session may do with the run's drafts.
type Role int

const (
	Reviewer      Role = iota + 1 // its own drafts only
	Synthesizer                   // lists every draft; its proposals name sources
	Verifier                      // lists every draft, and only marks confidence
	Triage                        // groups existing threads and drafts replies
	Summary                       // no draft access; receives no MCP server
	Suggester                     // sets the suggestion of its one Target draft
	Editor                        // writes code in its own worktree; no draft access and no MCP server
	ProjectTriage                 // proposes reviewers, labels and unblocking for a project's changes
)

// Session is an agent session as the store knows it.
type Session struct {
	ID      string
	Role    Role
	Drafted string // the tool and model, recorded on each draft
	// Target is the one draft a Suggester may give a suggestion.
	Target string
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
	if role == Summary || role == Editor {
		return &RuleError{"role.summary", "name", "a summary or an editor cannot draft"}
	}
	if role == Suggester {
		return suggesterErr
	}
	if role == Triage {
		if p.Change != 0 || len(p.Reviewers)+len(p.Labels) > 0 || p.Unblock != "" {
			return &RuleError{"triage.shape", "arguments", "thread_id, category, reason and optional description only"}
		}
		return validateTriage(p)
	}
	if role == ProjectTriage {
		return validateProjectTriage(p)
	}
	if p.Change != 0 || len(p.Reviewers)+len(p.Labels) > 0 || p.Unblock != "" {
		return &RuleError{"project.role", "change", "only project triage proposes reviewers and labels"}
	}
	if p.ThreadID != "" || p.Category != "" || p.Reason != "" {
		return &RuleError{"triage.role", "thread_id", "only author triage attaches a reply to a thread"}
	}
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
	if p.Suggestion != nil {
		switch {
		case p.Anchor == nil || p.Anchor.Side == Base:
			return &RuleError{"suggestion.anchor", "suggestion", "an anchor on head-side lines, which the suggestion replaces"}
		case len(*p.Suggestion) > maxText:
			return &RuleError{"suggestion.length", "suggestion", fmt.Sprintf("at most %d bytes", maxText)}
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
	// CheckSuggestion, when set, checks a suggestion against the lines it
	// replaces, such as with Boundary. It returns a *RuleError.
	CheckSuggestion func(a Anchor, suggestion string) error
	// CheckThread binds a triage proposal to the run's immutable snapshot.
	CheckThread func(string) (string, error)
	// CheckProposal binds a project triage proposal to the run's changes,
	// candidates and labels. It returns a *RuleError.
	CheckProposal func(Proposal) error

	mu      sync.Mutex
	drafts  []*Draft
	earlier []Earlier
	// keeping holds the drafts whose Keep is writing to the forge, so a
	// second Keep, a drop, or an agent's change waits for the outcome.
	keeping map[string]bool
	next    int
	now     func() time.Time
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
	if s.Role == Triage && slices.ContainsFunc(st.drafts, func(d *Draft) bool { return d.ThreadID == p.ThreadID && d.Status == Pending }) {
		return Draft{}, &RuleError{"thread.duplicate", "thread_id", "revise the existing pending triage for this thread"}
	}
	if s.Role == ProjectTriage && slices.ContainsFunc(st.drafts, func(d *Draft) bool { return d.Change == p.Change && d.Status == Pending }) {
		return Draft{}, &RuleError{"change.duplicate", "change", "revise your existing pending proposal for this change"}
	}
	st.next++
	d := &Draft{
		ID: fmt.Sprintf("d%d", st.next), Status: Pending, Revision: 1,
		Session: s.ID, Drafted: s.Drafted, Updated: st.now(),
	}
	d.apply(p)
	if err := st.carry(d, p); err != nil {
		st.next--
		return Draft{}, err
	}
	if p.ThreadID != "" {
		d.ThreadKey, _ = st.CheckThread(p.ThreadID)
	}
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
	if s.Role == Triage && p.ThreadID != d.ThreadID {
		return Draft{}, &RuleError{"reply.thread", "thread_id", "the thread this draft already belongs to"}
	}
	if s.Role == ProjectTriage && (p.Change != d.Change || d.Decision != "") {
		return Draft{}, &RuleError{"proposal.change", "change", "a pending proposal you have not seen accepted, for the change it already names"}
	}
	if err := st.checkSources(p.Sources); err != nil {
		return Draft{}, err
	}
	was := *d
	d.apply(p)
	if err := st.carry(d, p); err != nil {
		*d = was
		return Draft{}, err
	}
	d.Revision++
	d.Updated = st.now()
	return *d, nil
}

func (st *Store) validate(p Proposal, role Role) error {
	if err := Validate(p, role); err != nil {
		return err
	}
	if role == ProjectTriage {
		if st.CheckProposal == nil {
			return &RuleError{"change.unknown", "change", "a change in this triage run"}
		}
		return st.CheckProposal(p)
	}
	if role == Triage {
		if st.CheckThread == nil {
			return &RuleError{"thread.unknown", "thread_id", "a thread in this triage run"}
		}
		_, err := st.CheckThread(p.ThreadID)
		return err
	}
	if p.Anchor == nil {
		return nil
	}
	a := *p.Anchor
	if a.Side == "" {
		a.Side = Head
	}
	if a.EndLine == 0 {
		a.EndLine = a.StartLine
	}
	if st.CheckAnchor != nil {
		if err := st.CheckAnchor(a); err != nil {
			return err
		}
	}
	if p.Suggestion != nil && st.CheckSuggestion != nil {
		return st.CheckSuggestion(a, *p.Suggestion)
	}
	return nil
}

// Boundary refuses a suggestion that repeats the line just before or just
// after the range it replaces. Applying it would leave that line twice, the
// commonest way a suggestion breaks the code, as when the range stops short
// of a closing brace that the suggestion also writes. lines is the file at
// the head, and a blank or missing neighbour is not checked.
func Boundary(lines []string, a Anchor, suggestion string) error {
	sl := strings.Split(strings.TrimRight(suggestion, "\n"), "\n")
	near := func(n int) string { // line n, from 1, or "" past either end
		if n < 1 || n > len(lines) {
			return ""
		}
		return strings.TrimRight(lines[n-1], "\r")
	}
	if prev := near(a.StartLine - 1); strings.TrimSpace(prev) != "" && strings.TrimRight(sl[0], "\r") == prev {
		return &RuleError{"suggestion.boundary", "suggestion", fmt.Sprintf(
			"replacement lines for %d–%d only: its first line repeats line %d, which stays; start the range at line %d, or leave that line out", a.StartLine, a.EndLine, a.StartLine-1, a.StartLine-1)}
	}
	if next := near(a.EndLine + 1); strings.TrimSpace(next) != "" && strings.TrimRight(sl[len(sl)-1], "\r") == next {
		return &RuleError{"suggestion.boundary", "suggestion", fmt.Sprintf(
			"replacement lines for %d–%d only: its last line repeats line %d, which stays; end the range at line %d, or leave that line out", a.StartLine, a.EndLine, a.EndLine+1, a.EndLine+1)}
	}
	return nil
}

// Withdraw marks one of s's own pending drafts as withdrawn. It stays in the
// store, so the person can see what was set aside.
func (st *Store) Withdraw(s Session, id string) (Draft, error) {
	if s.Role == Summary || s.Role == Editor {
		return Draft{}, &RuleError{"role.summary", "name", "a summary or an editor cannot draft"}
	}
	if s.Role == Verifier {
		return Draft{}, roleErr
	}
	if s.Role == Suggester {
		return Draft{}, suggesterErr
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

// Suggest sets the suggestion of s's target draft, the lines that replace
// its anchor at the head. anchor, when set, widens the draft's anchor to the
// lines the suggestion replaces: the same file, head side, and covering the
// lines it had. Nothing else of the draft changes.
func (st *Store) Suggest(s Session, id string, anchor *Anchor, suggestion string) (Draft, error) {
	switch {
	case s.Role != Suggester:
		return Draft{}, &RuleError{"suggest.role", "name", "only a suggestion session sets a suggestion this way"}
	case id != s.Target:
		return Draft{}, &RuleError{"suggest.target", "id", "the draft this session was started for, " + s.Target}
	case strings.TrimSpace(suggestion) == "":
		return Draft{}, &RuleError{"suggestion.empty", "suggestion", "the replacement lines; reply instead when none fixes it"}
	case len(suggestion) > maxText:
		return Draft{}, &RuleError{"suggestion.length", "suggestion", fmt.Sprintf("at most %d bytes", maxText)}
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	d := st.find(id)
	switch {
	case d == nil:
		return Draft{}, &RuleError{"draft.unknown", "id", "the id of a draft in this run"}
	case d.Status != Pending:
		return Draft{}, &RuleError{"draft.withdrawn", "id", "a draft that is still pending"}
	case d.Decision != "":
		return Draft{}, &RuleError{"draft.decided", "id", "a draft the person has not yet kept or dropped"}
	case d.Anchor == nil || d.Anchor.Side != Head:
		return Draft{}, &RuleError{"suggestion.anchor", "id", "a draft anchored on head-side lines"}
	case st.keeping[d.ID]:
		return Draft{}, keepingErr
	}
	a := *d.Anchor
	if anchor != nil {
		w := *anchor
		if w.Side == "" {
			w.Side = Head
		}
		if w.EndLine == 0 {
			w.EndLine = w.StartLine
		}
		if err := validateAnchor(&w); err != nil {
			return Draft{}, err
		}
		switch {
		case w.Path != a.Path || w.Side != Head:
			return Draft{}, &RuleError{"suggestion.anchor", "anchor", "the draft's own file, " + a.Path + ", on the head side"}
		case w.StartLine > a.StartLine || w.EndLine < a.EndLine:
			return Draft{}, &RuleError{"suggestion.anchor", "anchor", fmt.Sprintf("a range that still covers lines %d–%d", a.StartLine, a.EndLine)}
		}
		if st.CheckAnchor != nil {
			if err := st.CheckAnchor(w); err != nil {
				return Draft{}, err
			}
		}
		a = w
	}
	if st.CheckSuggestion != nil {
		if err := st.CheckSuggestion(a, suggestion); err != nil {
			return Draft{}, err
		}
	}
	text := strings.TrimRight(suggestion, "\n")
	d.Anchor, d.Suggestion = &a, &text
	d.Revision++
	d.Updated = st.now()
	return *d, nil
}

// roleErr refuses a verifier anything but marking.
var roleErr = &RuleError{"role.verifier", "name", "a verifier only lists drafts and rates them"}

// suggesterErr refuses a suggestion session anything but its one suggestion.
var suggesterErr = &RuleError{"role.suggester", "name", "a suggestion session only sets its draft's suggestion"}

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

// keepingErr refuses a change to a draft while the person's Keep of it is
// writing to the forge.
var keepingErr = &RuleError{"draft.keeping", "id", "a draft the person is not keeping right now"}

// ErrKeeping means a Keep of the draft is already writing to the forge.
var ErrKeeping = errors.New("drafts: this draft is already being kept")

// Keep records the person's Keep of a pending draft, around write, which
// puts it on the forge and returns the note's ID. The draft is held while
// write runs, without holding the store, so a second Keep is refused rather
// than writing the comment twice, and agents carry on meanwhile. It is
// vuoro's call, made from the UI, and no MCP tool reaches it.
func (st *Store) Keep(id string, write func(Draft) (noteID string, edited bool, err error)) (Draft, error) {
	st.mu.Lock()
	d := st.find(id)
	switch {
	case d == nil:
		st.mu.Unlock()
		return Draft{}, fmt.Errorf("drafts: no draft %s", id)
	case st.keeping[id]:
		st.mu.Unlock()
		return Draft{}, ErrKeeping
	case d.Status != Pending:
		st.mu.Unlock()
		return Draft{}, fmt.Errorf("drafts: %s was withdrawn", id)
	case d.Decision != "":
		st.mu.Unlock()
		return Draft{}, fmt.Errorf("drafts: %s is already %s", id, d.Decision)
	}
	if st.keeping == nil {
		st.keeping = map[string]bool{}
	}
	st.keeping[id] = true
	snapshot := *d
	st.mu.Unlock()

	noteID, edited, err := write(snapshot)

	st.mu.Lock()
	defer st.mu.Unlock()
	delete(st.keeping, id)
	if err != nil {
		return Draft{}, err
	}
	d.Decision, d.NoteID, d.Edited = Kept, noteID, edited || d.Edited
	d.Updated = st.now()
	return *d, nil
}

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
		case st.keeping[id]:
			return Draft{}, ErrKeeping
		case d.Status != Pending:
			return Draft{}, fmt.Errorf("drafts: %s was withdrawn", id)
		case d.Decision != "":
			return Draft{}, fmt.Errorf("drafts: %s is already %s", id, d.Decision)
		}
		d.Decision, d.NoteID, d.Edited = decision, noteID, edited || d.Edited
		d.Updated = st.now()
		return *d, nil
	}
	return Draft{}, fmt.Errorf("drafts: no draft %s", id)
}

// Undecide takes back the person's decision on a draft, around remove,
// which deletes a kept draft's pending comment on the forge. The draft is
// held while remove runs, as for Keep. A decision carried from an earlier
// review, or one whose fix was pushed or suggested, stands. It is vuoro's
// call, made from the UI, and no MCP tool reaches it.
func (st *Store) Undecide(id string, remove func(Draft) error) (Draft, error) {
	st.mu.Lock()
	d := st.find(id)
	switch {
	case d == nil:
		st.mu.Unlock()
		return Draft{}, fmt.Errorf("drafts: no draft %s", id)
	case st.keeping[id]:
		st.mu.Unlock()
		return Draft{}, ErrKeeping
	case d.Status != Pending || d.Decision == "":
		st.mu.Unlock()
		return Draft{}, fmt.Errorf("drafts: %s has no decision to take back", id)
	case d.Carried:
		st.mu.Unlock()
		return Draft{}, fmt.Errorf("drafts: %s was decided in the earlier review, where its comment went", id)
	case d.FixedIn != "" || d.Suggested != nil:
		st.mu.Unlock()
		return Draft{}, fmt.Errorf("drafts: %s already has its fix, so its decision stands", id)
	}
	if st.keeping == nil {
		st.keeping = map[string]bool{}
	}
	st.keeping[id] = true
	snapshot := *d
	st.mu.Unlock()

	err := remove(snapshot)

	st.mu.Lock()
	defer st.mu.Unlock()
	delete(st.keeping, id)
	if err != nil {
		return Draft{}, err
	}
	d.Decision, d.NoteID, d.Edited = "", "", false
	d.Updated = st.now()
	return *d, nil
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
	if s.Role == Summary || s.Role == Editor {
		return nil
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	var out []Draft
	for _, d := range st.drafts {
		if (s.Role == Reviewer || s.Role == Triage) && d.Session != s.ID || s.Role == Suggester && d.ID != s.Target {
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
		case d.ThreadID != "" && d.Edited:
			return nil, &RuleError{"draft.edited", "id", "a reply the person has not edited"}
		case d.FixedIn != "":
			return nil, &RuleError{"draft.fixed", "id", "a reply whose thread is not yet fixed by a pushed commit"}
		case st.keeping[d.ID]:
			return nil, keepingErr
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
	d.ThreadID, d.Category, d.Reason = p.ThreadID, p.Category, strings.TrimSpace(p.Reason)
	d.Anchor, d.Sources, d.Suggestion = nil, slices.Clone(p.Sources), nil
	d.applyProject(p)
	if p.Suggestion != nil {
		s := strings.TrimRight(*p.Suggestion, "\n")
		d.Suggestion = &s
	}
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

// Fence is a code fence longer than any run of backticks in s, so s cannot
// close it early.
func Fence(s string) string {
	n, run := 3, 0
	for _, r := range s {
		if r == '`' {
			run++
			n = max(n, run+1)
		} else {
			run = 0
		}
	}
	return strings.Repeat("`", n)
}

// SuggestionBlock is d's suggestion as the forge's suggestion block, for a
// comment placed on the anchor's last line, as vuoro places one, or, where
// span is set, spanning the anchor's range (D59). It replaces every line of
// the range. It is empty when d has no suggestion.
func SuggestionBlock(d Draft, span bool) string {
	if d.Suggestion == nil || d.Anchor == nil || d.Anchor.Side != Head {
		return ""
	}
	return Suggestion(*d.Suggestion, d.Anchor.EndLine-d.Anchor.StartLine, 0, span)
}

// Suggestion is a suggestion block replacing the lines from above lines
// before its comment's line to below lines after it, with text: GitLab's
// block, which carries those offsets, or, where span is set, GitHub's,
// which replaces the lines its comment spans.
func Suggestion(text string, above, below int, span bool) string {
	f := Fence(text)
	if span {
		return fmt.Sprintf("%ssuggestion\n%s\n%s", f, text, f)
	}
	return fmt.Sprintf("%ssuggestion:-%d+%d\n%s\n%s", f, above, below, text, f)
}

// ReplacementBlock is d's suggestion for a forge that cannot apply one: the
// replacement in a plain fence, under a line naming the lines it replaces
// (D43). It is empty when d has no suggestion.
func ReplacementBlock(d Draft) string {
	if d.Suggestion == nil || d.Anchor == nil || d.Anchor.Side != Head {
		return ""
	}
	lines := fmt.Sprintf("line %d", d.Anchor.EndLine)
	if d.Anchor.StartLine < d.Anchor.EndLine {
		lines = fmt.Sprintf("lines %d–%d", d.Anchor.StartLine, d.Anchor.EndLine)
	}
	f := Fence(*d.Suggestion)
	return fmt.Sprintf("Suggested replacement for %s:\n\n%s\n%s\n%s", lines, f, *d.Suggestion, f)
}
