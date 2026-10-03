package app

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
)

type reader struct {
	kind     string
	ref      forge.Ref
	checkout forge.Checkout
	opened   int
}

func (r *reader) Open(_ context.Context, ref forge.Ref) (*forge.Change, error) {
	r.ref = ref
	r.opened++
	return &forge.Change{Kind: r.kind, Ref: ref, TargetBranch: "main", Diff: forge.DiffRefs{Head: "head", Base: "base"}, Checkout: r.checkout}, nil
}
func (r *reader) Capabilities() forge.Capabilities { return forge.Capabilities{} }
func TestMixedForgesRouteBeforeResolvingCredentials(t *testing.T) {
	t.Setenv("VUORO_TEST_GITLAB_TOKEN", "gitlab-test-token")
	t.Setenv("VUORO_TEST_FORGEJO_TOKEN", "forgejo-test-token")
	gl := &reader{kind: forge.GitLab}
	fj := &reader{kind: forge.Forgejo}
	calls := map[string]int{}
	a := &App{Config: &config.Config{Forges: []config.Forge{
		{Name: "gl", Kind: forge.GitLab, URL: "https://gitlab.example", TokenEnv: "VUORO_TEST_GITLAB_TOKEN"},
		{Name: "fj", Kind: forge.Forgejo, URL: "https://forge.example", TokenEnv: "VUORO_TEST_FORGEJO_TOKEN"},
	}}, Dial: func(kind, base string, tok config.Secret) (forge.Provider, error) {
		calls[base]++
		switch base {
		case "https://gitlab.example":
			if kind != forge.GitLab || tok.Reveal() != "gitlab-test-token" {
				t.Fatal("wrong GitLab credentials or kind")
			}
			return gl, nil
		case "https://forge.example":
			if kind != forge.Forgejo || tok.Reveal() != "forgejo-test-token" {
				t.Fatal("wrong Forgejo credentials or kind")
			}
			return fj, nil
		case "https://public.example":
			if kind != forge.Forgejo || !tok.Empty() {
				t.Fatal("anonymous host received credentials")
			}
			return &reader{kind: forge.Forgejo}, nil
		default:
			t.Fatal("unexpected host")
			return nil, errors.New("unexpected host")
		}
	}}
	for _, raw := range []string{"https://gitlab.example/g/p/-/merge_requests/1", "https://forge.example/o/r/pulls/2", "https://forge.example/o/r/pulls/2/files", "https://public.example/o/r/pulls/3"} {
		if _, err := a.open(context.Background(), raw); err != nil {
			t.Fatal(err)
		}
	}
	if gl.ref.Project != "g/p" || fj.ref.Number != 2 || calls["https://forge.example"] != 1 {
		t.Fatalf("routing/cache: %+v", calls)
	}
	if _, err := a.open(context.Background(), "https://forge.example/o/r/-/merge_requests/2"); err == nil {
		t.Fatal("configured kind ignored")
	}
}
func TestForgejoCheckoutUsesTargetRepositoryAndAdvertisedSSHPort(t *testing.T) {
	ch := &forge.Change{Kind: forge.Forgejo, Ref: forge.Ref{Host: "forge.example", Project: "o/r", Number: 2}, SourceBranch: "contributor-feature", TargetBranch: "main", Diff: forge.DiffRefs{Head: "head", Base: "base"},
		Checkout: forge.Checkout{HTTPS: "https://forge.example/o/r.git", SSH: "ssh://git@forge.example:2222/o/r.git", HeadRef: "refs/pull/2/head"}}
	a := &App{Config: &config.Config{Forges: []config.Forge{{URL: "https://forge.example"}}}}
	src, err := a.source(ch)
	if err != nil {
		t.Fatal(err)
	}
	if src.Remote != ch.Checkout.SSH || src.Head != "head" || src.Base != "base" || strings.Join(src.Refspecs, " ") != "+refs/pull/2/head:refs/vuoro/head +refs/heads/main:refs/vuoro/target/main" {
		t.Fatalf("source: %+v", src)
	}
	// A fork's source branch name never replaces the target repository's PR ref.
	ch.SourceBranch = "fork/feature"
	src2, err := a.source(ch)
	if err != nil || strings.Join(src2.Refspecs, " ") != strings.Join(src.Refspecs, " ") {
		t.Fatal("fork altered refspecs")
	}
	a.Config.Forges[0].Clone = "https"
	src, err = a.source(ch)
	if err != nil || src.Remote != ch.Checkout.HTTPS {
		t.Fatal("https transport ignored")
	}
	ch.Checkout.HTTPS = "https://elsewhere.example/o/r.git"
	if _, err := a.source(ch); err == nil {
		t.Fatal("cross-host clone accepted")
	}
}
func TestForgejoRejectsEveryWriteRouteEvenWithAnAccount(t *testing.T) {
	r := &reader{kind: forge.Forgejo}
	t.Setenv("VUORO_TEST_FORGEJO_TOKEN", "test-token")
	a := &App{Config: &config.Config{Forges: []config.Forge{{Name: "fj", Kind: forge.Forgejo, URL: "https://forge.example", Username: "me", TokenEnv: "VUORO_TEST_FORGEJO_TOKEN"}}}, Dial: func(string, string, config.Secret) (forge.Provider, error) { return r, nil }}
	for _, path := range []string{"/api/changes/drafts", "/api/changes/drafts/1/update", "/api/changes/drafts/1/delete", "/api/changes/submit", "/api/changes/resolve", "/api/changes/approve"} {
		body := map[string]any{"url": "https://forge.example/o/r/pulls/1"}
		if path == "/api/changes/drafts" {
			body["body"] = "a comment"
		}
		code, _ := call(t, a.Handler(), path, body)
		if code != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d", path, code)
		}
	}
	if r.opened == 0 {
		t.Fatal("write check bypassed provider")
	}
}
