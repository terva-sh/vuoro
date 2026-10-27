package app

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/review"
	"github.com/terva-sh/vuoro/review/drafts"
	"github.com/terva-sh/vuoro/server"
	"github.com/terva-sh/vuoro/state"
)

// triagedProject is a project app with a project triage run on disk whose
// one proposal, for !1, adds bob and the backend label. stamp is the stamp
// the run recorded for !1; "" records the change as it reads now.
func triagedProject(t *testing.T, caps forge.Capabilities, stamp string) (*App, string, *[]forge.Changes) {
	t.Helper()
	a, administered := projectAppWith(t, &caps)
	key := addedProject(t, a)
	if stamp == "" {
		ch, _ := (&inboxReader{kind: forge.GitLab, host: "lab.test"}).ReadFacts(context.Background(), forge.Ref{})
		stamp = changeStamp(*ch)
	}
	dir := t.TempDir()
	d := drafts.Draft{ID: "d1", Change: 1, Reviewers: []string{"bob", "author"}, Labels: []string{"backend"}, Unblock: "Review it", Reason: "Bob owns it",
		Status: drafts.Pending, Revision: 1, Session: "s1", Drafted: "Fake tool", Updated: time.Now()}
	rec := map[string]any{"id": "project-run", "kind": forge.GitLab, "mode": review.ProjectMode, "ref": forge.Ref{Host: "lab.test", Project: "team/repo"},
		"title": "Team / Repo", "status": "idle", "drafts": []drafts.Draft{d}, "sessions": []any{},
		"project_triage": review.ProjectTriage{Path: "team/repo", Changes: []review.ProjectChange{{Number: 1, Stamp: stamp}}, Candidates: []string{"bob"}, Labels: []string{"backend"}}}
	if err := state.WriteJSON(filepath.Join(dir, "runs", "project-run", "run.json"), rec); err != nil {
		t.Fatal(err)
	}
	a.Runs = &review.Manager{StateDir: dir}
	if err := a.Runs.Load(); err != nil {
		t.Fatal(err)
	}
	return a, key, administered
}

type accepted struct {
	Run      review.RunView `json:"run"`
	Outcomes []Outcome      `json:"outcomes"`
}

func TestAProposalReachesTheForgeOnlyWhenThePersonAcceptsIt(t *testing.T) {
	a, key, administered := triagedProject(t, forge.Capabilities{Reviewers: true, Labels: true}, "")
	var run *review.RunView
	_ = json.Unmarshal(projectCall(t, a, "GET", "/api/projects/"+key+"/triage", "", 200), &run)
	if run == nil || run.ID != "project-run" || len(run.Drafts) != 1 || len(*administered) != 0 {
		t.Fatalf("triage run %+v, administered %+v", run, *administered)
	}
	// The run shows on its project, not among the changes' reviews, and the
	// generic run routes cannot keep or post its proposals.
	var list []review.RunView
	_ = json.Unmarshal(projectCall(t, a, "GET", "/api/runs", "", 200), &list)
	if len(list) != 0 {
		t.Fatal("a project triage run is listed with the reviews")
	}
	projectCall(t, a, "GET", "/api/runs/project-run", "", 200)
	for _, path := range []string{"/api/runs/project-run/drafts/d1/keep", "/api/runs/project-run/report", "/api/runs/project-run/editor"} {
		projectCall(t, a, "POST", path, "{}", 404)
	}
	projectCall(t, a, "POST", "/api/projects/"+key+"/triage/project-run/accept", `{"proposals":[]}`, 400)
	projectCall(t, a, "POST", "/api/projects/"+key+"/triage/other-run/accept", `{"proposals":[{"id":"d1"}]}`, 404)
	if len(*administered) != 0 {
		t.Fatal("a proposal reached the forge without the person accepting it")
	}

	var got accepted
	_ = json.Unmarshal(projectCall(t, a, "POST", "/api/projects/"+key+"/triage/project-run/accept", `{"proposals":[{"id":"d1"}]}`, 200), &got)
	if len(got.Outcomes) != 1 || got.Outcomes[0].Result != "accepted" || got.Run.Drafts[0].Decision != drafts.Kept {
		t.Fatalf("accept %+v", got)
	}
	// Only additions, and never the change's own author.
	if len(*administered) != 1 || strings.Join((*administered)[0].AddReviewers, ",") != "bob" || (*administered)[0].AddLabels[0] != "backend" ||
		len((*administered)[0].RemoveReviewers)+len((*administered)[0].RemoveLabels) != 0 {
		t.Fatalf("administered %+v", *administered)
	}
	_ = json.Unmarshal(projectCall(t, a, "POST", "/api/projects/"+key+"/triage/project-run/accept", `{"proposals":[{"id":"d1"}]}`, 200), &got)
	if got.Outcomes[0].Result != "failed" || len(*administered) != 1 {
		t.Fatal("a proposal was accepted twice")
	}
}

