package drafts

import (
	"slices"
	"strings"
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
	case p.Anchor != nil || p.Suggestion != nil || len(p.Sources) > 0 || p.Severity != "" || p.Title != "" || p.Evidence != "":
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

// CiteFix records that a pushed commit fixes a reply's thread (D41). The
// reply then names the commit, unless the person edited it, whose words
// stand. A reply already accepted keeps its text, since the forge holds it.
// No MCP tool reaches this method, and the agent can no longer revise the
// reply afterwards.
func (st *Store) CiteFix(id, sha, body string) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	d := st.find(id)
	if d == nil || d.ThreadID == "" || d.Status != Pending {
		return &RuleError{"reply.pending", "id", "a pending triage reply"}
	}
	d.FixedIn = sha
	if !d.Edited && d.Decision == "" {
		d.Description = body
	}
	d.Revision++
	d.Updated = st.now()
	return nil
}
