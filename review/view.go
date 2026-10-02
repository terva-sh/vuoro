package review

import (
	"time"

	"github.com/terva-sh/vuoro/agent"
	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/review/drafts"
	"github.com/terva-sh/vuoro/state"
)

// RunView is a run as the person sees it, and as runs/<id>/run.json keeps it.
type RunView struct {
	ID       string         `json:"id"`
	Ref      forge.Ref      `json:"ref"`
	Title    string         `json:"title"`
	WebURL   string         `json:"web_url"`
	Head     string         `json:"head"`
	Base     string         `json:"base"`
	Created  time.Time      `json:"created"`
	Status   Status         `json:"status"`
	Error    string         `json:"error,omitempty"`
	CostUSD  float64        `json:"cost_usd"`
	Sessions []SessionView  `json:"sessions"`
	Drafts   []drafts.Draft `json:"drafts"`
}

// SessionView is one session as the person sees it.
type SessionView struct {
	ID      string             `json:"id"`
	Profile string             `json:"profile"`
	Tool    string             `json:"tool"`
	Model   string             `json:"model,omitempty"`
	Role    string             `json:"role"`
	Status  Status             `json:"status"`
	Error   string             `json:"error,omitempty"`
	Info    *agent.SessionInfo `json:"info,omitempty"`
	Usage   agent.TurnUsage    `json:"usage"`
	Log     []Entry            `json:"log"`
}

// View returns a copy of the run's state.
func (r *Run) View() RunView {
	r.mu.Lock()
	defer r.mu.Unlock()
	v := RunView{
		ID: r.ID, Ref: r.Ref, Title: r.Title, WebURL: r.WebURL, Head: r.Head, Base: r.Base,
		Created: r.Created, Status: r.Status, Error: r.Error,
		Drafts: r.store.All(),
	}
	for _, s := range r.sessions {
		role := "reviewer"
		if s.Role == drafts.Synthesizer {
			role = "synthesizer"
		}
		v.CostUSD += s.Usage.CostUSD
		v.Sessions = append(v.Sessions, SessionView{
			ID: s.ID, Profile: s.Profile, Tool: s.Tool, Model: s.Model, Role: role,
			Status: s.Status, Error: s.Error, Info: s.Info, Usage: s.Usage,
			Log: append([]Entry(nil), s.Log...),
		})
	}
	if v.Drafts == nil {
		v.Drafts = []drafts.Draft{}
	}
	return v
}

// save writes the run record. A failure to write costs the record, not the
// run, so it is not reported.
func (r *Run) save() {
	r.saveMu.Lock()
	defer r.saveMu.Unlock()
	_ = state.WriteJSON(r.path, r.View())
}
