package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/projects"
	"github.com/terva-sh/vuoro/review"
	"github.com/terva-sh/vuoro/review/drafts"
)

// Agent triage over a project (docs/plan.md section 6.9, D65). A read-only
// session drafts proposals from facts vuoro supplies. Accepting one is the
// person's action here, and this code, not the agent, makes the writes.
func (a *App) projectTriageRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/projects/{key}/triage", a.startProjectTriage)
	mux.HandleFunc("GET /api/projects/{key}/triage", a.getProjectTriage)
	mux.HandleFunc("POST /api/projects/{key}/triage/{run}/accept", a.acceptProposals)
	mux.HandleFunc("POST /api/projects/{key}/triage/{run}/discard", a.discardProposal)
}

// maxTriaged bounds the changes one triage run is given. A person narrows a
// larger project with the view's filters.
const maxTriaged = 60

// maxTriageDirs bounds the directories whose recent committers are read.
const maxTriageDirs = 20

// changeStamp fingerprints what a proposal rests on. A change whose stamp
// moved since the agent read it makes its proposal stale.
func changeStamp(ch forge.Change) string {
	reviewers := []string{}
	for _, r := range ch.Reviewers {
		reviewers = append(reviewers, r.Username+":"+string(r.State))
	}
	labels := slices.Clone(ch.Labels)
	sort.Strings(reviewers)
	sort.Strings(labels)
	b, _ := json.Marshal([]any{ch.Diff.Head, ch.Draft, ch.State, reviewers, labels})
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:8])
}

type triageFacts struct {
	Project    string              `json:"project"`
	Target     string              `json:"target_branch"`
	CodeOwners string              `json:"codeowners,omitempty"`
	Committers map[string][]string `json:"recent_committers_by_directory,omitempty"`
	Candidates []triageCandidate   `json:"candidates"`
	Labels     []string            `json:"project_labels"`
	Changes    []triageChange      `json:"changes"`
	LeftOut    int                 `json:"changes_left_out,omitempty"`
}

type triageCandidate struct {
	Username string `json:"username"`
	Name     string `json:"name"`
	// Reviewing counts the project's open changes waiting on their review.
	Reviewing int `json:"reviewing"`
}

type triageChange struct {
	Number    int                    `json:"number"`
	Title     string                 `json:"title"`
	Author    string                 `json:"author"`
	Place     projects.Place         `json:"where_it_sits"`
	Since     *time.Time             `json:"since,omitempty"`
	Opened    *time.Time             `json:"opened,omitempty"`
	Reasons   []string               `json:"reasons"`
	Reviewers []projects.RowReviewer `json:"reviewers"`
	Assignees []string               `json:"assignees"`
	Labels    []string               `json:"labels"`
	Pipeline  string                 `json:"pipeline,omitempty"`
	Conflicts bool                   `json:"conflicts,omitempty"`
	Threads   projects.Threads       `json:"threads"`
	On        []int                  `json:"stacked_on,omitempty"`
	Paths     []forge.Touched        `json:"paths"`
	MorePaths int                    `json:"more_paths,omitempty"`
}

// placeOrder is which changes a triage run takes first when there are more
// than it can be given: those that most need routing.
var placeOrder = map[projects.Place]int{projects.NoReviewers: 0, projects.WaitingReview: 1, projects.Blocked: 2, projects.WaitingAuthor: 3, projects.Approved: 4}

