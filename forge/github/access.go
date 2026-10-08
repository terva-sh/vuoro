package github

import (
	"context"
	"net/http"
	"slices"
	"strings"

	"github.com/terva-sh/vuoro/forge"
)

// Access says how far the token reaches (D57). A classic token's scopes come
// back in X-OAuth-Scopes on any REST request. GitHub tells an application
// nothing of a fine-grained token's permissions, so that is reported as not
// known, with what to check on GitHub.
func (c *Client) Access(ctx context.Context) (forge.Access, error) {
	_, h, err := c.send(ctx, http.MethodGet, c.rest+"/user", "/user", nil)
	if err != nil {
		return forge.Access{}, err
	}
	if strings.HasPrefix(c.token.Reveal(), "github_pat_") || h.Values("X-OAuth-Scopes") == nil {
		return forge.Access{Level: forge.AccessUnknown, Note: "GitHub does not tell vuoro what a fine-grained token may do. On GitHub, give it Pull requests: read and write and Contents: read, on the repositories you review, and nothing else."}, nil
	}
	return classicAccess(h.Get("X-OAuth-Scopes")), nil
}

// classicAccess places a classic token's scopes on the gauge. A classic
// token reads private pull requests only with repo, which also writes, so it
// is never read-only on a private repository.
func classicAccess(header string) forge.Access {
	var scopes []string
	for s := range strings.SplitSeq(header, ",") {
		if s = strings.TrimSpace(s); s != "" {
			scopes = append(scopes, s)
		}
	}
	a := forge.Access{Scopes: scopes}
	for _, s := range scopes {
		if broadScope(s) {
			a.Broad = append(a.Broad, s)
		}
	}
	switch {
	case len(a.Broad) > 0:
		a.Level = forge.AccessBroad
	case slices.Contains(scopes, "repo"):
		a.Level = forge.AccessFull
	case slices.Contains(scopes, "public_repo"):
		a.Level = forge.AccessFull
		a.Note = "public_repo reaches public repositories only. Add repo to review private ones."
	default:
		a.Level = forge.AccessRead
		a.Missing = []string{"repo"}
		a.Note = "Without repo the token reads public repositories only, and posts nothing."
	}
	return a
}

// broadScope is a classic scope beyond reviewing: administration, deleting,
// organization writes, and Actions workflows.
func broadScope(s string) bool {
	switch s {
	case "delete_repo", "write:org", "workflow", "write:packages", "delete:packages", "site_admin":
		return true
	}
	return strings.HasPrefix(s, "admin:") || strings.HasSuffix(s, ":enterprise")
}
