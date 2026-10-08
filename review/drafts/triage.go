package drafts

import (
	"slices"
	"strings"
	"time"
)

type Category string

const (
	MustFix Category = "must_fix"
	Discuss Category = "discuss"
	Nit     Category = "nit"
)

var categories = []Category{MustFix, Discuss, Nit}

func validateTriage(p Proposal) error {
	switch {
	case p.ThreadID == "" || len(p.ThreadID) > 512:
		return &RuleError{"thread.id", "thread_id", "the id of an open thread in this run"}
	case !slices.Contains(categories, p.Category):
		return &RuleError{"triage.category", "category", "must_fix, discuss, or nit"}
	case strings.TrimSpace(p.Reason) == "" || len(p.Reason) > maxReason:
		return &RuleError{"triage.reason", "reason", "a reason of 1 to 512 bytes"}
	case len(p.Description) > maxText:
		return &RuleError{"reply.length", "description", "a reply of at most 20000 bytes"}
	case p.Anchor != nil || p.Suggestion != nil || len(p.Sources) > 0 || p.Severity != "" || p.Title != "" || p.Evidence != "" || p.Earlier != "":
		return &RuleError{"triage.shape", "arguments", "thread_id, category, reason and optional description only"}
	}
	return nil
}

// EditReply saves the person's text and category locally. Agents cannot replace
// a person-edited reply; no MCP tool reaches this method.
func (st *Store) EditReply(id, body string, category Category) (Draft, error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	d := st.find(id)
	if d == nil || d.ThreadID == "" || d.Status != Pending || d.Decision != "" {
		return Draft{}, &RuleError{"reply.pending", "id", "a pending triage reply"}
	}
	if len(body) > maxText || !slices.Contains(categories, category) {
		return Draft{}, &RuleError{"reply.edit", "arguments", "a valid category and reply of at most 20000 bytes"}
	}
	d.Description = body
	d.Category = category
	d.Edited = true
	d.Revision++
	d.Updated = st.now()
	return *d, nil
}

// AcceptReply serializes a person's acceptance against agent revision, editing,
// discard and another acceptance. The callback is supplied only by the UI API.
func (st *Store) AcceptReply(id string, accept func(Draft) (string, bool, error)) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	d := st.find(id)
	if d == nil || d.ThreadID == "" || d.Status != Pending || d.Decision != "" {
		return &RuleError{"reply.pending", "id", "a pending triage reply"}
	}
	noteID, edited, err := accept(*d)
	if err != nil {
		return err
	}
	d.Decision, d.NoteID, d.Edited = Kept, noteID, edited || d.Edited
	d.Updated = st.now()
	return nil
}

// CiteFix records that a pushed commit fixes a draft: a reply's thread
// (D41), or a finding (D42, D50). by is "editor", when the commit's
// trailer names the draft, or "person". A thread's reply then names the
// commit, unless the person edited it, whose words stand. A reply already
// accepted keeps its text, since the forge holds it. No MCP tool reaches
// this method, and the agent can no longer revise the draft afterwards.
func (st *Store) CiteFix(id, sha, subject, by, body string) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	d := st.find(id)
	if d == nil || d.Status != Pending {
		return &RuleError{"draft.pending", "id", "a pending draft"}
	}
	d.FixedIn, d.FixedSubject, d.FixedBy = sha, subject, by
	// A finding has no reply to rewrite; only a thread's does.
	if d.ThreadID != "" && !d.Edited && d.Decision == "" {
		d.Description = body
	}
	d.Revision++
	d.Updated = st.now()
	return nil
}

// SuggestedFix is an editor's commit sent back as suggestions (D59): the
// commit, and each suggestion's lines at the reviewed head.
type SuggestedFix struct {
	Commit  string          `json:"commit"`
	Subject string          `json:"subject"`
	At      time.Time       `json:"at"`
	Hunks   []SuggestedHunk `json:"hunks,omitempty"`
	// Misfits counts the changes that went in the comment as a plain diff,
	// an attached patch, or a note, rather than as suggestions.
	Misfits int `json:"misfits,omitempty"`
}

// SuggestedHunk is one suggestion: it replaces lines Start to End of Path at
// the reviewed head with Text. Above and Below are the head's lines just
// outside them, when there are any, which tell an applied suggestion from
// the same text elsewhere. NoteID is the pending comment that carries it.
type SuggestedHunk struct {
	Path   string  `json:"path"`
	Start  int     `json:"start"`
	End    int     `json:"end"`
	Text   string  `json:"text"`
	Above  *string `json:"above,omitempty"`
	Below  *string `json:"below,omitempty"`
	NoteID string  `json:"note_id,omitempty"`
}

// MarkSuggested records that an editor's fix went back as suggestions on a kept
// finding's comments (D59).
func (st *Store) MarkSuggested(id string, s SuggestedFix) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	d := st.find(id)
	if d == nil || d.Status != Pending || d.Decision != Kept {
		return &RuleError{"draft.kept", "id", "a kept finding"}
	}
	d.Suggested = &s
	d.Revision++
	d.Updated = st.now()
	return nil
}

// ClearFix takes back a fix the person linked by hand, or one vuoro found
// from a suggestion's lines (D59). A fix an editor's commit names stands,
// since the commit says so.
func (st *Store) ClearFix(id string) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	d := st.find(id)
	if d == nil || d.FixedIn == "" {
		return &RuleError{"draft.fixed", "id", "a draft marked fixed"}
	}
	if d.FixedBy != "person" && d.FixedBy != "suggestion" {
		return &RuleError{"draft.fixed_by", "id", "a fix you linked by hand, or one vuoro found from a suggestion"}
	}
	d.FixedIn, d.FixedSubject, d.FixedBy = "", "", ""
	d.Revision++
	d.Updated = st.now()
	return nil
}
