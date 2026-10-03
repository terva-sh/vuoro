package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/forge/gitlab"
	"github.com/terva-sh/vuoro/review"
)

func change(target string) *forge.Change {
	return &forge.Change{
		Ref:          forge.Ref{Host: "gitlab.com", Project: "group/sub/proj", Number: 12},
		TargetBranch: target,
		Diff:         forge.DiffRefs{Base: "b", Head: "h"},
		Checkout:     forge.Checkout{HTTPS: "https://gitlab.com/group/sub/proj.git", SSH: "git@gitlab.com:group/sub/proj.git", HeadRef: "refs/merge-requests/12/head"},
	}
}

func TestASourceFetchesTheMergeRequestOverThePersonsTransport(t *testing.T) {
	a := &App{Config: &config.Config{Forges: []config.Forge{{Name: "g", URL: "https://gitlab.com"}}}}
	src, err := a.source(change("release/1.x"))
	if err != nil {
		t.Fatal(err)
	}
	if src.Remote != "git@gitlab.com:group/sub/proj.git" || src.Key != "gitlab.com/group/sub/proj" {
		t.Fatalf("%+v", src)
	}
	if strings.Join(src.Refspecs, " ") != "+refs/merge-requests/12/head:refs/vuoro/head +refs/heads/release/1.x:refs/vuoro/target/release/1.x" {
		t.Fatalf("refspecs %q", src.Refspecs)
	}
	a.Config.Forges[0].Clone = "https"
	if src, _ := a.source(change("main")); src.Remote != "https://gitlab.com/group/sub/proj.git" {
		t.Fatalf("https remote %q", src.Remote)
	}
	// With no configured forge, a public project is fetched anonymously.
	if src, _ := (&App{}).source(change("main")); !strings.HasPrefix(src.Remote, "https://") {
		t.Fatalf("anonymous remote %q", src.Remote)
	}
}

func TestATargetBranchCannotSmuggleARefspec(t *testing.T) {
	for _, bad := range []string{"", "main:refs/heads/x", "-x", "a b", "a..b", "*"} {
		if _, err := (&App{}).source(change(bad)); err == nil {
			t.Errorf("target %q accepted", bad)
		}
	}
}

func TestErrorsMapToStatuses(t *testing.T) {
	for err, want := range map[error]int{
		review.ErrNoRun:    404,
		gitlab.ErrNotFound: 404,
		&gitlab.APIError{Method: "GET", Path: "/x", Status: 500}: 502,
		errors.New("bad"): 400,
	} {
		w := httptest.NewRecorder()
		writeError(w, err)
		if w.Code != want {
			t.Errorf("%v: %d, want %d", err, w.Code, want)
		}
	}
}

func TestAPrivateChangeReadAnonymouslySaysWhy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":{"project":null}}`))
	}))
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")
	a := &App{Dial: func(string, string, config.Secret) (forge.Provider, error) {
		return gitlab.New(srv.URL, config.Secret{})
	}}
	_, err := a.open(context.Background(), "https://"+host+"/g/p/-/merge_requests/1")
	if !errors.Is(err, gitlab.ErrNotFound) || !strings.Contains(err.Error(), "names no forge for "+host) {
		t.Fatalf("err %v", err)
	}
}
