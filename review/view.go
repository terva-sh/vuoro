package review

import (
	"path/filepath"
	"time"

	"github.com/terva-sh/vuoro/agent"
	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/review/drafts"
	"github.com/terva-sh/vuoro/state"
)

// RunView is a run as the person sees it, and as runs/<id>/run.json keeps it.
type RunView struct {
	ID             string         `json:"id"`
	Kind           string         `json:"kind"`
	Mode           string         `json:"mode,omitempty"`
	Summary        *SummaryRange  `json:"summary,omitempty"`
	ReviewComplete bool           `json:"review_complete,omitempty"`
	Threads        []forge.Thread `json:"triage_threads,omitempty"`
	Ref            forge.Ref      `json:"ref"`
	Title          string         `json:"title"`
	WebURL         string         `json:"web_url"`
	Head           string         `json:"head"`
	Base           string         `json:"base"`
	Created        time.Time      `json:"created"`
	Status         Status         `json:"status"`
	Error          string         `json:"error,omitempty"`
	CostUSD        float64        `json:"cost_usd"`
	Sessions       []SessionView  `json:"sessions"`
	Drafts         []drafts.Draft `json:"drafts"`
	Editor         *EditorState   `json:"editor,omitempty"`
}

// SessionView is one session as the person sees it.
type SessionView struct {
	ID            string             `json:"id"`
	Profile       string             `json:"profile"`
	Tool          string             `json:"tool"`
	Model         string             `json:"model,omitempty"`
	Role          string             `json:"role"`
	Status        Status             `json:"status"`
	Error         string             `json:"error,omitempty"`
	Info          *agent.SessionInfo `json:"info,omitempty"`
	Usage         agent.TurnUsage    `json:"usage"`
	Log           []Entry            `json:"log"`
	FirstComplete bool               `json:"first_turn_complete,omitempty"`
	FirstReported bool               `json:"first_turn_reported,omitempty"`
	Target        string             `json:"target,omitempty"` // a suggestion session's draft
}

// View returns a copy of the run's state.
func (r *Run) View() RunView {
	r.mu.Lock()
	defer r.mu.Unlock()
	v := RunView{
		ID: r.ID, Kind: r.Kind, Mode: r.Mode, Ref: r.Ref, Title: r.Title, WebURL: r.WebURL, Head: r.Head, Base: r.Base,
		Created: r.Created, Status: r.Status, Error: r.Error,
		Drafts: r.store.All(),
	}
	if r.editor != nil {
		e := *r.editor
		if e.Pushed != nil {
			p := *e.Pushed
			e.Pushed = &p
		}
		v.Editor = &e
	}
	if r.Summary != nil {
		sr := *r.Summary
		v.Summary = &sr
	}
	v.ReviewComplete = r.Mode == "" && len(r.sessions) > 0
	for _, t := range r.Threads {
		v.Threads = append(v.Threads, cleanThread(t))
	}
	for _, s := range r.sessions {
		// A suggestion or an editor asked for later is not part of the review.
		v.ReviewComplete = v.ReviewComplete && (s.FirstComplete || s.Role == drafts.Suggester || s.Role == drafts.Editor)
		role := roleNames[s.Role]
		v.CostUSD += s.Usage.CostUSD
		v.Sessions = append(v.Sessions, SessionView{
			ID: s.ID, Profile: s.Profile, Tool: s.Tool, Model: s.Model, Role: role,
			Status: s.Status, Error: s.Error, Info: s.Info, Usage: s.Usage, FirstComplete: s.FirstComplete, FirstReported: s.reported, Target: s.Target,
			Log: append([]Entry(nil), s.Log...),
		})
	}
	if v.Drafts == nil {
		v.Drafts = []drafts.Draft{}
	}
	return v
}

// roleNames are the roles as the view and the record name them.
var roleNames = map[drafts.Role]string{drafts.Reviewer: "reviewer", drafts.Synthesizer: "synthesizer", drafts.Verifier: "verifier", drafts.Triage: "triage", drafts.Summary: "summary", drafts.Suggester: "suggester", drafts.Editor: "editor"}

// record is what runs/<id>/run.json holds: the view, and what a reloaded run
// needs to carry on.
type record struct {
	RunView
	First   string            `json:"first_message,omitempty"`
	Repo    string            `json:"repo"`
	Anchors lineIndex         `json:"anchors"`
	Resume  map[string]string `json:"resume"` // each session's resume handle
	Who     *Identity         `json:"editor_identity,omitempty"`
}

