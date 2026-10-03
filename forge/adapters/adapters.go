// Package adapters selects forge implementations. The application knows this
// registry and the neutral contracts, never an individual forge's API.
package adapters

import (
	"fmt"
	"strings"

	"github.com/terva-sh/vuoro/config"
	"github.com/terva-sh/vuoro/forge"
	"github.com/terva-sh/vuoro/forge/forgejo"
	"github.com/terva-sh/vuoro/forge/gitlab"
)

type adapter struct {
	parse     func(string) (forge.Ref, error)
	newClient func(string, config.Secret) (forge.Provider, error)
}

var registry = map[string]adapter{
	forge.GitLab:  {gitlab.ParseURL, func(base string, t config.Secret) (forge.Provider, error) { return gitlab.New(base, t) }},
	forge.Forgejo: {forgejo.ParseURL, func(base string, t config.Secret) (forge.Provider, error) { return forgejo.New(base, t) }},
}

// ParseURL honors a configured host's kind. Anonymous URLs use their path
// shape; github.com is deliberately refused until its adapter exists.
func ParseURL(raw, kind string) (forge.Ref, string, error) {
	u, err := forge.ChangeURL(raw)
	if err != nil {
		return forge.Ref{}, "", err
	}
	if kind == "" {
		switch {
		case strings.EqualFold(u.Hostname(), "github.com"):
			return forge.Ref{}, "", fmt.Errorf("GitHub is not supported yet: %w", forge.ErrUnsupported)
		case strings.Contains(u.Path, "/-/merge_requests/"):
			kind = forge.GitLab
		case strings.Contains(u.Path, "/pulls/"):
			kind = forge.Forgejo
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
func New(kind, base string, t config.Secret) (forge.Provider, error) {
	a, ok := registry[kind]
	if !ok {
		return nil, forge.ErrUnsupported
	}
	return a.newClient(base, t)
}