func (a *App) startProjectTriage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Numbers []int  `json:"numbers,omitempty"`
		Tool    string `json:"tool,omitempty"`
		Model   string `json:"model,omitempty"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if a.Runs == nil {
		writeError(w, errors.New("Agent triage is unavailable."))
		return
	}
	_, snap, c, ad, err := a.admin(r)
	if err != nil {
		writeError(w, err)
		return
	}
	in, err := a.triageInput(r.Context(), snap, c, ad, req.Numbers)
	if err != nil {
		writeError(w, err)
		return
	}
	a.probeTools()
	run, err := a.Runs.StartProjectTriage(in, review.Reviewer{Tool: req.Tool, Model: req.Model})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, run.View())
}

// triageInput builds what the agent reads. Every fact comes from the
// snapshot or the forge, read by vuoro; the agent reads nothing itself.
func (a *App) triageInput(ctx context.Context, snap projects.Snapshot, c forge.Provider, ad forge.Administrator, numbers []int) (review.ProjectTriageInput, error) {
	view := snap.Display()
	byNumber := map[int]forge.Change{}
	for _, item := range snap.Items {
		byNumber[item.Change.Ref.Number] = item.Change
	}
	var rows []projects.Row
	reviewing := map[string]int{}
	for _, g := range view.Groups {
		for _, row := range g.Rows {
			if row.Place == projects.WaitingReview {
				for _, u := range row.Holder {
					reviewing[strings.ToLower(u)]++
				}
			}
			if row.Place == projects.Draft || row.Error != "" || (len(numbers) > 0 && !slices.Contains(numbers, row.Ref.Number)) {
				continue
			}
			rows = append(rows, row)
		}
	}
	if len(rows) == 0 {
		return review.ProjectTriageInput{}, errors.New("no open change here needs triage: drafts and changes that could not be read are left out")
	}
	slices.SortStableFunc(rows, func(x, y projects.Row) int { return placeOrder[x.Place] - placeOrder[y.Place] })
	facts := triageFacts{Project: snap.Project.Path, Committers: map[string][]string{}}
	if len(rows) > maxTriaged {
		facts.LeftOut = len(rows) - maxTriaged
		rows = rows[:maxTriaged]
	}

	members, err := ad.Members(ctx, snap.Project.Path, "")
	if err != nil {
		return review.ProjectTriageInput{}, fmt.Errorf("reading the project's members: %w", err)
	}
	labels, err := ad.Labels(ctx, snap.Project.Path)
	if err != nil {
		return review.ProjectTriageInput{}, fmt.Errorf("reading the project's labels: %w", err)
	}
	triage := review.ProjectTriage{Path: snap.Project.Path, Candidates: []string{}, Labels: labels}
	for _, m := range members {
		facts.Candidates = append(facts.Candidates, triageCandidate{Username: m.Username, Name: m.Name, Reviewing: reviewing[strings.ToLower(m.Username)]})
		triage.Candidates = append(triage.Candidates, m.Username)
	}
	facts.Labels = labels

	targets := map[string]int{}
	dirs := map[string]int{}
	for _, row := range rows {
		ch := byNumber[row.Ref.Number]
		targets[ch.TargetBranch]++
		tc := triageChange{Number: row.Ref.Number, Title: row.Title, Author: row.Author, Place: row.Place, Since: row.Since, Opened: row.Opened,
			Reviewers: row.Reviewers, Assignees: row.Assignees, Labels: row.Labels, Pipeline: row.Pipeline, Conflicts: row.Conflicts,
			Threads: row.Threads, Paths: ch.Touched}
		if tc.Paths == nil {
			tc.Paths = []forge.Touched{}
		}
		if len(tc.Paths) > 40 {
			tc.MorePaths, tc.Paths = len(tc.Paths)-40, tc.Paths[:40]
		}
		for _, reason := range row.Reasons {
			tc.Reasons = append(tc.Reasons, reason.Text)
		}
		if row.Stack != nil {
			tc.On = row.Stack.On
		}
		for _, t := range ch.Touched {
			dirs[path.Dir(t.Path)]++
		}
		facts.Changes = append(facts.Changes, tc)
		triage.Changes = append(triage.Changes, review.ProjectChange{Number: row.Ref.Number, Stamp: changeStamp(ch)})
	}
	for branch, n := range targets {
		if n > targets[facts.Target] || (n == targets[facts.Target] && branch < facts.Target) {
			facts.Target = branch
		}
	}
	if tr, ok := c.(forge.TriageReader); ok && facts.Target != "" {
		// Code owners and recent committers help, and a triage run without
		// them is still worth having, so a failed read leaves them out.
		if text, err := tr.CodeOwners(ctx, snap.Project.Path, facts.Target); err == nil {
			facts.CodeOwners = text
		}
		ranked := make([]string, 0, len(dirs))
		for d := range dirs {
			ranked = append(ranked, d)
		}
		slices.SortFunc(ranked, func(x, y string) int {
			if dirs[x] != dirs[y] {
				return dirs[y] - dirs[x]
			}
			return strings.Compare(x, y)
		})
		for _, d := range ranked[:min(len(ranked), maxTriageDirs)] {
			if names, err := tr.RecentAuthors(ctx, snap.Project.Path, facts.Target, d); err == nil && len(names) > 0 {
				facts.Committers[d] = names
			}
		}
	}
	b, _ := json.MarshalIndent(facts, "", " ")
	return review.ProjectTriageInput{Kind: snap.Project.Kind, Host: snap.Project.Host, Name: snap.Project.Name, WebURL: snap.Project.WebURL,
		Triage: triage, Facts: string(b)}, nil
}

// getProjectTriage answers the project's newest triage run, or null.
func (a *App) getProjectTriage(w http.ResponseWriter, r *http.Request) {
	s, err := a.projects()
	if err != nil {
		writeError(w, err)
		return
	}
	snap, ok := s.Snapshot(r.PathValue("key"))
	if !ok {
		writeError(w, errNoProject)
		return
	}
	if a.Runs == nil {
		writeJSON(w, http.StatusOK, nil)
		return
	}
	if runs := a.Runs.ProjectRuns(snap.Project.Host, snap.Project.Path); len(runs) > 0 {
		writeJSON(w, http.StatusOK, runs[0].View())
		return
	}
	writeJSON(w, http.StatusOK, nil)
}

// projectRun is a project triage run on this project.
func (a *App) projectRun(r *http.Request, snap projects.Snapshot) (*review.Run, error) {
	if a.Runs == nil {
		return nil, review.ErrNoRun
	}
	run := a.Runs.Get(r.PathValue("run"))
	if run == nil || run.Mode != review.ProjectMode || !strings.EqualFold(run.Ref.Host, snap.Project.Host) || !strings.EqualFold(run.Ref.Project, snap.Project.Path) {
		return nil, review.ErrNoRun
	}
	return run, nil
}

// Outcome is what accepting one proposal came to.
type Outcome struct {
	ID string `json:"id"`
	// Result is accepted, unchanged (nothing left to add), stale (the
	// change moved since the agent read it), or failed.
	Result string `json:"result"`
	Error  string `json:"error,omitempty"`
}

// acceptProposals is the person accepting proposals, after one confirmation
// in the window. Each is applied by this code through forge.Administrator,
// with only additions, and only once the change is read fresh and found as
// the agent saw it, unless the person, having seen it is stale, says so.
func (a *App) acceptProposals(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Proposals []struct {
			ID string `json:"id"`
			// Reviewers and Labels are the person's edits to the proposal;
			// nil keeps the agent's.
			Reviewers []string `json:"reviewers,omitempty"`
			Labels    []string `json:"labels,omitempty"`
			// Stale accepts a proposal the person has seen is stale.
			Stale bool `json:"stale,omitempty"`
		} `json:"proposals"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if len(req.Proposals) == 0 {
		writeError(w, errors.New("choose a proposal to accept"))
		return
	}
	s, snap, c, ad, err := a.admin(r)
	if err != nil {
		writeError(w, err)
		return
	}
	run, err := a.projectRun(r, snap)
	if err != nil {
		writeError(w, err)
		return
	}
	caps := c.Capabilities()
	outcomes := []Outcome{}
	for _, p := range req.Proposals {
		o := Outcome{ID: p.ID}
		err := a.Runs.AcceptProposal(run.ID, p.ID, func(d drafts.Draft) error {
			reviewers, labels := d.Reviewers, d.Labels
			if p.Reviewers != nil {
				reviewers = p.Reviewers
			}
			if p.Labels != nil {
				labels = p.Labels
			}
			ref := forge.Ref{Host: snap.Project.Host, Project: snap.Project.Path, Number: d.Change}
			if err := s.RefreshChange(r.Context(), ref); err != nil {
				return err
			}
			fresh, _ := s.Snapshot(snap.Project.Key)
			i := slices.IndexFunc(fresh.Items, func(it projects.Item) bool { return it.Change.Ref.Number == d.Change })
			if i < 0 {
				return fmt.Errorf("!%d is no longer open: %w", d.Change, forge.ErrNotFound)
			}
			ch := fresh.Items[i].Change
			if fresh.Items[i].Error != "" {
				return errors.New(fresh.Items[i].Error)
			}
			if changeStamp(ch) != run.Stamp(d.Change) && !p.Stale {
				return errStale
			}
			changes := forge.Changes{}
			for _, u := range reviewers {
				if !strings.EqualFold(u, ch.Author.Username) && !slices.ContainsFunc(ch.Reviewers, func(x forge.Reviewer) bool { return strings.EqualFold(x.Username, u) }) {
					changes.AddReviewers = append(changes.AddReviewers, u)
				}
			}
			for _, l := range labels {
				if !slices.Contains(ch.Labels, l) {
					changes.AddLabels = append(changes.AddLabels, l)
				}
			}
			if changes.Empty() {
				o.Result = "unchanged"
				return nil
			}
			if err := firstErr(
				allowed(caps.Reviewers || len(changes.AddReviewers) == 0, "changing reviewers"),
				allowed(caps.Labels || len(changes.AddLabels) == 0, "changing labels"),
			); err != nil {
				return err
			}
			if err := ad.Administer(r.Context(), ref, changes); err != nil {
				// A write that landed in part changed the change; show it.
				_ = s.RefreshChange(context.WithoutCancel(r.Context()), ref)
				return err
			}
			_ = s.RefreshChange(r.Context(), ref)
			if service, err := a.inbox(); err == nil {
				a.background.Go(func() { _ = service.RefreshChange(context.WithoutCancel(r.Context()), ref) })
			}
			o.Result = "accepted"
			return nil
		})
		switch {
		case errors.Is(err, errStale):
			o.Result = "stale"
		case err != nil:
			o.Result, o.Error = "failed", err.Error()
		}
		outcomes = append(outcomes, o)
	}
	snap, _ = s.Snapshot(snap.Project.Key)
	writeJSON(w, http.StatusOK, map[string]any{"view": snap.Display(), "run": run.View(), "outcomes": outcomes})
}

