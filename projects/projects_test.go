package projects

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
)

// fakeForge is a forge with one project, team/repo, whose open changes are
// open. noDeps makes it a forge that cannot say what blocks a change.
type fakeForge struct {
	open    []int
	closing map[int]bool // listed, then closed by the time it is read
	failing map[int]bool
	listErr error
	noDeps  bool
	labels  map[int][]string
	reads   int
}

func (f *fakeForge) Capabilities() forge.Capabilities { return forge.Capabilities{} }
func (f *fakeForge) Open(ctx context.Context, ref forge.Ref) (*forge.Change, error) {
	return f.ReadFacts(ctx, ref)
}
func (f *fakeForge) ReadFacts(_ context.Context, ref forge.Ref) (*forge.Change, error) {
	f.reads++
	if f.failing[ref.Number] {
		return nil, &forge.APIError{Status: 500, Message: "synthetic-project-secret"}
	}
	state := forge.Open
	if f.closing[ref.Number] {
		state = forge.Merged
	}
	return &forge.Change{Kind: forge.GitLab, Ref: ref, State: state, Title: fmt.Sprint("change ", ref.Number),
		Labels: f.labels[ref.Number], Files: []forge.FileDiff{{NewPath: "x"}}}, nil
}
func (f *fakeForge) SearchProjects(_ context.Context, q string) ([]forge.Project, error) {
	if !strings.Contains("team/repo", q) {
		return []forge.Project{}, nil
	}
	return []forge.Project{{Host: "lab.test", Path: "team/repo", Name: "Team / Repo"}}, nil
}
func (f *fakeForge) Project(_ context.Context, path string) (forge.Project, error) {
	if !strings.EqualFold(path, "team/repo") {
		return forge.Project{}, forge.ErrNotFound
	}
	return forge.Project{Host: "lab.test", Path: "team/repo", Name: "Team / Repo", WebURL: "https://lab.test/team/repo"}, nil
}
func (f *fakeForge) ListOpen(context.Context, string) ([]forge.Discovered, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := []forge.Discovered{}
	for _, n := range f.open {
		out = append(out, forge.Discovered{Ref: forge.Ref{Host: "lab.test", Project: "team/repo", Number: n}})
	}
	return out, nil
}

// depForge also reads dependencies: every change waits on the one before it.
type depForge struct{ fakeForge }

func (f *depForge) Dependencies(_ context.Context, ref forge.Ref) ([]forge.Ref, error) {
	if f.noDeps {
		return nil, forge.ErrUnsupported
	}
	if ref.Number == 1 {
		return []forge.Ref{}, nil
	}
	return []forge.Ref{{Host: ref.Host, Project: ref.Project, Number: ref.Number - 1}}, nil
}

var account = config.Forge{Name: "Work", Kind: forge.GitLab, URL: "https://lab.test", Username: "me", Bots: []string{"ci-bot"}}