func TestAProposalWhoseChangeMovedWaitsForThePerson(t *testing.T) {
	a, key, administered := triagedProject(t, forge.Capabilities{Reviewers: true, Labels: true}, "an-older-stamp")
	var got accepted
	_ = json.Unmarshal(projectCall(t, a, "POST", "/api/projects/"+key+"/triage/project-run/accept", `{"proposals":[{"id":"d1"}]}`, 200), &got)
	if got.Outcomes[0].Result != "stale" || len(*administered) != 0 || got.Run.Drafts[0].Decision != "" {
		t.Fatalf("a stale proposal was applied: %+v %+v", got, *administered)
	}
	// Having seen it is stale, the person accepts it anyway, edited.
	_ = json.Unmarshal(projectCall(t, a, "POST", "/api/projects/"+key+"/triage/project-run/accept", `{"proposals":[{"id":"d1","stale":true,"labels":[]}]}`, 200), &got)
	if got.Outcomes[0].Result != "accepted" || len(*administered) != 1 || len((*administered)[0].AddLabels) != 0 {
		t.Fatalf("accept anyway %+v %+v", got, *administered)
	}
}

func TestAProposalTheForgeCannotCarryOutIsRefused(t *testing.T) {
	a, key, administered := triagedProject(t, forge.Capabilities{Reviewers: true}, "")
	var got accepted
	_ = json.Unmarshal(projectCall(t, a, "POST", "/api/projects/"+key+"/triage/project-run/accept", `{"proposals":[{"id":"d1"}]}`, 200), &got)
	if got.Outcomes[0].Result != "failed" || len(*administered) != 0 || got.Run.Drafts[0].Decision != "" {
		t.Fatalf("labels reached a forge that cannot set them: %+v", got)
	}
	var run review.RunView
	_ = json.Unmarshal(projectCall(t, a, "POST", "/api/projects/"+key+"/triage/project-run/discard", `{"id":"d1"}`, 200), &run)
	if run.Drafts[0].Decision != drafts.Dropped || len(*administered) != 0 {
		t.Fatal("discard")
	}
}

func TestProjectTriageRoutesStayBehindWindowGuard(t *testing.T) {
	a, _, _ := triagedProject(t, forge.Capabilities{}, "")
	h := server.New(server.Config{Addr: "127.0.0.1:9123", Token: "fixture-window-token", Dist: fstest.MapFS{}, API: a.Handler()})
	for _, route := range []string{"POST /api/projects/k/triage", "GET /api/projects/k/triage", "POST /api/projects/k/triage/r/accept", "POST /api/projects/k/triage/r/discard"} {
		method, path, _ := strings.Cut(route, " ")
		r := httptest.NewRequest(method, "http://127.0.0.1:9123"+path, strings.NewReader("{}"))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatalf("%s bypassed window guard", route)
		}
	}
}

func TestTriageFactsComeFromTheSnapshotAndTheForge(t *testing.T) {
	a, key, _ := triagedProject(t, forge.Capabilities{Reviewers: true, Labels: true}, "")
	s, _ := a.projects()
	snap, _ := s.Snapshot(key)
	p := projectReader{inboxReader: &inboxReader{kind: forge.GitLab, host: "lab.test"}, administered: &[]forge.Changes{}}
	in, err := a.triageInput(context.Background(), snap, p, p, nil)
	if err != nil {
		t.Fatal(err)
	}
	var facts triageFacts
	if err := json.Unmarshal([]byte(in.Facts), &facts); err != nil {
		t.Fatal(err)
	}
	if len(facts.Changes) != 1 || facts.Changes[0].Number != 1 || facts.Changes[0].Paths == nil || facts.Labels[0] != "backend" || facts.Candidates[0].Username != "bob" {
		t.Fatalf("facts %s", in.Facts)
	}
	if len(in.Triage.Changes) != 1 || in.Triage.Changes[0].Stamp == "" || in.Triage.Candidates[0] != "bob" || in.Triage.Path != "team/repo" {
		t.Fatalf("binding %+v", in.Triage)
	}
	if _, err := a.triageInput(context.Background(), snap, p, p, []int{7}); err == nil {
		t.Fatal("a run was built for a change not in the project")
	}
}
