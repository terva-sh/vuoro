package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/terva-sh/vuoro/local"
	"github.com/terva-sh/vuoro/review"
	"github.com/terva-sh/vuoro/review/drafts"
	"github.com/terva-sh/vuoro/state"
)

// The Local tab (docs/plan.md section 6.8): the person's checkouts, each
// branch beyond its base, and how its reviews stand. Every route here reads
// checkouts and writes only local.json and its cache.

const (
	// localFresh is how long a built view is served before the next read
	// builds it again.
	localFresh = time.Minute
	// scanFresh is how long a scan of the roots stands before the tab, on
	// opening, scans again. Refresh always scans.
	scanFresh = time.Hour
	// runsPerProject bounds the runs matched to a project's branches.
	runsPerProject = 30
	// inspectAtOnce bounds the projects read at the same time.
	inspectAtOnce = 8
)

// localView is the Local tab.
type localView struct {
	Roots    []string       `json:"roots"`
	Added    []string       `json:"added"`
	Scanned  time.Time      `json:"scanned"`
	Stopped  string         `json:"stopped,omitempty"`
	Built    time.Time      `json:"built"`
	Projects []localProject `json:"projects"`
	Hidden   []localProject `json:"hidden"`
	// Unreadable are added folders that are no longer checkouts.
	Unreadable []unreadable `json:"unreadable"`
	// Needs counts the branches that need the person, for the tab's label.
	Needs int `json:"needs"`
}

type unreadable struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

type localProject struct {
	*local.Project
	Added      bool          `json:"added"` // the person added it, rather than a scan finding it
	BaseChosen bool          `json:"base_chosen"`
	Branches   []localBranch `json:"branches"`
}

type localBranch struct {
	local.Branch
	Run   *branchRun  `json:"run,omitempty"`
	Fixes []fixBranch `json:"fixes"`
	// Needs are why the branch needs the person, empty when it does not.
	Needs []string `json:"needs"`
}

// branchRun is a branch's newest review.
type branchRun struct {
	ID        string        `json:"id"`
	Status    review.Status `json:"status"`
	Head      string        `json:"head"`
	Created   time.Time     `json:"created"`
	Moved     bool          `json:"moved"`     // the branch has commits since
	Undecided int           `json:"undecided"` // findings not yet picked, dismissed, or fixed
}

// fixBranch is an editor's branch for this one, and whether it waits to be
// merged into it.
type fixBranch struct {
	Branch  string `json:"branch"`
	Waiting bool   `json:"waiting"`
}

// scanRecord is the last scan, kept in cache/local.json.
type scanRecord struct {
	Roots   []string         `json:"roots"`
	At      time.Time        `json:"at"`
	Result  local.ScanResult `json:"result"`
	Version int              `json:"version"`
}

// localState is the App's cache of the tab.
type localState struct {
	mu   sync.Mutex
	view *localView
}

func (a *App) localStorePath() string {
	if a.StateDir == "" {
		return ""
	}
	return filepath.Join(a.StateDir, "local.json")
}

func (a *App) scanPath() string {
	if a.StateDir == "" {
		return ""
	}
	return filepath.Join(a.StateDir, "cache", "local.json")
}

func (a *App) localRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/local", a.getLocal)
	mux.HandleFunc("POST /api/local/refresh", a.refreshLocal)
	mux.HandleFunc("POST /api/local/projects", a.localEdit(func(s *local.Store, req localReq) error {
		p, err := local.CleanPath(req.Path)
		if err != nil {
			return err
		}
		if _, err := local.Inspect(context.Background(), "", p, ""); err != nil {
			return err
		}
		s.AddProject(p)
		return nil
	}))
	mux.HandleFunc("POST /api/local/projects/remove", a.localEdit(func(s *local.Store, req localReq) error {
		s.RemoveProject(req.Path)
		return nil
	}))
	mux.HandleFunc("POST /api/local/roots", a.localEdit(func(s *local.Store, req localReq) error {
		p, err := local.CleanPath(req.Path)
		if err != nil {
			return err
		}
		s.AddRoot(p)
		return nil
	}))
	mux.HandleFunc("POST /api/local/roots/remove", a.localEdit(func(s *local.Store, req localReq) error {
		s.RemoveRoot(req.Path)
		return nil
	}))
	mux.HandleFunc("POST /api/local/hide", a.localEdit(func(s *local.Store, req localReq) error {
		if a.knownProject(req.Key) == nil {
			return errUnknownProject
		}
		if req.Hidden {
			s.Hide(req.Key)
		} else {
			s.Unhide(req.Key)
		}
		return nil
	}))
	mux.HandleFunc("POST /api/local/base", a.localEdit(func(s *local.Store, req localReq) error {
		known := a.knownProject(req.Key)
		if known == nil {
			return errUnknownProject
		}
		if req.Base != "" {
			p, err := local.Inspect(context.Background(), "", known.Path, req.Base)
			if err != nil {
				return err
			}
			if p.Error != "" {
				return errors.New(p.Error)
			}
		}
		s.SetBase(req.Key, req.Base)
		return nil
	}))
}