func service(t *testing.T, dir string, p forge.Provider) *Service {
	t.Helper()
	s, err := New(dir, []config.Forge{account}, func(context.Context, config.Forge) (forge.Provider, error) { return p, nil })
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestAddedProjectsSurviveARestart(t *testing.T) {
	dir := t.TempDir()
	f := &depForge{fakeForge{open: []int{1, 2}}}
	s := service(t, dir, f)
	e, err := s.Add(context.Background(), "LAB.test", "Team/Repo")
	if err != nil || e.Path != "team/repo" || e.Kind != forge.GitLab {
		t.Fatalf("added %+v %v", e, err)
	}
	if again, _ := s.Add(context.Background(), "lab.test", "team/repo"); again.Key != e.Key || len(s.Entries()) != 1 {
		t.Fatal("adding a project twice listed it twice")
	}
	if _, err := s.Refresh(context.Background(), e.Key); err != nil {
		t.Fatal(err)
	}
	restarted := service(t, dir, f)
	snap, ok := restarted.Snapshot(e.Key)
	if !ok || len(snap.Items) != 2 || snap.Project.Path != "team/repo" {
		t.Fatalf("after restart %+v", snap)
	}
	if err := restarted.Remove(e.Key); err != nil {
		t.Fatal(err)
	}
	if len(service(t, dir, f).Entries()) != 0 {
		t.Fatal("a removed project came back")
	}
}

func TestAProjectTheAccountCannotSeeIsNotAdded(t *testing.T) {
	s := service(t, "", &fakeForge{})
	if _, err := s.Add(context.Background(), "lab.test", "other/repo"); err == nil {
		t.Fatal("added a project the account cannot see")
	}
	if _, err := s.Add(context.Background(), "elsewhere.test", "team/repo"); err == nil || !strings.Contains(err.Error(), "Settings") {
		t.Fatalf("added a project with no account: %v", err)
	}
}

func TestRefreshReadsEveryOpenChange(t *testing.T) {
	f := &depForge{fakeForge{open: []int{1, 2, 3}, closing: map[int]bool{3: true}}}
	s := service(t, "", f)
	e, _ := s.Add(context.Background(), "lab.test", "team/repo")
	snap, err := s.Refresh(context.Background(), e.Key)
	if err != nil || snap.Error != "" {
		t.Fatal(err, snap.Error)
	}
	if len(snap.Items) != 2 {
		t.Fatalf("a change closed after listing stayed: %d items", len(snap.Items))
	}
	if !snap.Dependencies || !snap.Items[1].Change.DependenciesKnown || snap.Items[1].Change.BlockedBy[0].Number != 1 {
		t.Fatalf("dependencies %+v", snap.Items[1].Change)
	}
	if snap.Items[0].Change.Files != nil {
		t.Fatal("a snapshot kept patches")
	}
}

func TestUnknownDependenciesAreNotNone(t *testing.T) {
	for _, p := range []forge.Provider{&fakeForge{open: []int{1, 2}}, &depForge{fakeForge{open: []int{1, 2}, noDeps: true}}} {
		s := service(t, "", p)
		e, _ := s.Add(context.Background(), "lab.test", "team/repo")
		snap, _ := s.Refresh(context.Background(), e.Key)
		if snap.Dependencies || snap.Items[0].Change.DependenciesKnown {
			t.Fatalf("%T: dependencies reported known", p)
		}
	}
}

func TestAFailedRefreshKeepsTheLastSnapshot(t *testing.T) {
	f := &fakeForge{open: []int{1, 2}}
	s := service(t, t.TempDir(), f)
	e, _ := s.Add(context.Background(), "lab.test", "team/repo")
	first, _ := s.Refresh(context.Background(), e.Key)
	f.listErr = &forge.APIError{Status: 403, Message: "synthetic-project-secret"}
	snap, _ := s.Refresh(context.Background(), e.Key)
	if len(snap.Items) != 2 || snap.Error == "" || strings.Contains(snap.Error, "synthetic") || !snap.LastSuccess.Equal(first.LastSuccess) {
		t.Fatalf("after a failure %+v", snap)
	}
	f.listErr = nil
	f.failing = map[int]bool{2: true}
	snap, _ = s.Refresh(context.Background(), e.Key)
	if len(snap.Items) != 2 || snap.Items[1].Error == "" || snap.Items[1].Change.Title != "change 2" || snap.Warning == "" {
		t.Fatalf("one unreadable change %+v", snap)
	}
}

func TestAProjectTooLargeIsRefusedWithItsCount(t *testing.T) {
	f := &fakeForge{}
	for i := 1; i <= MaxChanges+1; i++ {
		f.open = append(f.open, i)
	}
	s := service(t, "", f)
	e, _ := s.Add(context.Background(), "lab.test", "team/repo")
	snap, _ := s.Refresh(context.Background(), e.Key)
	if len(snap.Items) != 0 || !strings.Contains(snap.Error, fmt.Sprint(MaxChanges+1)) || f.reads != 0 {
		t.Fatalf("a large project was read in part: %d items, %d reads, %q", len(snap.Items), f.reads, snap.Error)
	}
}

func TestRefreshChangeUpdatesOneRow(t *testing.T) {
	f := &depForge{fakeForge{open: []int{1, 2}}}
	s := service(t, "", f)
	e, _ := s.Add(context.Background(), "lab.test", "team/repo")
	before, _ := s.Refresh(context.Background(), e.Key)
	f.labels = map[int][]string{2: {"backend"}}
	if err := s.RefreshChange(context.Background(), forge.Ref{Host: "lab.test", Project: "team/repo", Number: 2}); err != nil {
		t.Fatal(err)
	}
	snap, _ := s.Snapshot(e.Key)
	if len(snap.Items[1].Change.Labels) != 1 || !snap.Items[1].Change.DependenciesKnown || !snap.LastSuccess.Equal(before.LastSuccess) {
		t.Fatalf("after a targeted read %+v", snap.Items[1].Change)
	}
	f.closing = map[int]bool{2: true}
	_ = s.RefreshChange(context.Background(), forge.Ref{Host: "lab.test", Project: "team/repo", Number: 2})
	if snap, _ := s.Snapshot(e.Key); len(snap.Items) != 1 {
		t.Fatal("a closed change stayed after a targeted read")
	}
}

func TestSearchMarksProjectsAlreadyAdded(t *testing.T) {
	s := service(t, "", &fakeForge{})
	got := s.Search(context.Background(), "repo")
	if len(got.Projects) != 1 || got.Projects[0].Added || got.Projects[0].Account != "Work" {
		t.Fatalf("search %+v", got)
	}
	_, _ = s.Add(context.Background(), "lab.test", "team/repo")
	if !s.Search(context.Background(), "repo").Projects[0].Added {
		t.Fatal("an added project was not marked")
	}
	broken, _ := New("", []config.Forge{account}, func(context.Context, config.Forge) (forge.Provider, error) {
		return nil, errors.New("synthetic-project-secret")
	})
	if got := broken.Search(context.Background(), "repo"); len(got.Errors) != 1 || strings.Contains(got.Errors[0], "synthetic") {
		t.Fatalf("search errors %+v", got.Errors)
	}
}

func TestSnapshotsMarkConfiguredBots(t *testing.T) {
	f := &fakeForge{open: []int{1}}
	s := service(t, "", f)
	e, _ := s.Add(context.Background(), "lab.test", "team/repo")
	_, _ = s.Refresh(context.Background(), e.Key)
	s.mu.Lock()
	snap := s.snapshots[e.Key]
	snap.Items[0].Change.Reviewers = []forge.Reviewer{{User: forge.User{Username: "ci-bot"}}}
	s.snapshots[e.Key] = snap
	s.mu.Unlock()
	got, _ := s.Snapshot(e.Key)
	if !got.Items[0].Change.Reviewers[0].Bot {
		t.Fatal("a configured bot was not marked")
	}
}