func (r *Run) record() record {
	rec := record{RunView: r.View()}
	r.mu.Lock()
	defer r.mu.Unlock()
	rec.First = r.first
	rec.Repo, rec.Anchors, rec.Resume = r.repoKey, r.anchors, map[string]string{}
	for _, s := range r.sessions {
		rec.Resume[s.ID] = s.Ref
	}
	if r.editor != nil {
		who := r.who
		rec.Who = &who
	}
	return rec
}

// save writes the run record. A failure to write costs the record, not the
// run, so it is not reported.
func (r *Run) save() {
	r.saveMu.Lock()
	defer r.saveMu.Unlock()
	_ = state.WriteJSON(r.path, r.record())
}

// Load reads every run record under the state directory, so the person's
// runs, drafts, and conversations outlive a restart. A session comes back
// ended, and talking to it resumes it.
func (m *Manager) Load() error {
	paths, err := filepath.Glob(filepath.Join(m.StateDir, "runs", "*", "run.json"))
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.runs == nil {
		m.runs = map[string]*Run{}
	}
	for _, path := range paths {
		var rec record
		if err := state.ReadJSON(path, &rec); err != nil || rec.ID == "" || m.runs[rec.ID] != nil {
			continue
		}
		r := &Run{
			ID: rec.ID, Kind: rec.Kind, Mode: rec.Mode, Threads: rec.Threads, first: rec.First, Ref: rec.Ref, Title: rec.Title, WebURL: rec.WebURL, Head: rec.Head, Base: rec.Base,
			Created: rec.Created, Status: rec.Status, Error: rec.Error, Summary: rec.Summary,
			dir:     filepath.Join(m.StateDir, "worktrees", rec.ID),
			repoKey: rec.Repo,
			anchors: rec.Anchors,
			store:   drafts.Restore(rec.Drafts),
			path:    path,
			editor:  rec.Editor,
		}
		if rec.Who != nil {
			r.who = *rec.Who
		}
		if r.Mode == "triage" {
			r.bindThreads()
		}
		if r.anchors != nil {
			r.store.CheckAnchor = r.anchors.check
		}
		r.store.CheckSuggestion = r.checkSuggestion
		for _, sv := range rec.Sessions {
			s := &Session{
				ID: sv.ID, Profile: sv.Profile, Tool: sv.Tool, Model: sv.Model, Role: drafts.Reviewer,
				Status: sv.Status, Error: sv.Error, Info: sv.Info, Usage: sv.Usage, Log: sv.Log,
				Ref: rec.Resume[sv.ID], FirstComplete: sv.FirstComplete, Target: sv.Target,
				// A legacy ended/failed session is already past its initial turn.
				reported: sv.FirstReported || sv.FirstComplete || sv.Status != Waiting,
			}
			for role, name := range roleNames {
				if sv.Role == name {
					s.Role = role
				}
			}
			if s.Status == Working || s.Status == Idle || s.Status == Preparing {
				s.Status = Ended
			}
			switch p, ok := m.profile(s.Profile); {
			case s.Role == drafts.Triage:
				s.spec = agent.Spec{SystemPrompt: triagePrompt, Model: s.Model}
			case s.Role == drafts.Synthesizer:
				s.spec = agent.Spec{SystemPrompt: synthesisPrompt(), Model: s.Model}
			case s.Role == drafts.Verifier:
				s.spec = agent.Spec{SystemPrompt: verificationPrompt(), Model: s.Model}
			case s.Role == drafts.Suggester:
				s.spec = agent.Spec{SystemPrompt: suggestionPrompt(), Model: s.Model}
			case s.Role == drafts.Editor:
				s.spec = agent.Spec{SystemPrompt: editorPrompt(), Model: s.Model}
			case ok:
				s.spec = agent.Spec{SystemPrompt: systemPrompt(p), Model: s.Model}
			}
			r.sessions = append(r.sessions, s)
		}
		switch r.Status {
		case Preparing:
			r.Status, r.Error = Failed, "vuoro stopped before the checkout finished"
		case Working:
			r.Status = Idle
		}
		if r.Mode == "summary" && (rec.Status == Preparing || rec.Status == Working) {
			r.Status, r.Error = Failed, "vuoro stopped before the summary completed"
			for _, s := range r.sessions {
				s.Status, s.Error = Failed, r.Error
			}
		}
		m.runs[r.ID] = r
	}
	return nil
}
