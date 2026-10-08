package forgejo

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/terva-sh/vuoro/forge"
)

// Access says how far the token reaches (D57). Forgejo lists a person's
// tokens, with their scopes and the last eight characters of each, to a
// token that holds read:user, and vuoro finds its own among them. Where the
// list is refused, as by Gitea, a token's read scopes are learnt from reads
// of a repository that does not exist: Forgejo checks a token's scope before
// it looks for the repository, so a missing scope answers 403 and a present
// one 404. Nothing is ever written to learn whether a write would succeed.
func (c *Client) Access(ctx context.Context) (forge.Access, error) {
	me, err := c.Me(ctx)
	if err != nil {
		return forge.Access{}, err
	}
	who, _ := read[struct {
		IsAdmin bool `json:"is_admin"`
	}](ctx, c, "/user")
	tokens, err := pages[struct {
		Scopes    []string `json:"scopes"`
		LastEight string   `json:"token_last_eight"`
	}](ctx, c, "/users/"+url.PathEscape(me)+"/tokens")
	if err == nil {
		secret := c.token.Reveal()
		for _, t := range tokens {
			if len(secret) >= 8 && t.LastEight == secret[len(secret)-8:] {
				return scopeAccess(t.Scopes, who.IsAdmin), nil
			}
		}
	}
	return c.probeAccess(ctx, me)
}

// probeAccess learns the token's read scopes from reads of a repository that
// cannot exist, and says that whether it may post is not known.
func (c *Client) probeAccess(ctx context.Context, me string) (forge.Access, error) {
	missing := func(path string) (bool, error) {
		_, err := c.get(ctx, "/repos/"+url.PathEscape(me)+"/vuoro-access-check.invalid"+path, nil)
		var api *forge.APIError
		switch {
		case errors.As(err, &api) && api.Status == http.StatusForbidden && strings.Contains(api.Message, "required scope"):
			return true, nil
		case errors.As(err, &api) && api.Status == http.StatusNotFound:
			return false, nil
		case err == nil:
			return false, nil
		}
		return false, err
	}
	a := forge.Access{Note: "Forgejo would not list this token's scopes, so vuoro could not tell whether it may post. Give it read:user, and vuoro can say."}
	for _, p := range []struct{ path, scope string }{{"/pulls", "read:repository"}, {"/issues", "read:issue"}} {
		gone, err := missing(p.path)
		if err != nil {
			return forge.Access{Level: forge.AccessUnknown, Note: "vuoro could not tell what this token may do."}, nil
		}
		if gone {
			a.Missing = append(a.Missing, p.scope)
		}
	}
	a.Level = forge.AccessRead
	if len(a.Missing) > 0 {
		a.Level, a.Note = forge.AccessNone, ""
	}
	return a, nil
}

// scopeAccess places a token's scopes on the gauge. A write scope implies its
// read. all is every scope, including administration. Writes beyond
// repositories, issues, and notifications reach past reviewing, and so does
// administration. A token that cannot read reviews is placed there whatever
// else it holds, so it is never saved.
func scopeAccess(scopes []string, admin bool) forge.Access {
	a := forge.Access{Scopes: scopes}
	has := func(s string) bool { return slices.Contains(scopes, s) || slices.Contains(scopes, "all") }
	for _, s := range scopes {
		switch s {
		case "all", "sudo", "read:admin", "write:admin", "write:organization", "write:user", "write:package", "write:activitypub":
			a.Broad = append(a.Broad, s)
		}
	}
	if admin && slices.Contains(scopes, "all") {
		a.Broad = append(a.Broad, "an administrator's account")
	}
	can := func(verb, what string) bool { return has("write:"+what) || verb == "read" && has("read:"+what) }
	for _, need := range []string{"repository", "issue"} {
		if !can("read", need) {
			a.Missing = append(a.Missing, "read:"+need)
		}
	}
	switch {
	case len(a.Missing) > 0:
		a.Level = forge.AccessNone
	case len(a.Broad) > 0:
		a.Level = forge.AccessBroad
	case can("write", "repository") && can("write", "issue"):
		a.Level = forge.AccessFull
	default:
		a.Level = forge.AccessRead
		for _, need := range []string{"repository", "issue"} {
			if !can("write", need) {
				a.Missing = append(a.Missing, "write:"+need)
			}
		}
	}
	if slices.Contains(scopes, "public-only") {
		a.Note = "public-only reaches public repositories only."
	}
	return a
}
