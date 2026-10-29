package drafts

import "slices"

// Earlier is a finding of the review a run follows, as its reviewers are
// told of it (D50). Standing is where it stood when the run started: fixed,
// dismissed, kept (a comment the person posted, not yet fixed), or open.
type Earlier struct {
	ID           string   `json:"id"`
	Severity     Severity `json:"severity"`
	Title        string   `json:"title"`
	Anchor       *Anchor  `json:"anchor,omitempty"`
	Standing     string   `json:"standing"`
	FixedIn      string   `json:"fixed_in,omitempty"`
	FixedSubject string   `json:"fixed_subject,omitempty"`
	Decision     string   `json:"decision,omitempty"`
	NoteID       string   `json:"note_id,omitempty"`
	// Reason is why the person dismissed it, when they said.
	Reason string `json:"reason,omitempty"`
}

// Standings of an earlier finding.
const (
	StandingFixed     = "fixed"
	StandingDismissed = "dismissed"
	StandingKept      = "kept"
	StandingOpen      = "open"
)

// EarlierOf reads the findings of an earlier review, with where each stands.
// Triage replies are not findings, and are left out.
func EarlierOf(shown []Draft) []Earlier {
	var out []Earlier
	for _, d := range shown {
		if d.ThreadID != "" {
			continue
		}
		e := Earlier{ID: d.ID, Severity: d.Severity, Title: d.Title, FixedIn: d.FixedIn, FixedSubject: d.FixedSubject, Decision: d.Decision, NoteID: d.NoteID}
		if d.Anchor != nil {
			a := *d.Anchor
			e.Anchor = &a
		}
		switch {
		case d.FixedIn != "":
			e.Standing = StandingFixed
		case d.Decision == Dropped:
			e.Standing, e.Reason = StandingDismissed, d.Dismissed
		case d.Decision == Kept:
			e.Standing = StandingKept
		default:
			e.Standing = StandingOpen
		}
		out = append(out, e)
	}
	return out
}

// SetEarlier gives the store the findings of the review its run follows.
// Set it before any session starts.
func (st *Store) SetEarlier(list []Earlier) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.earlier = slices.Clone(list)
}

// Earlier returns the findings of the review the store's run follows.
func (st *Store) Earlier() []Earlier {
	st.mu.Lock()
	defer st.mu.Unlock()
	return slices.Clone(st.earlier)
}

// carry names the earlier finding a draft restates, and gives the draft that
// finding's decision, so a finding the person dismissed is not raised to
// them again as new (D50). A synthesizer's draft that names none takes the
// one its sources name. st.mu is held, and d is undecided.
func (st *Store) carry(d *Draft, p Proposal) error {
	id := p.Earlier
	if id == "" {
		for _, src := range p.Sources {
			if s := st.find(src); s != nil && s.Earlier != "" {
				id = s.Earlier
				break
			}
		}
	}
	d.Earlier, d.Carried = "", false
	if id == "" {
		return nil
	}
	i := slices.IndexFunc(st.earlier, func(e Earlier) bool { return e.ID == id })
	if i < 0 {
		return &RuleError{"earlier.unknown", "earlier", "the id of an earlier finding listed in the first message, or none"}
	}
	e := st.earlier[i]
	d.Earlier = e.ID
	if e.Decision != "" {
		d.Decision, d.NoteID, d.Carried = e.Decision, e.NoteID, true
		if e.Decision == Dropped {
			d.Dismissed = e.Reason
		}
	}
	return nil
}