var errUnknownProject = errors.New("local: that is not a project vuoro lists; refresh the Local tab")

// knownProject is the listed project with key, shown or hidden, or nil. A
// route never runs git in a folder the request names by key alone. The
// caller holds the lock.
func (a *App) knownProject(key string) *localProject {
	if a.local.view == nil || key == "" {
		return nil
	}
	for _, l := range [][]localProject{a.local.view.Projects, a.local.view.Hidden} {
		for i := range l {
			if l[i].Key == key {
				return &l[i]
			}
		}
	}
	return nil
}

// localReq is the body every edit takes; each route reads what it needs.
type localReq struct {
	Path   string `json:"path,omitempty"`
	Key    string `json:"key,omitempty"`
	Hidden bool   `json:"hidden,omitempty"`
	Base   string `json:"base,omitempty"`
}

// localEdit changes the store with edit, saves it, and answers with the
// tab built again.
func (a *App) localEdit(edit func(*local.Store, localReq) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req localReq
		if !readJSON(w, r, &req) {
			return
		}
		a.local.mu.Lock()
		defer a.local.mu.Unlock()
		s, err := local.LoadStore(a.localStorePath())
		if err != nil {
			writeError(w, err)
			return
		}
		if a.local.view == nil && req.Key != "" {
			if _, err := a.buildLocal(r.Context(), s, false); err != nil {
				writeError(w, err)
				return
			}
		}
		if err := edit(&s, req); err != nil {
			writeError(w, err)
			return
		}
		if err := s.Save(a.localStorePath()); err != nil {
			writeError(w, err)
			return
		}
		v, err := a.buildLocal(r.Context(), s, false)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, v)
	}
}

func (a *App) getLocal(w http.ResponseWriter, r *http.Request) {
	a.local.mu.Lock()
	defer a.local.mu.Unlock()
	if v := a.local.view; v != nil && time.Since(v.Built) < localFresh {
		writeJSON(w, http.StatusOK, v)
		return
	}
	a.serveLocal(w, r, false)
}

func (a *App) refreshLocal(w http.ResponseWriter, r *http.Request) {
	var req struct{}
	if !readJSON(w, r, &req) {
		return
	}
	a.local.mu.Lock()
	defer a.local.mu.Unlock()
	a.serveLocal(w, r, true)
}

// serveLocal builds the tab and answers with it. The caller holds the lock.
func (a *App) serveLocal(w http.ResponseWriter, r *http.Request, rescan bool) {
	s, err := local.LoadStore(a.localStorePath())
	if err != nil {
		writeError(w, err)
		return
	}
	v, err := a.buildLocal(r.Context(), s, rescan)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// buildLocal scans the roots if their scan is stale, missing, or for other
// roots, or rescan asks, then reads every project and matches its runs. The
// caller holds the lock.
func (a *App) buildLocal(ctx context.Context, s local.Store, rescan bool) (*localView, error) {
	scan := a.loadScan()
	if rescan || scan == nil || !slices.Equal(scan.Roots, s.Roots) || time.Since(scan.At) > scanFresh {
		scan = &scanRecord{Roots: s.Roots, At: time.Now().UTC(), Result: local.Scan(ctx, s.Roots, local.DefaultLimits), Version: 1}
		if path := a.scanPath(); path != "" {
			if err := state.MkdirAll(filepath.Dir(path)); err != nil {
				return nil, err
			}
			if err := state.WriteJSON(path, scan); err != nil {
				return nil, err
			}
		}
	}
	v := &localView{
		Roots: s.Roots, Added: s.Projects, Scanned: scan.At, Stopped: scan.Result.Stopped, Built: time.Now().UTC(),
		Projects: []localProject{}, Hidden: []localProject{}, Unreadable: []unreadable{},
	}

	// Read every folder, added ones first, at most inspectAtOnce at a time.
	paths := append(slices.Clone(s.Projects), scan.Result.Checkouts...)
	type read struct {
		p   *local.Project
		err error
	}
	reads := make([]read, len(paths))
	sem := make(chan struct{}, inspectAtOnce)
	var wg sync.WaitGroup
	for i, path := range paths {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			p, err := local.Inspect(ctx, "", path, "")
			if err == nil {
				if base := s.Bases[p.Key]; base != "" {
					p, err = local.Inspect(ctx, "", path, base)
				}
			}
			reads[i] = read{p, err}
		}()
	}
	wg.Wait()

	byKey := map[string]*localProject{}
	var order []string
	for i, rd := range reads {
		added := i < len(s.Projects)
		if rd.err != nil {
			if added {
				v.Unreadable = append(v.Unreadable, unreadable{Path: paths[i], Error: rd.err.Error()})
			}
			continue
		}
		if lp, ok := byKey[rd.p.Key]; ok {
			lp.Added = lp.Added || added
			continue
		}
		byKey[rd.p.Key] = &localProject{Project: rd.p, Added: added, BaseChosen: s.Bases[rd.p.Key] != ""}
		order = append(order, rd.p.Key)
	}
	runs := localRuns(a.Runs)
	for _, key := range order {
		lp := byKey[key]
		lp.Branches = a.matchRuns(ctx, lp.Project, runs)
		if slices.Contains(s.Hidden, key) {
			v.Hidden = append(v.Hidden, *lp)
			continue
		}
		for _, b := range lp.Branches {
			if len(b.Needs) > 0 {
				v.Needs++
			}
		}
		v.Projects = append(v.Projects, *lp)
	}
	sort.SliceStable(v.Projects, func(i, j int) bool {
		return strings.ToLower(v.Projects[i].Name) < strings.ToLower(v.Projects[j].Name)
	})
	a.local.view = v
	return v, nil
}

