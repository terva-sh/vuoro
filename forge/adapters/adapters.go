// Package adapters selects forge implementations. The application knows this
// registry and the neutral contracts, never an individual forge's API.
package adapters

import (
	"strings"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/forge/forgejo"
	"github.com/terva-sh/vuoro/forge/github"
	"github.com/terva-sh/vuoro/forge/gitlab"
)

type adapter struct {
	parse        func(string) (forge.Ref, error)
	parseProject func(string) (forge.Project, error)
	newClient    func(string, config.Secret) (forge.Provider, error)
}

var registry = map[string]adapter{
	forge.GitLab:  {gitlab.ParseURL, gitlab.ParseProjectURL, func(base string, t config.Secret) (forge.Provider, error) { return gitlab.New(base, t) }},
	forge.Forgejo: {forgejo.ParseURL, twoPart, func(base string, t config.Secret) (forge.Provider, error) { return forgejo.New(base, t) }},
	forge.GitHub:  {github.ParseURL, twoPart, func(base string, t config.Secret) (forge.Provider, error) { return github.New(base, t) }},
}

// ParseURL honors a configured host's kind. Other URLs go by their path
// shape, and github.com is GitHub's.
func ParseURL(raw, kind string) (forge.Ref, string, error) {
	u, err := forge.ChangeURL(raw)
	if err != nil {
		return forge.Ref{}, "", err
	}
	if kind == "" {
		switch {
		case strings.EqualFold(u.Hostname(), "github.com"):
			kind = forge.GitHub
		case strings.Contains(u.Path, "/-/merge_requests/"):
			kind = forge.GitLab
		case strings.Contains(u.Path, "/pulls/"):
			kind = forge.Forgejo
		case strings.Contains(u.Path, "/pull/"):
			kind = forge.GitHub
		default:
			return forge.Ref{}, "", forge.ErrNotAChange
		}
	}
	a, ok := registry[kind]
	if !ok {
		return forge.Ref{}, "", forge.ErrUnsupported
	}
	ref, err := a.parse(raw)
	return ref, kind, err
}

// twoPart reads a project URL on a forge whose projects are an owner and a
// name, with no nested groups.
func twoPart(raw string) (forge.Project, error) { return forge.ProjectURL(raw, 2) }

// ParseProjectURL reads a project URL on a host of the given kind. Unlike a
// change URL, a project URL has no path shape that names its forge, so the
// host must be configured, or be github.com.
func ParseProjectURL(raw, kind string) (forge.Project, string, error) {
	u, err := forge.ChangeURL(raw)
	if err != nil {
		return forge.Project{}, "", forge.ErrNotAProject
	}
	if kind == "" && strings.EqualFold(u.Hostname(), "github.com") {
		kind = forge.GitHub
	}
	a, ok := registry[kind]
	if !ok {
		return forge.Project{}, "", forge.ErrUnsupported
	}
	p, err := a.parseProject(raw)
	return p, kind, err
}

func New(kind, base string, t config.Secret) (forge.Provider, error) {
	a, ok := registry[kind]
	if !ok {
		return nil, forge.ErrUnsupported
	}
	return a.newClient(base, t)
}