var errStale = errors.New("the change moved since the agent read it")

func (a *App) discardProposal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	s, err := a.projects()
	if err != nil {
		writeError(w, err)
		return
	}
	snap, ok := s.Snapshot(r.PathValue("key"))
	if !ok {
		writeError(w, errNoProject)
		return
	}
	run, err := a.projectRun(r, snap)
	if err != nil {
		writeError(w, err)
		return
	}
	d, _, err := a.Runs.Draft(run.ID, req.ID)
	if err != nil || d.Change == 0 {
		writeError(w, review.ErrNoRun)
		return
	}
	if err := a.Runs.Decide(run.ID, d.ID, drafts.Dropped, "", false); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, run.View())
}

// projectRunRoutes are the generic run routes a project triage run answers:
// reading it, closing it, and talking to its session. The rest act on a
// change's review, and a proposal is accepted only through acceptProposals.
var projectRunRoutes = map[string]bool{"": true, "close": true, "sessions": true}

// updateRunRoutes are the generic run routes an update run answers. The rest
// act on a review, and its editor is driven only through the update routes,
// so its work is never pushed as a stacked change.
var updateRunRoutes = map[string]bool{"": true, "close": true, "sessions": true, "update": true}

// keepProjectRuns refuses the generic run routes that a project triage run,
// or an update run, does not answer.
func (a *App) keepProjectRuns(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rest, ok := strings.CutPrefix(r.URL.Path, "/api/runs/"); ok && a.Runs != nil {
			id, sub, _ := strings.Cut(rest, "/")
			sub, _, _ = strings.Cut(sub, "/")
			run := a.Runs.Get(id)
			if run != nil && (run.Mode == review.ProjectMode && !projectRunRoutes[sub] || run.Mode == review.UpdateMode && !updateRunRoutes[sub]) {
				writeError(w, review.ErrNoRun)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