func (a *App) loadScan() *scanRecord {
	path := a.scanPath()
	if path == "" {
		return nil
	}
	var rec scanRecord
	if err := state.ReadJSON(path, &rec); err != nil || rec.Version != 1 {
		return nil
	}
	if rec.Roots == nil {
		rec.Roots = []string{}
	}
	return &rec
}

// localRun is a review of a local range, as matched to branches.
type localRun struct {
	origin string
	view   review.RunView
}

// localRuns are the reviews of local ranges, newest first.
func localRuns(m *review.Manager) []localRun {
	if m == nil {
		return nil
	}
	var out []localRun
	for _, r := range m.Runs() {
		if o := r.Origin(); o != "" {
			v := r.View()
			if v.Mode == "" {
				out = append(out, localRun{origin: filepath.Clean(o), view: v})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].view.Created.After(out[j].view.Created) })
	return out
}

// matchRuns gives each branch its newest review: the newest run, from one of
// the project's checkouts, whose reviewed head is in the branch's history
// and not in the base's. A branch with none has never been reviewed.
func (a *App) matchRuns(ctx context.Context, p *local.Project, runs []localRun) []localBranch {
	out := make([]localBranch, 0, len(p.Branches))
	names := make([]string, 0, len(p.Branches))
	for _, b := range p.Branches {
		out = append(out, localBranch{Branch: b, Fixes: []fixBranch{}, Needs: []string{}})
		names = append(names, b.Name)
	}
	checkouts := map[string]bool{p.Path: true}
	for _, w := range p.Worktrees {
		checkouts[w.Path] = true
	}
	seen := 0
	for _, r := range runs {
		if !checkouts[r.origin] && !sameDir(r.origin, checkouts) {
			continue
		}
		if seen++; seen > runsPerProject {
			break
		}
		in, _, err := p.Containing(ctx, r.view.Head, names)
		if err != nil {
			continue
		}
		for i := range out {
			b := &out[i]
			if b.Run != nil || !slices.Contains(in, b.Name) {
				continue
			}
			b.Run = &branchRun{ID: r.view.ID, Status: r.view.Status, Head: r.view.Head, Created: r.view.Created, Moved: r.view.Head != b.Tip}
			for _, d := range r.view.Drafts {
				if d.Status == drafts.Pending && d.Decision == "" && d.FixedIn == "" {
					b.Run.Undecided++
				}
			}
			if ed := r.view.Editor; ed != nil && ed.Local && !ed.Discarded && ed.Pushed != nil {
				if _, ok := p.Fixes[ed.Pushed.Branch]; ok {
					b.Fixes = append(b.Fixes, fixBranch{Branch: ed.Pushed.Branch, Waiting: !p.Merged(ctx, ed.Pushed.Branch, b.Name)})
				}
			}
		}
	}
	for i := range out {
		b := &out[i]
		switch {
		case b.Run == nil:
			b.Needs = append(b.Needs, "never reviewed")
		case b.Run.Moved:
			b.Needs = append(b.Needs, "moved since its review")
		}
		if b.Run != nil && b.Run.Undecided > 0 {
			b.Needs = append(b.Needs, fmt.Sprintf("%d finding%s undecided", b.Run.Undecided, plural(b.Run.Undecided)))
		}
		for _, f := range b.Fixes {
			if f.Waiting {
				b.Needs = append(b.Needs, fmt.Sprintf("%s waits to be merged", f.Branch))
			}
		}
	}
	return out
}

// sameDir says whether path names one of the checkouts, however each is
// spelled, for a run recorded before a path's spelling changed.
func sameDir(path string, checkouts map[string]bool) bool {
	x, err := os.Stat(path)
	if err != nil {
		return false
	}
	for c := range checkouts {
		if y, err := os.Stat(c); err == nil && os.SameFile(x, y) {
			return true
		}
	}
	return false
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
